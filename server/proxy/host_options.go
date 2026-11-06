package proxy

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/file"
)

// hostProxyProtocolVersion 返回域名后端配置的 PROXY 协议版本字符串。
// 0/缺省=关闭，保持旧规则行为。
func hostProxyProtocolVersion(host *file.Host) string {
	if host == nil {
		return ""
	}
	host.RLock()
	target := host.Target
	host.RUnlock()
	if target == nil {
		return ""
	}
	target.RLock()
	version := target.ProxyProtocol
	target.RUnlock()
	switch version {
	case 1:
		return "V1"
	case 2:
		return "V2"
	default:
		return ""
	}
}

// applyHeaderChanges 按行应用「Name: Value」规则；空值或 `-Name` 删除头。
// 非法键值忽略，避免可选规则弄坏代理流。
func applyHeaderChanges(headers http.Header, rules string) {
	for _, line := range strings.Split(strings.ReplaceAll(rules, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		nameForProtection := line
		if strings.HasPrefix(nameForProtection, "-") {
			nameForProtection = strings.TrimSpace(nameForProtection[1:])
		} else if name, _, ok := strings.Cut(nameForProtection, ":"); ok {
			nameForProtection = strings.TrimSpace(name)
		}
		if protectedResponseHeader(nameForProtection) {
			continue
		}
		if strings.HasPrefix(line, "-") && strings.TrimSpace(line[1:]) != "" {
			headers.Del(strings.TrimSpace(line[1:]))
			continue
		}
		name, value, ok := strings.Cut(line, ":")
		name, value = strings.TrimSpace(name), strings.TrimSpace(value)
		if !ok || name == "" || strings.ContainsAny(name, "\r\n") || strings.ContainsAny(value, "\r\n") {
			continue
		}
		if value == "" {
			headers.Del(name)
		} else {
			headers.Set(name, value)
		}
	}
}

func protectedResponseHeader(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "content-length", "transfer-encoding", "connection":
		return true
	default:
		return false
	}
}

// rewriteHostRequestPath 按 Host.PathRewrite 改写请求路径。
// 规则：每行 `旧前缀 => 新前缀`（也接受 `=`）；仅匹配前缀；空规则跳过。
func rewriteHostRequestPath(request *http.Request, rules string) {
	if request == nil || strings.TrimSpace(rules) == "" || request.URL == nil {
		return
	}
	path := request.URL.EscapedPath()
	if path == "" {
		path = "/"
	}
	for _, line := range strings.Split(strings.ReplaceAll(rules, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		old, replacement, ok := strings.Cut(line, "=>")
		if !ok {
			old, replacement, ok = strings.Cut(line, "=")
		}
		if !ok {
			continue
		}
		old, replacement = strings.TrimSpace(old), strings.TrimSpace(replacement)
		if old == "" || !strings.HasPrefix(old, "/") || !strings.HasPrefix(replacement, "/") {
			continue
		}
		if path == old || strings.HasPrefix(path, strings.TrimRight(old, "/")+"/") {
			remainder := path[len(old):]
			path = strings.TrimRight(replacement, "/") + remainder
			if path == "" {
				path = "/"
			}
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			request.URL.Path, _ = url.PathUnescape(path)
			request.URL.RawPath = path
			return
		}
	}
}

func hostNeedsResponseTransform(host *file.Host) bool {
	if host == nil {
		return false
	}
	host.RLock()
	defer host.RUnlock()
	return strings.TrimSpace(host.ResponseHeaderChange) != "" || strings.TrimSpace(host.Cors) != ""
}

// responseHeaderTransform 转发响应时只替换首个 HTTP 响应头块；正文与 chunk 帧不动。
type responseHeaderTransform struct {
	reader io.Reader
	host   *file.Host
	buf    []byte
	done   bool
}

func newResponseHeaderTransform(reader io.Reader, host *file.Host) io.Reader {
	return &responseHeaderTransform{reader: reader, host: host}
}

func (t *responseHeaderTransform) Read(p []byte) (int, error) {
	if t.done {
		if len(t.buf) > 0 {
			n := copy(p, t.buf)
			t.buf = t.buf[n:]
			return n, nil
		}
		return t.reader.Read(p)
	}
	for {
		if end := bytes.Index(t.buf, []byte("\r\n\r\n")); end >= 0 {
			headers := append([]byte(nil), t.buf[:end+4]...)
			remainder := append([]byte(nil), t.buf[end+4:]...)
			transformed := transformResponseHeaders(headers, t.host)
			t.buf = append(transformed, remainder...)
			t.done = true
			if len(t.buf) > 0 {
				n := copy(p, t.buf)
				t.buf = t.buf[n:]
				return n, nil
			}
			continue
		}
		var chunk [16 * 1024]byte
		n, err := t.reader.Read(chunk[:])
		if n > 0 {
			t.buf = append(t.buf, chunk[:n]...)
			if len(t.buf) > 64*1024 {
				copyN := copy(p, t.buf)
				t.buf = t.buf[copyN:]
				t.done = true
				return copyN, nil
			}
			continue
		}
		if err != nil {
			if len(t.buf) > 0 {
				copyN := copy(p, t.buf)
				t.buf = t.buf[copyN:]
				if len(t.buf) == 0 {
					t.done = true
				}
				return copyN, nil
			}
			return 0, err
		}
	}
}

func transformResponseHeaders(raw []byte, host *file.Host) []byte {
	response, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(raw)), nil)
	if err != nil {
		return raw
	}
	defer response.Body.Close()
	if host != nil {
		host.RLock()
		rules, cors := host.ResponseHeaderChange, host.Cors
		host.RUnlock()
		applyHeaderChanges(response.Header, rules)
		if strings.TrimSpace(cors) != "" {
			common.ApplyHostCORS(headerWriter{h: response.Header}, &http.Request{}, cors)
		}
	}
	var out bytes.Buffer
	statusText := response.Status
	if statusText == "" {
		statusText = strconv.Itoa(response.StatusCode)
	}
	fmt.Fprintf(&out, "HTTP/%d.%d %s\r\n", response.ProtoMajor, response.ProtoMinor, statusText)
	_ = response.Header.Write(&out)
	if len(response.TransferEncoding) > 0 {
		fmt.Fprintf(&out, "Transfer-Encoding: %s\r\n", strings.Join(response.TransferEncoding, ", "))
	}
	out.WriteString("\r\n")
	return out.Bytes()
}

// headerWriter 仅实现 Header()，供 ApplyHostCORS 写响应头。
type headerWriter struct {
	h http.Header
}

func (w headerWriter) Header() http.Header         { return w.h }
func (w headerWriter) Write([]byte) (int, error)   { return 0, nil }
func (w headerWriter) WriteHeader(statusCode int) {}

// respHeaderConn 在读方向做响应头改写，写方向原样透传。
type respHeaderConn struct {
	io.ReadWriteCloser
	tr io.Reader
}

func wrapResponseHeaderConn(c io.ReadWriteCloser, host *file.Host) io.ReadWriteCloser {
	if c == nil || !hostNeedsResponseTransform(host) {
		return c
	}
	return &respHeaderConn{ReadWriteCloser: c, tr: newResponseHeaderTransform(c, host)}
}

func (c *respHeaderConn) Read(p []byte) (int, error) {
	return c.tr.Read(p)
}
