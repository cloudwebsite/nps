package controllers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"ehang.io/nps/lib/acl"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/lib/rate"
	"ehang.io/nps/server"
	"ehang.io/nps/web/audit"
)

// Client GET /api/v1/client?id=
func (s *ApiV1Controller) Client() {
	id := s.GetIntNoErr("id")
	if id <= 0 {
		s.apiFail(http.StatusBadRequest, "bad_request", "缺少 id")
		return
	}
	c, err := file.GetDb().GetClient(id)
	if err != nil || c == nil {
		s.apiFail(http.StatusNotFound, "not_found", "客户端不存在")
		return
	}
	if !clientAllowed(s.allowedClientIDs(), c.Id) {
		s.apiFail(http.StatusForbidden, "forbidden", "无权访问该客户端")
		return
	}
	c.RLock()
	defer c.RUnlock()
	u, p := "", ""
	cryptOn, compress := false, false
	if c.Cnf != nil {
		u, p, cryptOn, compress = c.Cnf.U, c.Cnf.P, c.Cnf.Crypt, c.Cnf.Compress
	}
	s.apiOK(map[string]interface{}{
		"id": c.Id, "remark": c.Remark, "status": c.Status, "is_connect": c.IsConnect,
		"user_id": c.UserId, "verify_key": c.VerifyKey, "addr": c.Addr, "version": c.Version,
		"rate_limit": c.RateLimit, "max_conn": c.MaxConn, "max_tunnel": c.MaxTunnelNum,
		"config_conn_allow": c.ConfigConnAllow,
		"expire_time": c.ExpireTime, "create_time": c.CreateTime,
		"basic_user": u, "basic_pass_set": p != "", "crypt": cryptOn, "compress": compress,
	})
}

// ClientCreate POST /api/v1/client/create — 管理员可指定 user_id；用户 Token 只能建到自己名下
func (s *ApiV1Controller) ClientCreate() {
	if s.Ctx.Request.Method != http.MethodPost {
		s.apiFail(http.StatusMethodNotAllowed, "method_not_allowed", "仅支持 POST")
		return
	}
	if !s.isAdmin() && !s.userAPIAuthorized {
		s.apiFail(http.StatusForbidden, "forbidden", "缺少有效凭据")
		return
	}
	webUserName := s.apiStr("web_username")
	webPassword := s.apiStr("web_password")
	if msg := validateClientWebPair(webUserName, webPassword); msg != "" {
		s.apiFail(http.StatusBadRequest, "bad_request", msg)
		return
	}
	if webPassword != "" && !common.IsBcryptHash(webPassword) {
		if h, err := common.HashPassword(webPassword); err == nil {
			webPassword = h
		}
	}
	ownerID := 0
	if s.isAdmin() {
		ownerID = s.GetIntNoErr("user_id")
		if ownerID > 0 {
			if _, err := file.GetDb().GetUser(ownerID); err != nil {
				s.apiFail(http.StatusBadRequest, "bad_request", "user_id 无效")
				return
			}
		}
	} else {
		ownerID = s.apiUserID
	}
	if err := file.GetDb().CheckCanAddClient(ownerID); err != nil {
		s.apiFail(http.StatusConflict, "quota_exceeded", err.Error())
		return
	}
	t := &file.Client{
		VerifyKey: s.apiStr("vkey"),
		Status:    true,
		Remark:    s.apiStr("remark"),
		Cnf: &file.Config{
			U:        s.apiStr("u"),
			P:        s.apiStr("p"),
			Compress: s.GetBoolNoErr("compress"),
			Crypt:    s.GetBoolNoErr("crypt"),
		},
		ConfigConnAllow: s.GetBoolNoErr("config_conn_allow", true),
		RateLimit:       s.GetIntNoErr("rate_limit"),
		MaxConn:         s.GetIntNoErr("max_conn"),
		WebUserName:     webUserName,
		WebPassword:     webPassword,
		MaxTunnelNum:    s.GetIntNoErr("max_tunnel"),
		Flow:            &file.Flow{FlowLimit: int64(s.GetIntNoErr("flow_limit"))},
		BlackIpList:     normalizeACLRules(s.GetString("blackiplist")),
		IpWhite:         s.GetBoolNoErr("ipwhite"),
		IpWhitePass:     s.apiStr("ipwhitepass"),
		IpWhiteList:     normalizeACLRules(s.GetString("ipwhitelist")),
		IpListPriority:  acl.NormalizePriority(s.GetIntNoErr("iplists_priority")),
		ExpireTime:      normalizeExpireTime(s.apiStr("expire_time")),
		CreateTime:      time.Now().Format("2006-01-02 15:04:05"),
		UserId:          ownerID,
	}
	if !s.isAdmin() {
		// 普通用户不得自提额度字段外的高权限项：限流等仍允许；vkey 可空由服务端生成
		t.Flow.FlowLimit = 0
	}
	if err := file.GetDb().NewClient(t); err != nil {
		s.auditAPI("client.create", "client", 0, t.UserId, audit.ResultFailure, err.Error(), nil)
		s.apiFail(http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	s.auditAPI("client.create", "client", t.Id, t.UserId, audit.ResultSuccess, "", nil)
	s.apiOK(map[string]interface{}{"id": t.Id, "verify_key": t.VerifyKey})
}

// ClientUpdate POST /api/v1/client/update
func (s *ApiV1Controller) ClientUpdate() {
	if s.Ctx.Request.Method != http.MethodPost {
		s.apiFail(http.StatusMethodNotAllowed, "method_not_allowed", "仅支持 POST")
		return
	}
	id := s.GetIntNoErr("id")
	c, err := file.GetDb().GetClient(id)
	if err != nil || c == nil {
		s.apiFail(http.StatusNotFound, "not_found", "客户端不存在")
		return
	}
	if !clientAllowed(s.allowedClientIDs(), c.Id) {
		s.apiFail(http.StatusForbidden, "forbidden", "无权操作该客户端")
		return
	}
	if c.Cnf == nil {
		c.Cnf = &file.Config{}
	}
	if s.paramProvided("remark") {
		c.Remark = s.apiStr("remark")
	}
	if c.Flow == nil {
		c.Flow = &file.Flow{}
	}
	if s.isAdmin() {
		if s.paramProvided("vkey") {
			vkey := s.apiStr("vkey")
			if vkey != "" && !file.GetDb().VerifyVkey(vkey, c.Id) {
				s.apiFail(http.StatusConflict, "conflict", "Vkey duplicate")
				return
			}
			if vkey != "" {
				c.VerifyKey = vkey
			}
		}
		if s.paramProvided("flow_limit") {
			c.Flow.FlowLimit = int64(s.GetIntNoErr("flow_limit"))
		}
		if s.paramProvided("rate_limit") {
			c.RateLimit = s.GetIntNoErr("rate_limit")
		}
		if s.paramProvided("max_conn") {
			c.MaxConn = s.GetIntNoErr("max_conn")
		}
		if s.paramProvided("max_tunnel") {
			c.MaxTunnelNum = s.GetIntNoErr("max_tunnel")
		}
		if s.paramProvided("user_id") {
			uid := s.GetIntNoErr("user_id")
			if uid > 0 {
				if _, err := file.GetDb().GetUser(uid); err != nil {
					s.apiFail(http.StatusBadRequest, "bad_request", "user_id 无效")
					return
				}
			}
			if uid != c.UserId && uid > 0 {
				if err := file.GetDb().CheckCanAddClient(uid); err != nil {
					s.apiFail(http.StatusConflict, "quota_exceeded", err.Error())
					return
				}
			}
			c.UserId = uid
		}
	}
	if s.paramProvided("u") {
		c.Cnf.U = s.apiStr("u")
	}
	if s.paramProvided("p") {
		c.Cnf.P = s.apiStr("p")
	}
	c.Cnf.Compress = s.GetBoolNoErr("compress", c.Cnf.Compress)
	c.Cnf.Crypt = s.GetBoolNoErr("crypt", c.Cnf.Crypt)
	c.ConfigConnAllow = s.GetBoolNoErr("config_conn_allow", c.ConfigConnAllow)
	c.IpWhite = s.GetBoolNoErr("ipwhite", c.IpWhite)
	if s.paramProvided("ipwhitepass") {
		c.IpWhitePass = s.apiStr("ipwhitepass")
	}
	if s.paramProvided("blackiplist") {
		c.BlackIpList = normalizeACLRules(s.GetString("blackiplist"))
	}
	if s.paramProvided("ipwhitelist") {
		c.IpWhiteList = normalizeACLRules(s.GetString("ipwhitelist"))
	}
	if s.paramProvided("iplists_priority") {
		c.IpListPriority = acl.NormalizePriority(s.GetIntNoErr("iplists_priority"))
	}
	if s.paramProvided("expire_time") {
		c.ExpireTime = normalizeExpireTime(s.apiStr("expire_time"))
	}
	if s.paramProvided("web_username") || s.paramProvided("web_password") {
		webUserName := c.WebUserName
		webPassword := ""
		if s.paramProvided("web_username") {
			webUserName = s.apiStr("web_username")
		}
		if s.paramProvided("web_password") {
			webPassword = s.apiStr("web_password")
		}
		if webPassword != "" {
			if msg := validateClientWebPair(webUserName, webPassword); msg != "" && webUserName != "" {
				s.apiFail(http.StatusBadRequest, "bad_request", msg)
				return
			}
			if !common.IsBcryptHash(webPassword) {
				if h, err := common.HashPassword(webPassword); err == nil {
					webPassword = h
				}
			}
			c.WebPassword = webPassword
		}
		if s.paramProvided("web_username") {
			if webUserName != "" && !file.GetDb().VerifyUserName(webUserName, c.Id) {
				s.apiFail(http.StatusConflict, "conflict", "web login username duplicate")
				return
			}
			c.WebUserName = webUserName
		}
	}
	if c.Rate != nil {
		c.Rate.Stop()
	}
	if c.RateLimit > 0 {
		c.Rate = rate.NewRate(int64(c.RateLimit * 1024))
	} else {
		c.Rate = rate.NewRate((2 << 23) * 1024)
	}
	c.Rate.Start()
	file.GetDb().JsonDb.StoreClientsToJsonFile()
	s.auditAPI("client.update", "client", c.Id, c.UserId, audit.ResultSuccess, "", nil)
	s.apiOK(map[string]interface{}{"id": c.Id})
}

// ClientStatus POST /api/v1/client/status
func (s *ApiV1Controller) ClientStatus() {
	if s.Ctx.Request.Method != http.MethodPost {
		s.apiFail(http.StatusMethodNotAllowed, "method_not_allowed", "仅支持 POST")
		return
	}
	id := s.GetIntNoErr("id")
	c, err := file.GetDb().GetClient(id)
	if err != nil || c == nil {
		s.apiFail(http.StatusNotFound, "not_found", "客户端不存在")
		return
	}
	if !clientAllowed(s.allowedClientIDs(), c.Id) {
		s.apiFail(http.StatusForbidden, "forbidden", "无权操作该客户端")
		return
	}
	c.Status = s.GetBoolNoErr("status", c.Status)
	if !c.Status {
		server.DelClientConnect(c.Id)
	}
	file.GetDb().JsonDb.StoreClientsToJsonFile()
	s.auditAPI("client.status", "client", c.Id, c.UserId, audit.ResultSuccess, "", map[string]string{
		"status": strconv.FormatBool(c.Status),
	})
	s.apiOK(map[string]interface{}{"id": c.Id, "status": c.Status})
}

// ClientDelete POST /api/v1/client/delete — 仅管理员
func (s *ApiV1Controller) ClientDelete() {
	if s.Ctx.Request.Method != http.MethodPost {
		s.apiFail(http.StatusMethodNotAllowed, "method_not_allowed", "仅支持 POST")
		return
	}
	if !s.isAdmin() {
		s.apiFail(http.StatusForbidden, "forbidden", "该接口仅限管理员调用")
		return
	}
	id := s.GetIntNoErr("id")
	if id <= 0 {
		s.apiFail(http.StatusBadRequest, "bad_request", "缺少 id")
		return
	}
	owner := 0
	if c, err := file.GetDb().GetClient(id); err == nil && c != nil {
		owner = c.UserId
	} else {
		s.apiFail(http.StatusNotFound, "not_found", "客户端不存在")
		return
	}
	if err := file.GetDb().DelClient(id); err != nil {
		s.auditAPI("client.delete", "client", id, owner, audit.ResultFailure, err.Error(), nil)
		s.apiFail(http.StatusInternalServerError, "internal", err.Error())
		return
	}
	server.DelTunnelAndHostByClientId(id, false)
	server.DelClientConnect(id)
	s.auditAPI("client.delete", "client", id, owner, audit.ResultSuccess, "", nil)
	s.apiOK(map[string]interface{}{"id": id})
}

// ClientRemoteUpdate POST /api/v1/client/remote_update — 管理员；需 allow_remote_update
func (s *ApiV1Controller) ClientRemoteUpdate() {
	if s.Ctx.Request.Method != http.MethodPost {
		s.apiFail(http.StatusMethodNotAllowed, "method_not_allowed", "仅支持 POST")
		return
	}
	if !s.isAdmin() {
		s.apiFail(http.StatusForbidden, "forbidden", "该接口仅限管理员调用")
		return
	}
	if !file.EffectiveAllowRemoteUpdate() {
		s.apiFail(http.StatusForbidden, "forbidden", "服务端已全局禁止远程升级")
		return
	}
	id := s.GetIntNoErr("id")
	if id <= 0 {
		s.apiFail(http.StatusBadRequest, "bad_request", "缺少 id")
		return
	}
	c, err := file.GetDb().GetClient(id)
	if err != nil || c == nil {
		s.apiFail(http.StatusNotFound, "not_found", "客户端不存在")
		return
	}
	if !c.Status || c.NoStore {
		s.apiFail(http.StatusConflict, "conflict", "客户端已停用或不支持远程升级")
		return
	}
	out, err := server.RequestClientUpdate(id)
	if err != nil {
		s.auditAPI("client.remote_update", "client", id, c.UserId, audit.ResultFailure, err.Error(), nil)
		s.apiFail(http.StatusBadGateway, "upstream", err.Error())
		return
	}
	s.auditAPI("client.remote_update", "client", id, c.UserId, audit.ResultSuccess, out.Status, map[string]string{
		"version": c.Version,
		"status":  out.Status,
		"channel": out.Channel,
	})
	code := http.StatusOK
	if out.Status == "fail" {
		code = http.StatusBadGateway
	}
	payload := map[string]interface{}{
		"id": id, "status": out.Status, "channel": out.Channel, "message": out.Message,
	}
	if out.Status == "fail" {
		s.Ctx.Output.SetStatus(code)
		s.apiFail(code, "update_failed", out.Message)
		return
	}
	s.apiOK(payload)
}

func splitLines(raw string) []string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	return RemoveRepeatedElement(strings.Split(raw, "\n"))
}
