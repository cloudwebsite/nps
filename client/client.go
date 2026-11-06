package client

import (
	"bufio"
	"bytes"
	"errors"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ehang.io/nps/lib/nps_mux"
	"github.com/pires/go-proxyproto"

	"github.com/astaxie/beego/logs"
	"github.com/xtaci/kcp-go"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/config"
	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/crypt"
	"ehang.io/nps/lib/install"
)

type TRPClient struct {
	svrAddr        string
	bridgeConnType string
	proxyUrl       string
	vKey           string
	p2pAddr        map[string]string
	tunnel         *nps_mux.Mux
	signal         *conn.Conn
	ticker         *time.Ticker
	cnf            *config.Config
	disconnectTime int
	once           sync.Once
	closeCh        chan struct{} // closed when client is shutting down; stops ping
	logger         Logger        // 每客户端独立的 logger，未设置时使用全局 logger
}

// Logger 客户端日志接口，GUI / CLI 均可注入实现
type Logger interface {
	Info(format string, v ...interface{})
	Error(format string, v ...interface{})
	Warn(format string, v ...interface{})
	Trace(format string, v ...interface{})
}

// new client
func NewRPClient(svraddr string, vKey string, bridgeConnType string, proxyUrl string, cnf *config.Config, disconnectTime int) *TRPClient {
	return &TRPClient{
		svrAddr:        svraddr,
		p2pAddr:        make(map[string]string, 0),
		vKey:           vKey,
		bridgeConnType: bridgeConnType,
		proxyUrl:       proxyUrl,
		cnf:            cnf,
		disconnectTime: disconnectTime,
		once:           sync.Once{},
		closeCh:        make(chan struct{}),
		logger:         nil, // 默认使用全局 logger，可通过 SetLogger 设置
	}
}

// SetLogger 设置客户端的独立 logger
func (s *TRPClient) SetLogger(logger Logger) {
	s.logger = logger
}

// log 辅助方法：如果设置了独立 logger 就使用，否则使用全局 logger
func (s *TRPClient) logInfo(format string, v ...interface{}) {
	if s.logger != nil {
		s.logger.Info(format, v...)
	} else {
		logs.Info(format, v...)
	}
}

func (s *TRPClient) logError(format string, v ...interface{}) {
	if s.logger != nil {
		s.logger.Error(format, v...)
	} else {
		logs.Error(format, v...)
	}
}

func (s *TRPClient) logWarn(format string, v ...interface{}) {
	if s.logger != nil {
		s.logger.Warn(format, v...)
	} else {
		logs.Warn(format, v...)
	}
}

func (s *TRPClient) logTrace(format string, v ...interface{}) {
	if s.logger != nil {
		s.logger.Trace(format, v...)
	} else {
		logs.Trace(format, v...)
	}
}

// IsConnected 返回客户端是否已成功连接到服务器
func (s *TRPClient) IsConnected() bool {
	return s.signal != nil
}

var NowStatus int
var CloseClient bool

var remoteUpdateInProgress atomic.Bool

// start
func (s *TRPClient) Start() {
	CloseClient = false
retry:
	if CloseClient {
		return
	}
	NowStatus = 0
	c, err := NewConn(s.bridgeConnType, s.vKey, s.svrAddr, common.WORK_MAIN, s.proxyUrl)
	if err != nil {
		s.logError("The connection server failed and will be reconnected in five seconds, error", err.Error())
		time.Sleep(time.Second * 5)
		goto retry
	}
	if c == nil {
		s.logError("Error data from server, and will be reconnected in five seconds")
		time.Sleep(time.Second * 5)
		goto retry
	}
	s.logInfo("Successful connection with server %s", s.svrAddr)
	//monitor the connection
	go s.ping()
	s.signal = c
	//start a channel connection
	go s.newChan()
	//start health check if the it's open
	if s.cnf != nil && len(s.cnf.Healths) > 0 {
		go heathCheck(s.cnf.Healths, s.signal)
	}
	NowStatus = 1
	//msg connection, eg udp
	s.handleMain()
}

// handle main connection
func (s *TRPClient) handleMain() {
mainLoop:
	for {
		flags, err := s.signal.ReadFlag()
		if err != nil {
			s.logError("Accept server data error %s, end this service", err.Error())
			break
		}
		switch flags {
		case common.REPORT_LOCAL_IP:
			// server requests private/LAN IPs (new nps); ignore failures so main loop continues
			localIPs := common.GetLocalIPs(s.signal.Conn)
			if err := s.signal.WriteLenContent([]byte(localIPs)); err != nil {
				s.logWarn("report local ip failed: %s", err.Error())
			} else {
				s.logInfo("reported local addr: %s", localIPs)
			}
		case common.CLIENT_UPDATE:
			s.requestRemoteUpdate()
		case common.NEW_UDP_CONN:
			//read server udp addr and password
			if lAddr, err := s.signal.GetShortLenContent(); err != nil {
				s.logWarn(err.Error())
				break mainLoop
			} else if pwd, err := s.signal.GetShortLenContent(); err == nil {
				var localAddr string
				//The local port remains unchanged for a certain period of time
				if v, ok := s.p2pAddr[crypt.Md5(string(pwd)+strconv.Itoa(int(time.Now().Unix()/100)))]; !ok {
					tmpConn, err := common.GetLocalUdpAddr()
					if err != nil {
						s.logError(err.Error())
						break mainLoop
					}
					localAddr = discoverP2PLocalAddr(tmpConn.LocalAddr().String())
				} else {
					localAddr = v
				}
				go s.newUdpConn(localAddr, string(lAddr), string(pwd))
			}
		}
	}
	s.Close()
}

func (s *TRPClient) newUdpConn(localAddr, rAddr string, md5Password string) {
	var localConn net.PacketConn
	var err error
	var remoteAddress string
	if GetP2PMode() == P2PModeModern && GetP2PHolepunchAddr() != "" {
		if quicConn, herr := tryHolepunchQUIC(md5Password, "provider"); herr == nil {
			s.logTrace("successful holepunch p2p connection, address %v", quicConn.RemoteAddr())
			conn.Accept(nps_mux.NewMux(quicConn, "quic", s.disconnectTime), func(c net.Conn) {
				go s.handleChan(c)
			})
			return
		} else {
			s.logWarn("P2P holepunch failed, fallback classic: %v", herr)
		}
	}
	if remoteAddress, localConn, err = handleP2PUdp(localAddr, rAddr, md5Password, common.WORK_P2P_PROVIDER); err != nil {
		s.logError(err.Error())
		return
	}
	if GetP2PMode() == P2PModeModern {
		quicConn, qerr := acceptP2PQUIC(localConn, remoteAddress)
		if qerr != nil {
			s.logError("P2P modern QUIC accept failed: %v", qerr)
			_ = localConn.Close()
			return
		}
		s.logTrace("successful modern p2p connection with client, address %v", quicConn.RemoteAddr())
		conn.Accept(nps_mux.NewMux(quicConn, "quic", s.disconnectTime), func(c net.Conn) {
			go s.handleChan(c)
		})
		return
	}
	l, err := kcp.ServeConn(nil, 150, 3, localConn)
	if err != nil {
		s.logError(err.Error())
		return
	}
	s.logTrace("start local p2p udp listen, local address %s", localConn.LocalAddr().String())
	for {
		udpTunnel, err := l.AcceptKCP()
		if err != nil {
			s.logError(err.Error())
			l.Close()
			return
		}
		if udpTunnel.RemoteAddr().String() == string(remoteAddress) {
			conn.SetUdpSession(udpTunnel)
			s.logTrace("successful connection with client ,address %s", udpTunnel.RemoteAddr().String())
			//read link info from remote
			conn.Accept(nps_mux.NewMux(udpTunnel, s.bridgeConnType, s.disconnectTime), func(c net.Conn) {
				go s.handleChan(c)
			})
			break
		}
	}
}

// pmux tunnel
func (s *TRPClient) newChan() {
	tunnel, err := NewConn(s.bridgeConnType, s.vKey, s.svrAddr, common.WORK_CHAN, s.proxyUrl)
	if err != nil {
		// WORK_MAIN may already be up; without Close the client stays half-connected
		// (handleMain blocks, outer reconnect loop never runs). See #115.
		s.logError("connect to %s error: %v, client will reconnect", s.svrAddr, err)
		s.Close()
		return
	}
	s.tunnel = nps_mux.NewMux(tunnel.Conn, s.bridgeConnType, s.disconnectTime)
	for {
		src, err := s.tunnel.Accept()
		if err != nil {
			s.logWarn(err.Error())
			s.Close()
			break
		}
		go s.handleChan(src)
	}
}

func (s *TRPClient) handleChan(src net.Conn) {
	lk, err := conn.NewConn(src).GetLinkInfo()
	if err != nil || lk == nil {
		src.Close()
		s.logError("get connection info from server error %v", err)
		return
	}
	//host for target processing
	lk.Host = common.FormatAddress(lk.Host)
	//if Conn type is http, read the request and log
	if lk.ConnType == "http" {
		if targetConn, err := net.DialTimeout(common.CONN_TCP, lk.Host, lk.Option.Timeout); err != nil {
			s.logWarn("connect to %s error %s", lk.Host, err.Error())
			src.Close()
		} else {
			srcConn := conn.GetConn(src, lk.Crypt, lk.Compress, nil, false)
			go func() {
				common.CopyBuffer(srcConn, targetConn)
				srcConn.Close()
				targetConn.Close()
			}()
			br := bufio.NewReader(srcConn)
			for {
				if r, err := http.ReadRequest(br); err != nil {
					srcConn.Close()
					targetConn.Close()
					break
				} else {
					remoteAddr := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
					if len(remoteAddr) == 0 {
						remoteAddr = r.RemoteAddr
					}
					s.logTrace("http request, method %s, host %s, url %s, remote address %s", r.Method, r.Host, r.URL.Path, remoteAddr)
					r.Write(targetConn)
				}
			}
		}
		return
	}
	if lk.ConnType == "udp5" {
		s.logTrace("new %s connection with the goal of %s, remote address:%s", lk.ConnType, lk.Host, lk.RemoteAddr)
		s.handleUdp(src)
		return
	}
	// 面板终端（旧客户端不认识 shell/shellresize，会落到下方 Dial 失败）
	if lk.ConnType == "shellresize" {
		handleShellResize(lk)
		src.Close()
		return
	}
	if lk.ConnType == "shell" {
		s.logTrace("new shell connection, remote address:%s", lk.RemoteAddr)
		handleShellLink(s, src, lk)
		return
	}
	//connect to target if conn type is tcp or udp
	if targetConn, err := net.DialTimeout(lk.ConnType, lk.Host, lk.Option.Timeout); err != nil {
		s.logWarn("connect to %s error %s", lk.Host, err.Error())
		src.Close()
	} else {
		s.logTrace("new %s connection with the goal of %s, remote address:%s", lk.ConnType, lk.Host, lk.RemoteAddr)

		if lk.ProtoVersion == "V1" || lk.ProtoVersion == "V2" {
			if err := writeProxyProtocolHeader(targetConn, lk.RemoteAddr, lk.ProtoVersion); err != nil {
				s.logError("PROXY protocol: %s", err.Error())
			}
		}

		conn.CopyWaitGroup(src, targetConn, lk.Crypt, lk.Compress, nil, nil, false, nil, nil, nil)
	}
}

func (s *TRPClient) requestRemoteUpdate() {
	if !remoteUpdateInProgress.CompareAndSwap(false, true) {
		s.logInfo("remote client update is already in progress")
		return
	}
	go func() {
		defer remoteUpdateInProgress.Store(false)
		s.logInfo("[升级] 收到服务端远程升级指令，开始自更新")
		if install.UpdateNpcNew() {
			s.logInfo("[升级] 二进制已替换，进程即将退出以便重启")
			time.Sleep(time.Second)
			os.Exit(0)
		}
	}()
}

// writeProxyProtocolHeader 向后端写入 PROXY v1/v2；正确处理 IPv6 字面量。
func writeProxyProtocolHeader(target net.Conn, remoteAddr, protoVersion string) error {
	if target == nil {
		return errors.New("PROXY protocol target is nil")
	}
	var version byte
	switch protoVersion {
	case "V1":
		version = 1
	case "V2":
		version = 2
	default:
		return errors.New("unsupported PROXY protocol version " + protoVersion)
	}
	sourceAddr, err := net.ResolveTCPAddr("tcp", strings.TrimSpace(remoteAddr))
	if err != nil || sourceAddr == nil || sourceAddr.IP == nil {
		return errors.New("invalid source address for PROXY protocol: " + remoteAddr)
	}
	destinationAddr, ok := target.RemoteAddr().(*net.TCPAddr)
	if !ok || destinationAddr == nil || destinationAddr.IP == nil {
		return errors.New("invalid destination address for PROXY protocol")
	}
	sourceIsIPv4 := sourceAddr.IP.To4() != nil
	destinationIsIPv4 := destinationAddr.IP.To4() != nil
	if sourceIsIPv4 != destinationIsIPv4 {
		return errors.New("PROXY protocol cannot represent mixed address families")
	}
	transport := proxyproto.TCPv4
	if !sourceIsIPv4 {
		transport = proxyproto.TCPv6
	}
	header := &proxyproto.Header{
		Command:           proxyproto.PROXY,
		SourceAddr:        sourceAddr,
		DestinationAddr:   destinationAddr,
		Version:           version,
		TransportProtocol: transport,
	}
	_, err = header.WriteTo(target)
	return err
}

func (s *TRPClient) handleUdp(serverConn net.Conn) {
	// bind a local udp port
	local, err := net.ListenUDP("udp", nil)
	defer serverConn.Close()
	if err != nil {
		s.logError("bind local udp port error %s", err.Error())
		return
	}
	defer local.Close()
	go func() {
		defer serverConn.Close()
		b := common.BufPoolUdp.Get().([]byte)
		defer common.BufPoolUdp.Put(b)
		var buf bytes.Buffer
		for {
			n, raddr, err := local.ReadFrom(b)
			if err != nil {
				s.logError("read data from remote server error %s", err.Error())
				return
			}
			buf.Reset()
			dgram := common.NewUDPDatagram(common.NewUDPHeader(0, 0, common.ToSocksAddr(raddr)), b[:n])
			dgram.Write(&buf)
			data, err := conn.GetLenBytes(buf.Bytes())
			if err != nil {
				s.logWarn("get len bytes error %s", err.Error())
				continue
			}
			if _, err := serverConn.Write(data); err != nil {
				s.logError("write data to remote  error %s", err.Error())
				return
			}
		}
	}()
	b := common.BufPoolUdp.Get().([]byte)
	defer common.BufPoolUdp.Put(b)
	for {
		n, err := serverConn.Read(b)
		if err != nil {
			s.logError("read udp data from server error %s", err.Error())
			return
		}

		udpData, err := common.ReadUDPDatagram(bytes.NewReader(b[:n]))
		if err != nil {
			s.logError("unpack data error %s", err.Error())
			return
		}
		raddr, err := net.ResolveUDPAddr("udp", udpData.Header.Addr.String())
		if err != nil {
			s.logError("build remote addr err %s", err.Error())
			continue // drop silently
		}
		_, err = local.WriteTo(udpData.Data, raddr)
		if err != nil {
			s.logError("write data to remote %s error %s", raddr.String(), err.Error())
			return
		}
	}
}

// Whether the monitor channel is closed
func (s *TRPClient) ping() {
	s.ticker = time.NewTicker(time.Second * 5)
	defer s.ticker.Stop()
	for {
		select {
		case <-s.ticker.C:
			// tunnel still nil: newChan failed or still connecting; if Close already ran, closeCh fires.
			// tunnel established then closed: tear down so outer loop can reconnect.
			if s.tunnel != nil && s.tunnel.IsClose() {
				s.Close()
				return
			}
		case <-s.closeCh:
			return
		}
	}
}

func (s *TRPClient) Close() {
	s.once.Do(s.closing)
}

func (s *TRPClient) closing() {
	CloseClient = true
	NowStatus = 0
	// unblock ping; safe: closing runs only once via once.Do
	select {
	case <-s.closeCh:
	default:
		close(s.closeCh)
	}
	if s.tunnel != nil {
		_ = s.tunnel.Close()
	}
	if s.signal != nil {
		_ = s.signal.Close()
	}
}
