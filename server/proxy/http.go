package proxy

import (
	"bufio"
	"crypto/tls"
	"ehang.io/nps/bridge"
	"ehang.io/nps/lib/cache"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/lib/goroutine"
	"ehang.io/nps/lib/waf"
	"ehang.io/nps/server/connection"
	"ehang.io/nps/web"
	"encoding/json"
	"fmt"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
	"golang.org/x/crypto/acme/autocert"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type httpServer struct {
	BaseServer
	httpPort      int
	httpsPort     int
	http3Port     int // 0=不监听
	httpServer    *http.Server
	httpsServer   *http.Server
	httpsListener net.Listener
	http3Server   http3Service
	useCache      bool
	cache         *cache.Cache
	cacheLen      int
	acmeManager   *autocert.Manager // 可选 ACME；nil=未启用
}

// http3Service HTTP/3 监听生命周期（与具体 quic-go 实现解耦）。
type http3Service interface {
	Close() error
}

func NewHttp(bridge *bridge.Bridge, c *file.Tunnel, httpPort, httpsPort int, useCache bool, cacheLen int, http3Ports ...int) *httpServer {
	http3Port := 0
	if len(http3Ports) > 0 {
		http3Port = http3Ports[0]
	}
	httpServer := &httpServer{
		BaseServer: BaseServer{
			task:   c,
			bridge: bridge,
			Mutex:  sync.Mutex{},
		},
		httpPort:  httpPort,
		httpsPort: httpsPort,
		http3Port: http3Port,
		useCache:  useCache,
		cacheLen:  cacheLen,
	}
	if useCache {
		httpServer.cache = cache.New(cacheLen)
	}
	return httpServer
}

	func (s *httpServer) Start() error {
		var err error
		s.errorContent = LoadErrorPageContent()
		if len(s.errorContent) == 0 {
			s.errorContent = []byte("nps error")
		}
	if err := s.initACME(); err != nil {
		logs.Warn("ACME 自动证书未启用: %v", err)
	}
	if s.httpPort > 0 {
		s.httpServer = s.NewServer(s.httpPort, "http")
		go func() {
			l, err := connection.GetHttpListener()
			if err != nil {
				logs.Error(err)
				os.Exit(0)
			}
			err = s.httpServer.Serve(l)
			if err != nil {
				logs.Error(err)
				os.Exit(0)
			}
		}()
	}
	if s.httpsPort > 0 {
		s.httpsServer = s.NewServer(s.httpsPort, "https")
		go func() {
			s.httpsListener, err = connection.GetHttpsListener()
			if err != nil {
				logs.Error(err)
				os.Exit(0)
			}
			httpsProxy := NewHttpsServer(s.httpsListener, s.bridge, s.useCache, s.cacheLen)
			httpsProxy.bridge = s.bridge
			httpsProxy.task = s.task
			httpsProxy.errorContent = s.errorContent
			httpsProxy.httpPort = s.httpPort
			httpsProxy.httpsPort = s.httpsPort
			httpsProxy.acmeManager = s.acmeManager
			httpsProxy.http3Port = s.http3Port
			logs.Error(httpsProxy.Start())
		}()
	}
	// HTTP/3 需同时启用 HTTPS 入口（证书/SNI）；失败仅告警，不影响 HTTP/HTTPS。
	if s.http3Port > 0 {
		if s.httpsPort <= 0 {
			logs.Warn("HTTP/3 反代需要启用 HTTPS 入口，已跳过 UDP %d", s.http3Port)
		} else if _, http3Srv, listenErr := s.startHTTP3(); listenErr != nil {
			logs.Warn("HTTP/3 反代启动失败，继续提供 HTTP/HTTPS: %v", listenErr)
		} else {
			s.http3Server = http3Srv
		}
	}
	return nil
}

func (s *httpServer) Close() error {
	if s.http3Server != nil {
		_ = s.http3Server.Close()
		s.http3Server = nil
	}
	if s.httpsListener != nil {
		s.httpsListener.Close()
	}
	if s.httpsServer != nil {
		s.httpsServer.Close()
	}
	if s.httpServer != nil {
		s.httpServer.Close()
	}
	return nil
}

func (s *httpServer) handleTunneling(w http.ResponseWriter, r *http.Request) {
	// WAF：Host 解析前尽早拦截（管理面板不走此入口）
	if checkHTTPWAF(w, r) {
		return
	}

	var host *file.Host
	var err error
	host, err = file.GetDb().GetInfoByHost(r.Host, r)
	if err != nil {
		logs.Debug("the url %s %s %s can't be parsed!", r.URL.Scheme, r.Host, r.RequestURI)
		return
	}

	// 自动 http 301 https
	if host.AutoHttps && r.TLS == nil {
		http.Redirect(w, r, "https://"+host.Host+":"+beego.AppConfig.String("https_proxy_port"), http.StatusMovedPermanently)
		return
	}

	// CORS 预检就地应答（不进后端）
	if strings.TrimSpace(host.Cors) != "" && r.Method == http.MethodOptions {
		common.ApplyHostCORS(w, r, host.Cors)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if strings.TrimSpace(host.Cors) != "" {
		common.ApplyHostCORS(w, r, host.Cors)
	}

	// HTTP/2+（含 HTTP/3）与 WebSocket 走 ReverseProxy；HTTP/1.1 仍 Hijack 原样转发。
	if r.ProtoMajor >= 2 || r.Header.Get("Upgrade") != "" {
		rProxy := NewHttpReverseProxy(s)
		rProxy.ServeHTTP(w, r)
	} else {
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
			return
		}
		c, rw, err := hijacker.Hijack()
		if err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}

		s.handleHttp(conn.NewConn(c), r, rw.Reader)
	}

}

func (s *httpServer) handleHttp(c *conn.Conn, r *http.Request, br *bufio.Reader) {
	var (
		host       *file.Host
		target     net.Conn
		err        error
		connClient io.ReadWriteCloser
		scheme     = r.URL.Scheme
		lk         *conn.Link
		targetAddr string
		lenConn    *conn.LenConn
		isReset    bool
		wg         sync.WaitGroup
		remoteAddr string
	)
		defer func() {
			if connClient != nil {
				connClient.Close()
			} else {
				hostName := ""
				if host != nil {
					hostName = host.Host
					if hostName == "" {
						hostName = r.Host
					}
				} else if r != nil {
					hostName = r.Host
				}
				remote := ""
				if c != nil && c.Conn != nil && c.Conn.RemoteAddr() != nil {
					remote = c.Conn.RemoteAddr().String()
				}
				s.writeConnFailInfo(c.Conn, hostName, "Cannot connect to the server", remote)
			}
			c.Close()
		}()
	firstReq := true
reset:
	if isReset {
		host.Client.AddConn()
	}

	// 可信转发头解析后的访客 IP（ACL / 日志）；直连对端仍用于部分兼容路径。
	remoteAddr = effectiveHTTPRemoteAddr(r)
	if remoteAddr == "" {
		remoteAddr = c.RemoteAddr().String()
	}
	peerAddr := c.RemoteAddr().String()

		// 判断访问地址是否在全局黑名单内（访客有效 IP + 直连对端）
		if IsGlobalBlackIp(remoteAddr) || IsGlobalBlackIp(peerAddr) {
			c.Close()
			return
		}
		// WAF（Hijack 路径；handleTunneling 已检过一次，此处对 keep-alive 后续请求补检）
		if waf.Enabled() {
			res := waf.CheckHTTP(r, remoteAddr, trustHTTPCountryHeader(r))
			if !res.Allowed {
				logs.Warn("WAF 拦截 HTTP(hijack) reason=%s ip=%s", res.Reason, common.GetIpByAddr(remoteAddr))
				_, _ = c.Write([]byte("HTTP/1.1 403 Forbidden\r\nContent-Type: text/plain\r\nConnection: close\r\n\r\nForbidden"))
				c.Close()
				return
			}
		}

		if host, err = file.GetDb().GetInfoByHost(r.Host, r); err != nil {
		logs.Notice("the url %s %s %s can't be parsed!, host %s, url %s, remote address %s", r.URL.Scheme, r.Host, r.RequestURI, r.Host, r.URL.Path, remoteAddr)
		c.Close()
		return
	}

	if err := s.CheckFlowAndConnNum(host.Client); err != nil {
		logs.Warn("client id %d, host id %d, error %s, when https connection", host.Client.Id, host.Id, err.Error())
		c.Close()
		return
	}
	if !isReset {
		defer host.Client.AddConn()
	}
	// Hijack 路径下的 CORS 预检（handleTunneling 未走 ResponseWriter 时）
	if strings.TrimSpace(host.Cors) != "" && r.Method == http.MethodOptions {
		var hdr http.Header = make(http.Header)
		rw := corsHeaderWriter{h: hdr}
		common.ApplyHostCORS(rw, r, host.Cors)
		var b strings.Builder
		b.WriteString("HTTP/1.1 204 No Content\r\n")
		for k, vv := range hdr {
			for _, v := range vv {
				b.WriteString(k)
				b.WriteString(": ")
				b.WriteString(v)
				b.WriteString("\r\n")
			}
		}
		b.WriteString("Connection: close\r\n\r\n")
		_, _ = c.Write([]byte(b.String()))
		return
	}
	if err = s.auth(r, c, host.Client.Cnf.U, host.Client.Cnf.P); err != nil {
		logs.Warn("auth error", err, r.RemoteAddr)
		return
	}
	if targetAddr, err = host.Target.GetRandomTarget(); err != nil {
		logs.Warn(err.Error())
		return
	}

	if host.Client.IpWhite && host.Client.IpWhitePass != "" {
		if common.IsAuthIp(remoteAddr, host.Client.VerifyKey, host.Client.IpWhiteList) {

			if r.URL.Path == "/authIp" {
				rawQuery := r.URL.RawQuery

				var jsonBytes []byte
				ip := common.GetIpByAddr(remoteAddr)

				if strings.Contains(rawQuery, "pass=") {
					// 截取 = 后面的值
					pass := strings.Split(rawQuery, "pass=")[1]
					if pass == host.Client.IpWhitePass {
						host.Client.IpWhiteList = append(host.Client.IpWhiteList, ip)
						file.GetDb().UpdateClient(host.Client)
						logs.Info("客户端IP白名单认证授权成功:vkey [%s] ip [%s] password [%s]", host.Client.VerifyKey, ip, pass)
						jsonBytes, err = json.Marshal(map[string]interface{}{"success": true, "message": "授权成功"})
					} else {
						logs.Error("客户端IP白名单认证授权密码错误:vkey [%s] ip [%s] password [%s]", host.Client.VerifyKey, ip, pass)
						jsonBytes, err = json.Marshal(map[string]interface{}{"success": false, "message": "密码错误"})
					}
				} else {
					logs.Error("客户端IP白名单认证授权密码错误:vkey [%s] ip [%s]", host.Client.VerifyKey, ip)
					jsonBytes, err = json.Marshal(map[string]interface{}{"success": false, "message": "参数错误"})
				}
				s.errorContent, err = jsonBytes, err
				s.errorCode = 200
				return
			}

			errorContent, _ := web.ReadStaticFile("page/auth.html")
			authHtml := string(errorContent)
			authHtml = strings.ReplaceAll(authHtml, "${ip}", common.GetIpByAddr(remoteAddr))
			s.errorContent, err = []byte(authHtml), err
			s.errorCode = 401
			return
		}
	}

	// 三层源地址 ACL（含域名 geosite:）
	if SourceDenied(remoteAddr, host.Client, nil, host, host.Host) {
		c.Close()
		return
	}

	lk = conn.NewLink("http", targetAddr, host.Client.Cnf.Crypt, host.Client.Cnf.Compress, remoteAddr, host.Target.LocalProxy, hostProxyProtocolVersion(host))
	if target, err = s.bridge.SendLinkInfo(host.Client.Id, lk, nil); err != nil {
		logs.Notice("connect to target %s error %s", lk.Host, err)
		return
	}
	connClient = conn.GetConn(target, lk.Crypt, lk.Compress, host.Client.Rate, true)
	// 读方向响应头改写 / CORS 注入（写方向仍直通）
	respReader := wrapResponseHeaderConn(connClient, host)

	//read from inc-client
	go func() {
		wg.Add(1)
		isReset = false
		defer connClient.Close()
		defer func() {
			wg.Done()
			if !isReset {
				c.Close()
			}
		}()

		if err1 := goroutine.CopyBuffer(c, respReader, host.Client.Flow, nil, host, ""); err1 != nil {
			return
		}
	}()

	for {
		//if the cache start and the request is in the cache list, return the cache
		if s.useCache {
			if v, ok := s.cache.Get(filepath.Join(host.Host, r.URL.Path)); ok {
				n, err := c.Write(v.([]byte))
				if err != nil {
					break
				}
				logs.Trace("%s request, method %s, host %s, url %s, remote address %s, return cache", r.URL.Scheme, r.Method, r.Host, r.URL.Path, c.RemoteAddr().String())
				host.Client.Flow.Add(int64(n), int64(n))
				host.Flow.Add(int64(n), int64(n))
				//if return cache and does not create a new conn with client and Connection is not set or close, close the connection.
				if strings.ToLower(r.Header.Get("Connection")) == "close" || strings.ToLower(r.Header.Get("Connection")) == "" {
					break
				}
				goto readReq
			}
		}

		// URL 路径重写（在改 Host/请求头之前）
		rewriteHostRequestPath(r, host.PathRewrite)
		//change the host and header and set proxy setting
		common.ChangeHostAndHeader(r, host.HostChange, host.HeaderChange, remoteAddr, file.GetDb().GetGlobalSettings().HttpAddOriginHeader)

		logs.Info("%s request, method %s, host %s, url %s, remote address %s, target %s", r.URL.Scheme, r.Method, r.Host, r.URL.Path, remoteAddr, lk.Host)

		//write
		lenConn = conn.NewLenConn(connClient)
		//lenConn = conn.LenConn
		if firstReq {
			if err = writeRequestRaw(lenConn, r, br); err != nil {
				logs.Error(err)
				break
			}
		} else {
			if err = r.Write(lenConn); err != nil {
				logs.Error(err)
				break
			}
		}
		firstReq = false
		host.Client.Flow.Add(int64(lenConn.Len), int64(lenConn.Len))
		host.Flow.Add(int64(lenConn.Len), int64(lenConn.Len))

	readReq:
		//read req from connection
		r, err = http.ReadRequest(br)
		if err != nil {
			//break
			return
		}
		r.URL.Scheme = scheme
		if hostTmp, err := file.GetDb().GetInfoByHost(r.Host, r); err != nil {
			logs.Notice("the url %s %s %s can't be parsed!", r.URL.Scheme, r.Host, r.RequestURI)
			break
		} else if host != hostTmp {
			host = hostTmp
			isReset = true
			connClient.Close()
			goto reset
		}
	}
	wg.Wait()
}

func writeRequestRaw(w io.Writer, r *http.Request, br *bufio.Reader) error {
	bw := bufio.NewWriter(w)
	if _, err := fmt.Fprintf(bw, "%s %s HTTP/1.1\r\n", r.Method, r.URL.RequestURI()); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(bw, "Host: %s\r\n", r.Host); err != nil {
		return err
	}
	chunked := len(r.TransferEncoding) > 0 && r.TransferEncoding[0] == "chunked"
	if chunked {
		if _, err := io.WriteString(bw, "Transfer-Encoding: chunked\r\n"); err != nil {
			return err
		}
	}
	if err := r.Header.Write(bw); err != nil {
		return err
	}
	if _, err := io.WriteString(bw, "\r\n"); err != nil {
		return err
	}
	if err := bw.Flush(); err != nil {
		return err
	}
	if r.ContentLength > 0 {
		if _, err := io.CopyN(w, br, r.ContentLength); err != nil {
			return err
		}
	} else if chunked {
		if err := copyRawChunked(w, br); err != nil {
			return err
		}
	}
	return nil
}

func copyRawChunked(w io.Writer, br *bufio.Reader) error {
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return err
		}
		if _, err := io.WriteString(w, line); err != nil {
			return err
		}
		sizeStr := strings.TrimSpace(line)
		if i := strings.IndexByte(sizeStr, ';'); i >= 0 {
			sizeStr = sizeStr[:i]
		}
		size, err := strconv.ParseInt(sizeStr, 16, 64)
		if err != nil {
			return err
		}
		if size == 0 {
			for {
				tline, err := br.ReadString('\n')
				if err != nil {
					return err
				}
				if _, err := io.WriteString(w, tline); err != nil {
					return err
				}
				if tline == "\r\n" || tline == "\n" {
					return nil
				}
			}
		}
		if _, err := io.CopyN(w, br, size); err != nil {
			return err
		}
		var crlf [2]byte
		if _, err := io.ReadFull(br, crlf[:]); err != nil {
			return err
		}
		if _, err := w.Write(crlf[:]); err != nil {
			return err
		}
	}
}

// corsHeaderWriter 仅收集 Header，便于 Hijack 路径手写 204 应答。
type corsHeaderWriter struct {
	h http.Header
}

func (c corsHeaderWriter) Header() http.Header         { return c.h }
func (c corsHeaderWriter) Write([]byte) (int, error)   { return 0, nil }
func (c corsHeaderWriter) WriteHeader(statusCode int) {}

func (s *httpServer) NewServer(port int, scheme string) *http.Server {
	handler := http.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Scheme = scheme
		s.handleTunneling(w, r)
	}))
	if scheme == "http" && s.acmeManager != nil {
		// HTTP-01 挑战须先于 Host 路由应答；其它请求仍走反代。
		handler = s.acmeManager.HTTPHandler(handler)
	}
	return &http.Server{
		Addr:    ":" + strconv.Itoa(port),
		Handler: handler,
		// Disable HTTP/2.
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	}
}

func (s *httpServer) NewServerWithTls(port int, scheme string, l net.Listener, certFile string, keyFile string) error {

	if certFile == "" || keyFile == "" {
		logs.Error("证书文件为空")
		return nil
	}
	var certFileByte = []byte(certFile)
	var keyFileByte = []byte(keyFile)

	config := &tls.Config{}
	config.Certificates = make([]tls.Certificate, 1)

	var err error
	config.Certificates[0], err = tls.X509KeyPair(certFileByte, keyFileByte)
	if err != nil {
		return err
	}

	s2 := &http.Server{
		Addr: ":" + strconv.Itoa(port),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.URL.Scheme = scheme
			s.handleTunneling(w, r)
		}),
		// Disable HTTP/2.
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
		TLSConfig:    config,
	}

	return s2.ServeTLS(l, "", "")
}
