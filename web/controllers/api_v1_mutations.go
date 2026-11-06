package controllers

import (
	"net/http"
	"strconv"
	"strings"

	"ehang.io/nps/lib/file"
	"ehang.io/nps/server"
	"ehang.io/nps/server/tool"
	"ehang.io/nps/web/audit"
)

func (s *ApiV1Controller) GetBoolNoErr(key string, def ...bool) bool {
	strv := strings.TrimSpace(s.GetString(key))
	if strv == "" && len(def) > 0 {
		return def[0]
	}
	v, err := strconv.ParseBool(strv)
	if err != nil {
		if len(def) > 0 {
			return def[0]
		}
		return false
	}
	return v
}

func (s *ApiV1Controller) apiStr(key string) string {
	return strings.TrimSpace(s.GetString(key))
}

func (s *ApiV1Controller) tunnelModeParam() string {
	mode := s.apiStr("mode")
	if mode == "" {
		mode = s.apiStr("type")
	}
	return mode
}

func ownerUserIDOfClient(c *file.Client) int {
	if c == nil {
		return 0
	}
	c.RLock()
	defer c.RUnlock()
	return c.UserId
}

func (s *ApiV1Controller) loadAllowedClient(clientID int) (*file.Client, bool) {
	if clientID <= 0 {
		s.apiFail(http.StatusBadRequest, "bad_request", "缺少 client_id")
		return nil, false
	}
	if !clientAllowed(s.allowedClientIDs(), clientID) {
		s.apiFail(http.StatusForbidden, "forbidden", "无权操作该客户端")
		return nil, false
	}
	client, err := file.GetDb().GetClient(clientID)
	if err != nil || client == nil {
		s.apiFail(http.StatusNotFound, "not_found", "客户端不存在")
		return nil, false
	}
	return client, true
}

func (s *ApiV1Controller) loadOwnedTunnel(id int) (*file.Tunnel, bool) {
	if id <= 0 {
		s.apiFail(http.StatusBadRequest, "bad_request", "缺少 id")
		return nil, false
	}
	t, err := file.GetDb().GetTask(id)
	if err != nil || t == nil {
		s.apiFail(http.StatusNotFound, "not_found", "隧道不存在")
		return nil, false
	}
	clientID := 0
	if t.Client != nil {
		clientID = t.Client.Id
	}
	if !clientAllowed(s.allowedClientIDs(), clientID) {
		s.apiFail(http.StatusForbidden, "forbidden", "无权操作该隧道")
		return nil, false
	}
	return t, true
}

func (s *ApiV1Controller) loadOwnedHost(id int) (*file.Host, bool) {
	if id <= 0 {
		s.apiFail(http.StatusBadRequest, "bad_request", "缺少 id")
		return nil, false
	}
	h, err := file.GetDb().GetHostById(id)
	if err != nil || h == nil {
		s.apiFail(http.StatusNotFound, "not_found", "域名不存在")
		return nil, false
	}
	clientID := 0
	if h.Client != nil {
		clientID = h.Client.Id
	}
	if !clientAllowed(s.allowedClientIDs(), clientID) {
		s.apiFail(http.StatusForbidden, "forbidden", "无权操作该域名")
		return nil, false
	}
	return h, true
}

// TunnelCreate POST /api/v1/tunnel/create
func (s *ApiV1Controller) TunnelCreate() {
	if s.Ctx.Request.Method != http.MethodPost {
		s.apiFail(http.StatusMethodNotAllowed, "method_not_allowed", "仅支持 POST")
		return
	}
	mode := s.tunnelModeParam()
	target := s.apiStr("target")
	password := s.apiStr("password")
	localPath := s.apiStr("local_path")
	if msg := validateTunnelInput(mode, target, password, localPath); msg != "" {
		s.apiFail(http.StatusBadRequest, "bad_request", msg)
		return
	}
	client, ok := s.loadAllowedClient(s.GetIntNoErr("client_id"))
	if !ok {
		return
	}
	httpProxy, socks5Proxy := false, false
	if mode == "mixProxy" {
		httpProxy = s.GetBoolNoErr("http_proxy", true)
		socks5Proxy = s.GetBoolNoErr("socks5_proxy", true)
		if s.GetString("http_proxy") == "" && s.GetString("socks5_proxy") == "" {
			httpProxy, socks5Proxy = true, true
		}
	}
	t := &file.Tunnel{
		Port:           s.GetIntNoErr("port"),
		ServerIp:       s.apiStr("server_ip"),
		Mode:           mode,
		Target:         &file.Target{TargetStr: target, LocalProxy: s.GetBoolNoErr("local_proxy")},
		Status:         true,
		Remark:         s.apiStr("remark"),
		Password:       password,
		LocalPath:      localPath,
		StripPre:       s.apiStr("strip_pre"),
		ProtoVersion:          s.apiStr("proto_version"),
		IdleAutoStopMinutes:   s.GetIntNoErr("idle_auto_stop_minutes"),
		HttpProxy:             httpProxy,
		Socks5Proxy:           socks5Proxy,
		Flow:                  &file.Flow{},
		Client:                client,
	}
	if t.Port <= 0 {
		t.Port = tool.GenerateServerPort(t.Mode)
	}
	if !tool.TestServerPort(t.Port, t.Mode) {
		s.apiFail(http.StatusConflict, "port_unavailable", "端口不可用或未允许")
		return
	}
	if client.MaxTunnelNum != 0 && client.GetTunnelNum() >= client.MaxTunnelNum {
		s.apiFail(http.StatusConflict, "quota_exceeded", "隧道数量超过限制")
		return
	}
	if err := file.GetDb().CheckCanAddTunnelOrHost(ownerUserIDOfClient(client)); err != nil {
		s.apiFail(http.StatusConflict, "quota_exceeded", err.Error())
		return
	}
	if err := file.GetDb().NewTask(t); err != nil {
		s.auditAPI("tunnel.create", "tunnel", 0, ownerUserIDOfClient(client), audit.ResultFailure, err.Error(), nil)
		s.apiFail(http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	if err := server.AddTask(t); err != nil {
		_ = server.DelTask(t.Id)
		s.auditAPI("tunnel.create", "tunnel", t.Id, ownerUserIDOfClient(client), audit.ResultFailure, err.Error(), nil)
		s.apiFail(http.StatusInternalServerError, "internal", err.Error())
		return
	}
	s.auditAPI("tunnel.create", "tunnel", t.Id, ownerUserIDOfClient(client), audit.ResultSuccess, "", map[string]string{
		"mode": mode, "port": strconv.Itoa(t.Port),
	})
	s.apiOK(map[string]interface{}{"id": t.Id, "port": t.Port, "mode": t.Mode})
}

// TunnelUpdate POST /api/v1/tunnel/update
func (s *ApiV1Controller) TunnelUpdate() {
	if s.Ctx.Request.Method != http.MethodPost {
		s.apiFail(http.StatusMethodNotAllowed, "method_not_allowed", "仅支持 POST")
		return
	}
	t, ok := s.loadOwnedTunnel(s.GetIntNoErr("id"))
	if !ok {
		return
	}
	mode := s.tunnelModeParam()
	if mode == "" {
		mode = t.Mode
	}
	target := s.apiStr("target")
	if target == "" && t.Target != nil {
		target = t.Target.TargetStr
	}
	password := s.GetString("password")
	if strings.TrimSpace(password) == "" {
		password = t.Password
	} else {
		password = strings.TrimSpace(password)
	}
	localPath := s.apiStr("local_path")
	if localPath == "" {
		localPath = t.LocalPath
	}
	if msg := validateTunnelInput(mode, target, password, localPath); msg != "" {
		s.apiFail(http.StatusBadRequest, "bad_request", msg)
		return
	}
	clientID := s.GetIntNoErr("client_id")
	if clientID <= 0 && t.Client != nil {
		clientID = t.Client.Id
	}
	client, ok := s.loadAllowedClient(clientID)
	if !ok {
		return
	}
	newPort := s.GetIntNoErr("port")
	if newPort <= 0 {
		newPort = t.Port
	}
	if newPort != t.Port {
		if !tool.TestServerPort(newPort, mode) {
			s.apiFail(http.StatusConflict, "port_unavailable", "端口不可用或未允许")
			return
		}
		t.Port = newPort
	}
	t.Client = client
	t.Mode = mode
	if s.paramProvided("server_ip") {
		t.ServerIp = s.apiStr("server_ip")
	}
	t.Target = &file.Target{TargetStr: target, LocalProxy: s.GetBoolNoErr("local_proxy", t.Target != nil && t.Target.LocalProxy)}
	t.Password = password
	t.LocalPath = localPath
	if s.paramProvided("strip_pre") {
		t.StripPre = s.apiStr("strip_pre")
	}
	if s.paramProvided("proto_version") {
		t.ProtoVersion = s.apiStr("proto_version")
	}
	if s.paramProvided("remark") {
		t.Remark = s.apiStr("remark")
	}
	if s.paramProvided("idle_auto_stop_minutes") {
		t.IdleAutoStopMinutes = s.GetIntNoErr("idle_auto_stop_minutes")
	}
	if mode == "mixProxy" {
		if s.paramProvided("http_proxy") || s.paramProvided("socks5_proxy") {
			t.HttpProxy = s.GetBoolNoErr("http_proxy", t.HttpProxy)
			t.Socks5Proxy = s.GetBoolNoErr("socks5_proxy", t.Socks5Proxy)
		} else if !t.HttpProxy && !t.Socks5Proxy {
			t.HttpProxy, t.Socks5Proxy = true, true
		}
	} else {
		t.HttpProxy, t.Socks5Proxy = false, false
	}
	file.GetDb().UpdateTask(t)
	_ = server.StopServer(t.Id)
	if err := server.StartTask(t.Id); err != nil {
		s.auditAPI("tunnel.update", "tunnel", t.Id, ownerUserIDOfClient(client), audit.ResultFailure, err.Error(), nil)
		s.apiFail(http.StatusInternalServerError, "internal", err.Error())
		return
	}
	s.auditAPI("tunnel.update", "tunnel", t.Id, ownerUserIDOfClient(client), audit.ResultSuccess, "", nil)
	s.apiOK(map[string]interface{}{"id": t.Id})
}

func firstNonEmpty(v, fallback string) string {
	if strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

func (s *ApiV1Controller) paramProvided(key string) bool {
	if s.Ctx == nil || s.Ctx.Request == nil {
		return false
	}
	_ = s.Ctx.Request.ParseForm()
	if _, ok := s.Ctx.Request.Form[key]; ok {
		return true
	}
	return false
}

// TunnelStart POST /api/v1/tunnel/start
func (s *ApiV1Controller) TunnelStart() {
	if s.Ctx.Request.Method != http.MethodPost {
		s.apiFail(http.StatusMethodNotAllowed, "method_not_allowed", "仅支持 POST")
		return
	}
	t, ok := s.loadOwnedTunnel(s.GetIntNoErr("id"))
	if !ok {
		return
	}
	if err := server.StartTask(t.Id); err != nil {
		s.auditAPI("tunnel.start", "tunnel", t.Id, ownerUserIDOfClient(t.Client), audit.ResultFailure, err.Error(), nil)
		s.apiFail(http.StatusConflict, "start_failed", err.Error())
		return
	}
	s.auditAPI("tunnel.start", "tunnel", t.Id, ownerUserIDOfClient(t.Client), audit.ResultSuccess, "", nil)
	s.apiOK(map[string]interface{}{"id": t.Id, "status": true})
}

// TunnelStop POST /api/v1/tunnel/stop
func (s *ApiV1Controller) TunnelStop() {
	if s.Ctx.Request.Method != http.MethodPost {
		s.apiFail(http.StatusMethodNotAllowed, "method_not_allowed", "仅支持 POST")
		return
	}
	t, ok := s.loadOwnedTunnel(s.GetIntNoErr("id"))
	if !ok {
		return
	}
	if err := server.StopServer(t.Id); err != nil {
		s.auditAPI("tunnel.stop", "tunnel", t.Id, ownerUserIDOfClient(t.Client), audit.ResultFailure, err.Error(), nil)
		s.apiFail(http.StatusConflict, "stop_failed", err.Error())
		return
	}
	s.auditAPI("tunnel.stop", "tunnel", t.Id, ownerUserIDOfClient(t.Client), audit.ResultSuccess, "", nil)
	s.apiOK(map[string]interface{}{"id": t.Id, "status": false})
}

// TunnelDelete POST /api/v1/tunnel/delete
func (s *ApiV1Controller) TunnelDelete() {
	if s.Ctx.Request.Method != http.MethodPost {
		s.apiFail(http.StatusMethodNotAllowed, "method_not_allowed", "仅支持 POST")
		return
	}
	t, ok := s.loadOwnedTunnel(s.GetIntNoErr("id"))
	if !ok {
		return
	}
	owner := ownerUserIDOfClient(t.Client)
	id := t.Id
	if err := server.DelTask(id); err != nil {
		s.auditAPI("tunnel.delete", "tunnel", id, owner, audit.ResultFailure, err.Error(), nil)
		s.apiFail(http.StatusInternalServerError, "internal", err.Error())
		return
	}
	s.auditAPI("tunnel.delete", "tunnel", id, owner, audit.ResultSuccess, "", nil)
	s.apiOK(map[string]interface{}{"id": id})
}

// HostCreate POST /api/v1/host/create
func (s *ApiV1Controller) HostCreate() {
	if s.Ctx.Request.Method != http.MethodPost {
		s.apiFail(http.StatusMethodNotAllowed, "method_not_allowed", "仅支持 POST")
		return
	}
	hostName := s.apiStr("host")
	target := s.apiStr("target")
	if msg := validateHostInput(hostName, target); msg != "" {
		s.apiFail(http.StatusBadRequest, "bad_request", msg)
		return
	}
	client, ok := s.loadAllowedClient(s.GetIntNoErr("client_id"))
	if !ok {
		return
	}
	proxyProtocol, proxyErr := parseHostProxyProtocolValue(s.apiStr("proxy_protocol"), 0, s.apiStr("proxy_protocol") != "")
	if proxyErr != nil {
		s.apiFail(http.StatusBadRequest, "bad_request", proxyErr.Error())
		return
	}
	h := &file.Host{
		Host:                   hostName,
		Target:                 &file.Target{TargetStr: target, LocalProxy: s.GetBoolNoErr("local_proxy"), ProxyProtocol: proxyProtocol},
		HeaderChange:           s.apiStr("header"),
		ResponseHeaderChange:   s.apiStr("resp_header"),
		HostChange:             s.apiStr("hostchange"),
		PathRewrite:            s.apiStr("path_rewrite"),
		Cors:                   s.apiStr("cors"),
		Remark:                 s.apiStr("remark"),
		Location:               s.apiStr("location"),
		Flow:                   &file.Flow{},
		Scheme:                 s.apiStr("scheme"),
		KeyFilePath:            s.apiStr("key_file_path"),
		CertFilePath:           s.apiStr("cert_file_path"),
		AutoHttps:              s.GetBoolNoErr("auto_https") || s.GetBoolNoErr("AutoHttps"),
		AutoSSL:                s.GetBoolNoErr("auto_ssl") || s.GetBoolNoErr("AutoSSL"),
		HttpsJustProxy:         s.GetBoolNoErr("https_just_proxy") || s.GetBoolNoErr("HttpsJustProxy"),
		Client:                 client,
	}
	if h.Scheme == "" {
		h.Scheme = "all"
	}
	if h.Scheme == "http" {
		h.AutoHttps = false
		h.AutoSSL = false
	}
	if client.MaxTunnelNum != 0 && client.GetTunnelNum() >= client.MaxTunnelNum {
		s.apiFail(http.StatusConflict, "quota_exceeded", "隧道/域名数量超过限制")
		return
	}
	if err := file.GetDb().CheckCanAddTunnelOrHost(ownerUserIDOfClient(client)); err != nil {
		s.apiFail(http.StatusConflict, "quota_exceeded", err.Error())
		return
	}
	if err := file.GetDb().NewHost(h); err != nil {
		s.auditAPI("host.create", "host", 0, ownerUserIDOfClient(client), audit.ResultFailure, err.Error(), nil)
		s.apiFail(http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	s.auditAPI("host.create", "host", h.Id, ownerUserIDOfClient(client), audit.ResultSuccess, "", map[string]string{"host": hostName})
	s.apiOK(map[string]interface{}{"id": h.Id})
}

// HostUpdate POST /api/v1/host/update
func (s *ApiV1Controller) HostUpdate() {
	if s.Ctx.Request.Method != http.MethodPost {
		s.apiFail(http.StatusMethodNotAllowed, "method_not_allowed", "仅支持 POST")
		return
	}
	h, ok := s.loadOwnedHost(s.GetIntNoErr("id"))
	if !ok {
		return
	}
	hostName := firstNonEmpty(s.apiStr("host"), h.Host)
	target := s.apiStr("target")
	if target == "" && h.Target != nil {
		target = h.Target.TargetStr
	}
	if msg := validateHostInput(hostName, target); msg != "" {
		s.apiFail(http.StatusBadRequest, "bad_request", msg)
		return
	}
	clientID := s.GetIntNoErr("client_id")
	if clientID <= 0 && h.Client != nil {
		clientID = h.Client.Id
	}
	client, ok := s.loadAllowedClient(clientID)
	if !ok {
		return
	}
	location := firstNonEmpty(s.apiStr("location"), h.Location)
	scheme := firstNonEmpty(s.apiStr("scheme"), h.Scheme)
	if hostName != h.Host || location != h.Location || scheme != h.Scheme {
		tmp := &file.Host{Id: h.Id, Host: hostName, Location: location, Scheme: scheme}
		if file.GetDb().IsHostExist(tmp) {
			s.apiFail(http.StatusConflict, "conflict", "host has exist")
			return
		}
	}
	h.Client = client
	h.Host = hostName
	currentPP := 0
	localProxy := h.Target != nil && h.Target.LocalProxy
	if h.Target != nil {
		currentPP = h.Target.ProxyProtocol
	}
	proxyProtocol, proxyErr := parseHostProxyProtocolValue(s.apiStr("proxy_protocol"), currentPP, s.apiStr("proxy_protocol") != "")
	if proxyErr != nil {
		s.apiFail(http.StatusBadRequest, "bad_request", proxyErr.Error())
		return
	}
	h.Target = &file.Target{TargetStr: target, LocalProxy: s.GetBoolNoErr("local_proxy", localProxy), ProxyProtocol: proxyProtocol}
	h.HeaderChange = firstNonEmpty(s.apiStr("header"), h.HeaderChange)
	h.ResponseHeaderChange = firstNonEmpty(s.apiStr("resp_header"), h.ResponseHeaderChange)
	h.HostChange = firstNonEmpty(s.apiStr("hostchange"), h.HostChange)
	h.PathRewrite = firstNonEmpty(s.apiStr("path_rewrite"), h.PathRewrite)
	if s.GetString("cors") != "" || s.Ctx.Input.Query("cors") != "" {
		h.Cors = s.apiStr("cors")
	}
	if s.GetString("remark") != "" || s.Ctx.Input.Query("remark") != "" {
		h.Remark = s.apiStr("remark")
	}
	h.Location = location
	h.Scheme = scheme
	h.KeyFilePath = firstNonEmpty(s.apiStr("key_file_path"), h.KeyFilePath)
	h.CertFilePath = firstNonEmpty(s.apiStr("cert_file_path"), h.CertFilePath)
	h.AutoHttps = s.GetBoolNoErr("auto_https", s.GetBoolNoErr("AutoHttps", h.AutoHttps))
	h.AutoSSL = s.GetBoolNoErr("auto_ssl", s.GetBoolNoErr("AutoSSL", h.AutoSSL))
	h.HttpsJustProxy = s.GetBoolNoErr("https_just_proxy", s.GetBoolNoErr("HttpsJustProxy", h.HttpsJustProxy))
	if h.Scheme == "http" {
		h.AutoHttps = false
		h.AutoSSL = false
	}
	file.GetDb().JsonDb.StoreHostToJsonFile()
	s.auditAPI("host.update", "host", h.Id, ownerUserIDOfClient(client), audit.ResultSuccess, "", nil)
	s.apiOK(map[string]interface{}{"id": h.Id})
}

// HostStart POST /api/v1/host/start
func (s *ApiV1Controller) HostStart() {
	if s.Ctx.Request.Method != http.MethodPost {
		s.apiFail(http.StatusMethodNotAllowed, "method_not_allowed", "仅支持 POST")
		return
	}
	h, ok := s.loadOwnedHost(s.GetIntNoErr("id"))
	if !ok {
		return
	}
	h.IsClose = false
	file.GetDb().JsonDb.StoreHostToJsonFile()
	s.auditAPI("host.start", "host", h.Id, ownerUserIDOfClient(h.Client), audit.ResultSuccess, "", nil)
	s.apiOK(map[string]interface{}{"id": h.Id, "is_close": false})
}

// HostStop POST /api/v1/host/stop
func (s *ApiV1Controller) HostStop() {
	if s.Ctx.Request.Method != http.MethodPost {
		s.apiFail(http.StatusMethodNotAllowed, "method_not_allowed", "仅支持 POST")
		return
	}
	h, ok := s.loadOwnedHost(s.GetIntNoErr("id"))
	if !ok {
		return
	}
	h.IsClose = true
	file.GetDb().JsonDb.StoreHostToJsonFile()
	s.auditAPI("host.stop", "host", h.Id, ownerUserIDOfClient(h.Client), audit.ResultSuccess, "", nil)
	s.apiOK(map[string]interface{}{"id": h.Id, "is_close": true})
}

// HostDelete POST /api/v1/host/delete
func (s *ApiV1Controller) HostDelete() {
	if s.Ctx.Request.Method != http.MethodPost {
		s.apiFail(http.StatusMethodNotAllowed, "method_not_allowed", "仅支持 POST")
		return
	}
	h, ok := s.loadOwnedHost(s.GetIntNoErr("id"))
	if !ok {
		return
	}
	owner := ownerUserIDOfClient(h.Client)
	id := h.Id
	if err := file.GetDb().DelHost(id); err != nil {
		s.auditAPI("host.delete", "host", id, owner, audit.ResultFailure, err.Error(), nil)
		s.apiFail(http.StatusInternalServerError, "internal", err.Error())
		return
	}
	s.auditAPI("host.delete", "host", id, owner, audit.ResultSuccess, "", nil)
	s.apiOK(map[string]interface{}{"id": id})
}
