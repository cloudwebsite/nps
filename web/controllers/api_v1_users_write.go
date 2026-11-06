package controllers

import (
	"net/http"
	"strings"
	"time"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/web/audit"
)

// UserCreate POST /api/v1/users/create — 仅管理员
func (s *ApiV1Controller) UserCreate() {
	if s.Ctx.Request.Method != http.MethodPost {
		s.apiFail(http.StatusMethodNotAllowed, "method_not_allowed", "仅支持 POST")
		return
	}
	if !s.isAdmin() {
		s.apiFail(http.StatusForbidden, "forbidden", "该接口仅限管理员调用")
		return
	}
	username := s.apiStr("username")
	password := s.apiStr("password")
	if username == "" || password == "" {
		s.apiFail(http.StatusBadRequest, "bad_request", "username/password 必填")
		return
	}
	hash := password
	if !common.IsBcryptHash(password) {
		h, err := common.HashPassword(password)
		if err != nil {
			s.apiFail(http.StatusInternalServerError, "internal", "密码哈希失败")
			return
		}
		hash = h
	}
	u := &file.User{
		UserName:     username,
		Password:     hash,
		Remark:       s.apiStr("remark"),
		MaxClientNum: s.GetIntNoErr("max_client_num"),
		MaxTunnelNum: s.GetIntNoErr("max_tunnel_num"),
		ExpireTime:   normalizeExpireTime(s.apiStr("expire_time")),
		CreateTime:   time.Now().Format("2006-01-02 15:04:05"),
		Status:       true,
	}
	if err := file.GetDb().NewUser(u); err != nil {
		s.auditAPI("user.create", "user", 0, 0, audit.ResultFailure, err.Error(), nil)
		s.apiFail(http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	s.auditAPI("user.create", "user", u.Id, u.Id, audit.ResultSuccess, "", map[string]string{"username": username})
	s.apiOK(map[string]interface{}{"id": u.Id, "username": u.UserName})
}

// UserUpdate POST /api/v1/users/update — 仅管理员
func (s *ApiV1Controller) UserUpdate() {
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
	u, err := file.GetDb().GetUser(id)
	if err != nil || u == nil {
		s.apiFail(http.StatusNotFound, "not_found", "用户不存在")
		return
	}
	u.Lock()
	if s.paramProvided("username") {
		u.UserName = s.apiStr("username")
	}
	if s.paramProvided("password") {
		pw := s.apiStr("password")
		if pw != "" {
			if common.IsBcryptHash(pw) {
				u.Password = pw
			} else if h, err := common.HashPassword(pw); err == nil {
				u.Password = h
			} else {
				u.Unlock()
				s.apiFail(http.StatusInternalServerError, "internal", "密码哈希失败")
				return
			}
		}
	}
	if s.paramProvided("remark") {
		u.Remark = s.apiStr("remark")
	}
	if s.paramProvided("max_client_num") {
		u.MaxClientNum = s.GetIntNoErr("max_client_num")
	}
	if s.paramProvided("max_tunnel_num") {
		u.MaxTunnelNum = s.GetIntNoErr("max_tunnel_num")
	}
	if s.paramProvided("expire_time") {
		u.ExpireTime = normalizeExpireTime(s.apiStr("expire_time"))
	}
	if s.paramProvided("status") {
		u.Status = s.GetBoolNoErr("status", u.Status)
	}
	u.Unlock()
	if err := file.GetDb().UpdateUser(u); err != nil {
		s.auditAPI("user.update", "user", id, id, audit.ResultFailure, err.Error(), nil)
		s.apiFail(http.StatusBadRequest, "bad_request", err.Error())
		return
	}
	s.auditAPI("user.update", "user", id, id, audit.ResultSuccess, "", nil)
	s.apiOK(map[string]interface{}{"id": id})
}

// UserDelete POST /api/v1/users/delete — 仅管理员；解除客户端归属后删除
func (s *ApiV1Controller) UserDelete() {
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
	if _, err := file.GetDb().GetUser(id); err != nil {
		s.apiFail(http.StatusNotFound, "not_found", "用户不存在")
		return
	}
	if err := file.GetDb().DelUser(id); err != nil {
		s.auditAPI("user.delete", "user", id, id, audit.ResultFailure, err.Error(), nil)
		s.apiFail(http.StatusInternalServerError, "internal", err.Error())
		return
	}
	s.auditAPI("user.delete", "user", id, id, audit.ResultSuccess, "", nil)
	s.apiOK(map[string]interface{}{"id": id})
}

// UserBindClient POST /api/v1/users/bind_client — 管理员把客户端归属到用户
func (s *ApiV1Controller) UserBindClient() {
	if s.Ctx.Request.Method != http.MethodPost {
		s.apiFail(http.StatusMethodNotAllowed, "method_not_allowed", "仅支持 POST")
		return
	}
	if !s.isAdmin() {
		s.apiFail(http.StatusForbidden, "forbidden", "该接口仅限管理员调用")
		return
	}
	userID := s.GetIntNoErr("user_id")
	clientID := s.GetIntNoErr("client_id")
	if userID < 0 || clientID <= 0 {
		s.apiFail(http.StatusBadRequest, "bad_request", "需要 user_id（可 0 解绑）与 client_id")
		return
	}
	if userID > 0 {
		if _, err := file.GetDb().GetUser(userID); err != nil {
			s.apiFail(http.StatusNotFound, "not_found", "用户不存在")
			return
		}
	}
	c, err := file.GetDb().GetClient(clientID)
	if err != nil || c == nil {
		s.apiFail(http.StatusNotFound, "not_found", "客户端不存在")
		return
	}
	c.RLock()
	prevOwner := c.UserId
	c.RUnlock()
	if userID > 0 && userID != prevOwner {
		if err := file.GetDb().CheckCanAddClient(userID); err != nil {
			s.apiFail(http.StatusConflict, "quota_exceeded", err.Error())
			return
		}
	}
	c.Lock()
	c.UserId = userID
	c.Unlock()
	file.GetDb().JsonDb.StoreClientsToJsonFile()
	s.auditAPI("user.bind_client", "client", clientID, userID, audit.ResultSuccess, "", map[string]string{
		"user_id": strings.TrimSpace(s.GetString("user_id")),
	})
	s.apiOK(map[string]interface{}{"client_id": clientID, "user_id": userID})
}
