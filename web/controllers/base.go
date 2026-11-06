package controllers

import (
	"html"
	"math"
	"strconv"
	"strings"
	"time"

	"ehang.io/nps/bridge"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/crypt"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/lib/version"
	"ehang.io/nps/server"
	"ehang.io/nps/web/audit"
	"github.com/astaxie/beego"
)

type BaseController struct {
	beego.Controller
	controllerName string
	actionName     string
}

// 初始化参数
func (s *BaseController) Prepare() {
	s.Data["web_base_url"] = beego.AppConfig.String("web_base_url")
	controllerName, actionName := s.GetControllerAndAction()
	s.controllerName = strings.ToLower(controllerName[0 : len(controllerName)-10])
	s.actionName = strings.ToLower(actionName)
	// web api verify
	// param 1 is md5(authKey+Current timestamp)
	// param 2 is timestamp (It's limited to 20 seconds.)
	md5Key := s.getEscapeString("auth_key")
	timestamp := s.GetIntNoErr("timestamp")
	configKey := beego.AppConfig.String("auth_key")
	if configKey == "" {
		configKey = crypt.GetRandomString(64)
	}
	timeNowUnix := time.Now().Unix()
	if !(md5Key != "" && (math.Abs(float64(timeNowUnix-int64(timestamp))) <= 20) && (crypt.Md5(configKey+strconv.Itoa(timestamp)) == md5Key)) {
		if s.GetSession("auth") != true {
			s.Redirect(beego.AppConfig.String("web_base_url")+"/login/index", 302)
		}
	} else {
		s.SetSession("isAdmin", true)
		s.Data["isAdmin"] = true
	}
	if s.GetSession("isAdmin") != nil && !s.GetSession("isAdmin").(bool) {
		s.Ctx.Input.SetData("client_id", s.GetSession("clientId").(int))
		s.Ctx.Input.SetParam("client_id", strconv.Itoa(s.GetSession("clientId").(int)))
		s.Data["isAdmin"] = false
		s.Data["username"] = s.GetSession("username")
		s.CheckUserAuth()
	} else {
		s.Data["isAdmin"] = true
	}
	globalSettings := file.GetDb().GetGlobalSettings()
	s.Data["allow_user_login"] = globalSettings.AllowUserLogin
	s.Data["allow_flow_limit"] = globalSettings.AllowFlowLimit
	s.Data["allow_rate_limit"] = globalSettings.AllowRateLimit
	s.Data["allow_connection_num_limit"] = globalSettings.AllowConnectionNumLimit
	s.Data["allow_multi_ip"] = globalSettings.AllowMultiIp
	s.Data["system_info_display"] = globalSettings.SystemInfoDisplay
	s.Data["allow_tunnel_num_limit"] = globalSettings.AllowTunnelNumLimit
	s.Data["allow_local_proxy"] = globalSettings.AllowLocalProxy
	s.Data["allow_user_change_username"] = globalSettings.AllowUserChangeUsername
	httpPort := beego.AppConfig.String("http_proxy_port")
	if httpPort != "80" && globalSettings.ShowHttpProxyPort {
		s.Data["http_proxy_port"] = ":" + httpPort
	}
}

// 加载模板
func (s *BaseController) display(tpl ...string) {
	s.Data["web_base_url"] = beego.AppConfig.String("web_base_url")
	var tplname string
	if s.Data["menu"] == nil {
		s.Data["menu"] = s.actionName
	}
	if len(tpl) > 0 {
		tplname = strings.Join([]string{tpl[0], "html"}, ".")
	} else {
		tplname = s.controllerName + "/" + s.actionName + ".html"
	}
	ip := s.Ctx.Request.Host
	s.Data["ip"] = common.GetIpByAddr(ip)

	global := file.GetDb().GetGlobal()
	if global != nil && global.ServerUrl != "" && global.ServerUrl != ip {
		// 替换掉 http:// 或者 https://
		ip = global.ServerUrl
		ip = strings.ReplaceAll(ip, "http://", "")
		ip = strings.ReplaceAll(ip, "https://", "")
		s.Data["ip"] = ip
	}

	s.Data["bridgeType"] = beego.AppConfig.String("bridge_type")
	if common.IsWindows() {
		s.Data["win"] = "npc.exe"
	} else {
		s.Data["win"] = "./npc"
	}

	s.Data["p"] = strconv.Itoa(server.Bridge.TunnelPort)
	s.Data["version"] = version.VERSION

	if bridge.ServerTlsEnable {
		tlsPort := strconv.Itoa(beego.AppConfig.DefaultInt("tls_bridge_port", 8025))
		s.Data["tls_p"] = tlsPort
		s.Data["tls_enable"] = true
		s.Data["p1"] = strconv.Itoa(server.Bridge.TunnelPort) + " / " + tlsPort
	} else {
		s.Data["tls_enable"] = false
		s.Data["p1"] = strconv.Itoa(server.Bridge.TunnelPort)
	}

	s.Data["proxyPort"] = beego.AppConfig.String("hostPort")
	// 顶栏环境标识 / 侧栏副标题（空则模板回退语言包或默认文案；由 html/template 转义）
	s.Data["web_welcome"] = file.EffectiveWebWelcome()
	s.Data["web_subtitle"] = file.EffectiveWebSubtitle()
	// 面板产品名（global.json AppName，热更新）
	s.Data["app_name"] = file.EffectiveAppName()
	s.Layout = "public/layout.html"
	s.TplName = tplname
}

// 错误
func (s *BaseController) error() {
	s.Data["web_base_url"] = beego.AppConfig.String("web_base_url")
	s.Layout = "public/layout.html"
	s.TplName = "public/error.html"
}

// getEscapeString
func (s *BaseController) getEscapeString(key string) string {
	return html.EscapeString(s.GetString(key))
}

// 去掉没有err返回值的int
// 优先读 Prepare 里 SetParam 强制写入的值（如非管理员的 client_id），避免表单绕过归属。
func (s *BaseController) GetIntNoErr(key string, def ...int) int {
	if s.Ctx != nil && s.Ctx.Input != nil {
		if p := s.Ctx.Input.Param(key); p != "" {
			if val, err := strconv.Atoi(p); err == nil {
				return val
			}
		}
	}
	strv := s.Ctx.Input.Query(key)
	if len(strv) == 0 && len(def) > 0 {
		return def[0]
	}
	val, _ := strconv.Atoi(strv)
	return val
}

// 获取去掉错误的bool值
func (s *BaseController) GetBoolNoErr(key string, def ...bool) bool {
	if s.Ctx != nil && s.Ctx.Input != nil {
		if p := s.Ctx.Input.Param(key); p != "" {
			if val, err := strconv.ParseBool(p); err == nil {
				return val
			}
		}
	}
	strv := s.Ctx.Input.Query(key)
	if len(strv) == 0 && len(def) > 0 {
		return def[0]
	}
	val, _ := strconv.ParseBool(strv)
	return val
}

// ajax正确返回
func (s *BaseController) AjaxOk(str string) {
	s.Data["json"] = ajax(str, 1)
	s.ServeJSON()
	s.StopRun()
}

// ajax正确返回
func (s *BaseController) AjaxOkWithId(str string, id int) {
	s.Data["json"] = ajaxWithId(str, 1, id)
	s.ServeJSON()
	s.StopRun()
}

// ajax错误返回
func (s *BaseController) AjaxErr(str string) {
	s.Data["json"] = ajax(str, 0)
	s.ServeJSON()
	s.StopRun()
}

// auditPanel 面板管理面审计；audit_log_path 未配置时无操作。
func (s *BaseController) auditPanel(action, resourceType string, resourceID, ownerUserID int, result, errMsg string, details map[string]string) {
	actorType, actorID, actorName := "web", 0, "user"
	if isAdmin, ok := s.GetSession("isAdmin").(bool); ok && isAdmin {
		actorType, actorName = "admin", "admin"
	} else {
		if cid, ok := s.GetSession("clientId").(int); ok {
			actorID = cid
		}
		if name, ok := s.GetSession("username").(string); ok && name != "" {
			actorName = name
		}
	}
	path := ""
	method := ""
	ip := ""
	if s.Ctx != nil && s.Ctx.Request != nil {
		method = s.Ctx.Request.Method
		if s.Ctx.Request.URL != nil {
			path = s.Ctx.Request.URL.Path
		}
		ip = s.Ctx.Input.IP()
	}
	_ = audit.Append(audit.Event{
		ActorType: actorType, ActorID: actorID, ActorName: actorName,
		SourceIP: ip, Method: method, Path: path,
		Action: action, ResourceType: resourceType, ResourceID: resourceID,
		OwnerUserID: ownerUserID, Result: result, Error: errMsg, Details: details,
	})
}

// 组装ajax
func ajax(str string, status int) map[string]interface{} {
	json := make(map[string]interface{})
	json["status"] = status
	json["msg"] = str
	return json
}

// 组装ajax
func ajaxWithId(str string, status int, id int) map[string]interface{} {
	json := make(map[string]interface{})
	json["status"] = status
	json["msg"] = str
	json["id"] = id
	return json
}

// ajax table返回
func (s *BaseController) AjaxTable(list interface{}, cnt int, recordsTotal int, kwargs map[string]interface{}) {
	json := make(map[string]interface{})
	json["rows"] = list
	json["total"] = recordsTotal
	if kwargs != nil {
		for k, v := range kwargs {
			if v != nil {
				json[k] = v
			}
		}
	}
	s.Data["json"] = json
	s.ServeJSON()
	s.StopRun()
}

// ajax table参数
func (s *BaseController) GetAjaxParams() (start, limit int) {
	return s.GetIntNoErr("offset"), s.GetIntNoErr("limit")
}

func (s *BaseController) SetInfo(name string) {
	s.Data["name"] = name
}

func (s *BaseController) SetType(name string) {
	s.Data["type"] = name
}

// indexHostActions 域名相关动作（须查 Hosts 表；不可用 Contains("h")，否则会误伤 changestatus 等）。
var indexHostActions = map[string]struct{}{
	"hostlist": {}, "gethost": {}, "delhost": {}, "hoststop": {}, "hoststart": {},
	"addhost": {}, "edithost": {},
}

// CheckUserAuth 非管理员归属校验。
// Host 与 Task 的 id 空间独立且会重号，不能「任一命中即放行」。
func (s *BaseController) CheckUserAuth() {
	myClientId := -1
	if cid, ok := s.GetSession("clientId").(int); ok {
		myClientId = cid
	}
	switch s.controllerName {
	case "client":
		if s.actionName == "add" {
			s.denyForbidden()
			return
		}
		if id := s.GetIntNoErr("id"); id != 0 && id != myClientId {
			s.denyForbidden()
			return
		}
	case "index":
		// 新增/改挂 client_id 时必须是自己（Prepare 已 SetParam，这里再挡表单直传）。
		if cid := s.rawQueryInt("client_id"); cid != 0 && cid != myClientId {
			s.denyForbidden()
			return
		}
		id := s.GetIntNoErr("id")
		if id == 0 {
			return
		}
		if _, isHost := indexHostActions[s.actionName]; isHost {
			if !s.hostBelongsToMe(id, myClientId) {
				s.denyForbidden()
			}
			return
		}
		if !s.tunnelBelongsToMe(id, myClientId) {
			s.denyForbidden()
		}
		case "global", "user", "auditpanel", "serverconfig", "waf":
			s.denyForbidden()
	}
}

// rawQueryInt 只读请求 Query/Form，不受 SetParam 覆盖；用于检测伪造 client_id。
func (s *BaseController) rawQueryInt(key string) int {
	if s.Ctx == nil || s.Ctx.Input == nil {
		return 0
	}
	val, _ := strconv.Atoi(s.Ctx.Input.Query(key))
	return val
}

func (s *BaseController) denyForbidden() {
	s.CustomAbort(403, "forbidden")
}

func (s *BaseController) tunnelBelongsToMe(id, myClientId int) bool {
	v, ok := file.GetDb().JsonDb.Tasks.Load(id)
	if !ok {
		return false
	}
	t, ok := v.(*file.Tunnel)
	return ok && t.Client != nil && t.Client.Id == myClientId
}

func (s *BaseController) hostBelongsToMe(id, myClientId int) bool {
	v, ok := file.GetDb().JsonDb.Hosts.Load(id)
	if !ok {
		return false
	}
	h, ok := v.(*file.Host)
	return ok && h.Client != nil && h.Client.Id == myClientId
}
