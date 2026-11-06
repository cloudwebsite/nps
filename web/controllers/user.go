package controllers

import (
	"net/http"
	"strings"
	"time"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/file"
)

// UserController 面板用户管理；仅管理员。
type UserController struct {
	BaseController
}

func (s *UserController) Prepare() {
	s.BaseController.Prepare()
	if isAdmin, ok := s.GetSession("isAdmin").(bool); !ok || !isAdmin {
		s.denyForbidden()
	}
}

func (s *UserController) List() {
	if s.Ctx.Request.Method == "GET" {
		s.Data["menu"] = "user"
		s.SetInfo("user list")
		s.display("user/list")
		return
	}
	start, length := s.GetAjaxParams()
	list, cnt := file.GetDb().GetUserList(start, length, s.getEscapeString("search"))
	rows := make([]map[string]interface{}, 0, len(list))
	for _, u := range list {
		u.RLock()
		owned := len(file.GetDb().UserClientIds(u.Id))
		rows = append(rows, map[string]interface{}{
			"Id": u.Id, "UserName": u.UserName, "Status": u.Status,
			"Remark": u.Remark, "MaxClientNum": u.MaxClientNum, "MaxTunnelNum": u.MaxTunnelNum,
			"ExpireTime": u.ExpireTime, "CreateTime": u.CreateTime,
			"ApiKeyEnabled": strings.TrimSpace(u.APIKeyHash) != "",
			"ClientCount":   owned,
		})
		u.RUnlock()
	}
	s.AjaxTable(rows, cnt, cnt, nil)
}

func (s *UserController) Add() {
	if s.Ctx.Request.Method == "GET" {
		s.Data["menu"] = "user"
		s.SetInfo("add user")
		s.display("user/add")
		return
	}
	username := s.getEscapeString("username")
	password := s.GetString("password")
	if username == "" || strings.TrimSpace(password) == "" {
		s.AjaxErr("username/password required")
		return
	}
	hash := password
	if !common.IsBcryptHash(password) {
		if h, err := common.HashPassword(password); err == nil {
			hash = h
		} else {
			s.AjaxErr("hash password failed")
			return
		}
	}
	u := &file.User{
		UserName:     username,
		Password:     hash,
		Remark:       s.getEscapeString("remark"),
		MaxClientNum: s.GetIntNoErr("max_client_num"),
		MaxTunnelNum: s.GetIntNoErr("max_tunnel_num"),
		ExpireTime:   normalizeExpireTime(s.getEscapeString("expire_time")),
		CreateTime:   time.Now().Format("2006-01-02 15:04:05"),
		Status:       true,
	}
	if err := file.GetDb().NewUser(u); err != nil {
		s.AjaxErr(err.Error())
		return
	}
	s.auditPanel("user.create", "user", u.Id, u.Id, "success", "", nil)
	s.AjaxOkWithId("add success", u.Id)
}

func (s *UserController) Edit() {
	id := s.GetIntNoErr("id")
	u, err := file.GetDb().GetUser(id)
	if err != nil || u == nil {
		if s.Ctx.Request.Method == "GET" {
			s.error()
			return
		}
		s.AjaxErr("user not found")
		return
	}
	if s.Ctx.Request.Method == "GET" {
		s.Data["menu"] = "user"
		s.Data["u"] = u
		s.SetInfo("edit user")
		s.display("user/edit")
		return
	}
	u.Lock()
	u.UserName = s.getEscapeString("username")
	if pw := strings.TrimSpace(s.GetString("password")); pw != "" {
		if common.IsBcryptHash(pw) {
			u.Password = pw
		} else if h, err := common.HashPassword(pw); err == nil {
			u.Password = h
		} else {
			u.Unlock()
			s.AjaxErr("hash password failed")
			return
		}
	}
	u.Remark = s.getEscapeString("remark")
	u.MaxClientNum = s.GetIntNoErr("max_client_num")
	u.MaxTunnelNum = s.GetIntNoErr("max_tunnel_num")
	u.ExpireTime = normalizeExpireTime(s.getEscapeString("expire_time"))
	u.Status = s.GetBoolNoErr("status", u.Status)
	u.Unlock()
	if err := file.GetDb().UpdateUser(u); err != nil {
		s.AjaxErr(err.Error())
		return
	}
	s.auditPanel("user.update", "user", u.Id, u.Id, "success", "", nil)
	s.AjaxOk("save success")
}

func (s *UserController) Del() {
	id := s.GetIntNoErr("id")
	if err := file.GetDb().DelUser(id); err != nil {
		s.AjaxErr(err.Error())
		return
	}
	s.auditPanel("user.delete", "user", id, id, "success", "", nil)
	s.AjaxOk("delete success")
}

func (s *UserController) BindClient() {
	if s.Ctx.Request.Method != http.MethodPost {
		s.AjaxErr("POST only")
		return
	}
	userID := s.GetIntNoErr("user_id")
	clientID := s.GetIntNoErr("client_id")
	if clientID <= 0 {
		s.AjaxErr("client_id required")
		return
	}
	if userID > 0 {
		if _, err := file.GetDb().GetUser(userID); err != nil {
			s.AjaxErr("user not found")
			return
		}
	}
	c, err := file.GetDb().GetClient(clientID)
	if err != nil || c == nil {
		s.AjaxErr("client not found")
		return
	}
	c.RLock()
	prevOwner := c.UserId
	c.RUnlock()
	if userID > 0 && userID != prevOwner {
		if err := file.GetDb().CheckCanAddClient(userID); err != nil {
			s.AjaxErr(err.Error())
			return
		}
	}
	c.Lock()
	c.UserId = userID
	c.Unlock()
	file.GetDb().JsonDb.StoreClientsToJsonFile()
	s.auditPanel("user.bind_client", "client", clientID, userID, "success", "", nil)
	s.AjaxOk("bind success")
}

func (s *UserController) ChangeStatus() {
	id := s.GetIntNoErr("id")
	u, err := file.GetDb().GetUser(id)
	if err != nil || u == nil {
		s.AjaxErr("user not found")
		return
	}
	u.Lock()
	u.Status = s.GetBoolNoErr("status")
	u.Unlock()
	if err := file.GetDb().UpdateUser(u); err != nil {
		s.AjaxErr(err.Error())
		return
	}
	s.auditPanel("user.status", "user", id, id, "success", "", nil)
	s.AjaxOk("modified success")
}
