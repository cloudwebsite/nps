package controllers

import (
	"net/http"
	"strings"

	"ehang.io/nps/lib/file"
	"ehang.io/nps/web/audit"
	"github.com/astaxie/beego/logs"
)

func (s *ApiV1Controller) pageParams() (offset, limit int, search string) {
	offset = s.GetIntNoErr("offset", 0)
	limit = s.GetIntNoErr("limit", apiV1DefaultLimit)
	if limit <= 0 {
		limit = apiV1DefaultLimit
	}
	if limit > apiV1MaxLimit {
		limit = apiV1MaxLimit
	}
	search = strings.TrimSpace(s.GetString("search"))
	return
}

func (s *ApiV1Controller) allowedClientIDs() map[int]struct{} {
	if s.isAdmin() {
		return nil
	}
	return file.GetDb().UserClientIds(s.apiUserID)
}

func clientAllowed(allowed map[int]struct{}, clientID int) bool {
	if allowed == nil {
		return true
	}
	_, ok := allowed[clientID]
	return ok
}

// Tunnels GET /api/v1/tunnels
func (s *ApiV1Controller) Tunnels() {
	offset, limit, search := s.pageParams()
	clientFilter := s.GetIntNoErr("client_id")
	allowed := s.allowedClientIDs()

	var list []*file.Tunnel
	var total int
	if s.isAdmin() {
		list, total = file.GetDb().GetTaskList(offset, limit, clientFilter, search)
	} else {
		all, _ := file.GetDb().GetTaskList(0, 100000, clientFilter, search)
		filtered := make([]*file.Tunnel, 0)
		for _, t := range all {
			if t == nil || t.Client == nil {
				continue
			}
			if clientAllowed(allowed, t.Client.Id) {
				filtered = append(filtered, t)
			}
		}
		total = len(filtered)
		if offset >= total {
			list = []*file.Tunnel{}
		} else {
			end := offset + limit
			if end > total {
				end = total
			}
			list = filtered[offset:end]
		}
	}

	rows := make([]map[string]interface{}, 0, len(list))
	for _, t := range list {
		clientID := 0
		if t.Client != nil {
			clientID = t.Client.Id
		}
		target := ""
		if t.Target != nil {
			target = t.Target.TargetStr
		}
		rows = append(rows, map[string]interface{}{
			"id": t.Id, "mode": t.Mode, "port": t.Port, "status": t.Status,
			"run_status": t.RunStatus, "remark": t.Remark, "client_id": clientID,
			"target": target, "idle_auto_stop_minutes": t.IdleAutoStopMinutes,
		})
	}
	s.apiOK(map[string]interface{}{
		"items": rows, "total": total, "offset": offset, "limit": limit,
	})
}

// Tunnel GET /api/v1/tunnel?id=
func (s *ApiV1Controller) Tunnel() {
	id := s.GetIntNoErr("id")
	if id <= 0 {
		s.apiFail(http.StatusBadRequest, "bad_request", "缺少 id")
		return
	}
	t, err := file.GetDb().GetTask(id)
	if err != nil || t == nil {
		s.apiFail(http.StatusNotFound, "not_found", "隧道不存在")
		return
	}
	clientID := 0
	if t.Client != nil {
		clientID = t.Client.Id
	}
	if !clientAllowed(s.allowedClientIDs(), clientID) {
		s.apiFail(http.StatusForbidden, "forbidden", "无权访问该隧道")
		return
	}
	target := ""
	localProxy := false
	if t.Target != nil {
		target = t.Target.TargetStr
		localProxy = t.Target.LocalProxy
	}
	s.apiOK(map[string]interface{}{
		"id": t.Id, "mode": t.Mode, "port": t.Port, "server_ip": t.ServerIp,
		"status": t.Status, "run_status": t.RunStatus, "remark": t.Remark,
		"client_id": clientID, "target": target, "local_proxy": localProxy,
		"password_set": t.Password != "", "idle_auto_stop_minutes": t.IdleAutoStopMinutes,
	})
}

// Hosts GET /api/v1/hosts
func (s *ApiV1Controller) Hosts() {
	offset, limit, search := s.pageParams()
	clientFilter := s.GetIntNoErr("client_id")
	allowed := s.allowedClientIDs()

	var list []*file.Host
	var total int
	if s.isAdmin() {
		list, total = file.GetDb().GetHost(offset, limit, clientFilter, search, "", "")
	} else {
		all, _ := file.GetDb().GetHost(0, 100000, clientFilter, search, "", "")
		filtered := make([]*file.Host, 0)
		for _, h := range all {
			if h == nil || h.Client == nil {
				continue
			}
			if clientAllowed(allowed, h.Client.Id) {
				filtered = append(filtered, h)
			}
		}
		total = len(filtered)
		if offset >= total {
			list = []*file.Host{}
		} else {
			end := offset + limit
			if end > total {
				end = total
			}
			list = filtered[offset:end]
		}
	}

	rows := make([]map[string]interface{}, 0, len(list))
	for _, h := range list {
		clientID := 0
		if h.Client != nil {
			clientID = h.Client.Id
		}
		target := ""
		if h.Target != nil {
			target = h.Target.TargetStr
		}
		rows = append(rows, map[string]interface{}{
			"id": h.Id, "host": h.Host, "scheme": h.Scheme, "location": h.Location,
			"remark": h.Remark, "client_id": clientID, "target": target,
			"is_close": h.IsClose,
		})
	}
	s.apiOK(map[string]interface{}{
		"items": rows, "total": total, "offset": offset, "limit": limit,
	})
}

// Host GET /api/v1/host?id=
func (s *ApiV1Controller) Host() {
	id := s.GetIntNoErr("id")
	if id <= 0 {
		s.apiFail(http.StatusBadRequest, "bad_request", "缺少 id")
		return
	}
	h, err := file.GetDb().GetHostById(id)
	if err != nil || h == nil {
		s.apiFail(http.StatusNotFound, "not_found", "域名不存在")
		return
	}
	clientID := 0
	if h.Client != nil {
		clientID = h.Client.Id
	}
	if !clientAllowed(s.allowedClientIDs(), clientID) {
		s.apiFail(http.StatusForbidden, "forbidden", "无权访问该域名")
		return
	}
		target := ""
		proxyProtocol := 0
		if h.Target != nil {
			target = h.Target.TargetStr
			proxyProtocol = h.Target.ProxyProtocol
		}
		s.apiOK(map[string]interface{}{
			"id": h.Id, "host": h.Host, "scheme": h.Scheme, "location": h.Location,
			"remark": h.Remark, "client_id": clientID, "target": target,
			"proxy_protocol": proxyProtocol, "https_just_proxy": h.HttpsJustProxy,
			"header_change": h.HeaderChange, "response_header_change": h.ResponseHeaderChange,
			"host_change": h.HostChange, "path_rewrite": h.PathRewrite, "cors": h.Cors,
			"is_close": h.IsClose, "auto_https": h.AutoHttps, "auto_ssl": h.AutoSSL,
		})
	}

// Audit GET /api/v1/audit — 管理员；audit_log_path 未配置时返回空列表
func (s *ApiV1Controller) Audit() {
	if !s.isAdmin() {
		s.apiFail(http.StatusForbidden, "forbidden", "该接口仅限管理员调用")
		return
	}
	offset, limit, search := s.pageParams()
	page, err := audit.List(audit.Filter{
		Offset:       offset,
		Limit:        limit,
		Search:       search,
		Action:       strings.TrimSpace(s.GetString("action")),
		ActorType:    strings.TrimSpace(s.GetString("actor_type")),
		ResourceType: strings.TrimSpace(s.GetString("resource_type")),
		Result:       strings.TrimSpace(s.GetString("result")),
	})
	if err != nil {
		logs.Warn("audit list: %v", err)
		s.apiFail(http.StatusInternalServerError, "internal", "读取审计日志失败")
		return
	}
	s.apiOK(map[string]interface{}{
		"items": page.Events, "total": page.Total, "offset": offset, "limit": limit,
		"enabled": audit.Path() != "",
	})
}

func (s *ApiV1Controller) auditAPI(action, resourceType string, resourceID, ownerUserID int, result, errMsg string, details map[string]string) {
	actorType, actorID, actorName := "api", 0, "api"
	if s.userAPIAuthorized {
		actorType, actorID = "api-user", s.apiUserID
		actorName = "api-user"
		if u, err := file.GetDb().GetUser(s.apiUserID); err == nil && u != nil {
			u.RLock()
			actorName = u.UserName
			u.RUnlock()
		}
	} else if s.apiAuthorized {
		actorType, actorName = "admin", "admin"
	}
	_ = audit.Append(audit.Event{
		ActorType: actorType, ActorID: actorID, ActorName: actorName,
		SourceIP: s.Ctx.Input.IP(), Method: s.Ctx.Request.Method, Path: s.Ctx.Request.URL.Path,
		Action: action, ResourceType: resourceType, ResourceID: resourceID,
		OwnerUserID: ownerUserID, Result: result, Error: errMsg, Details: details,
	})
}
