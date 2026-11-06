package controllers

import (
	"math/rand"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/crypt"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/lib/version"
	"ehang.io/nps/server"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/cache"
	"github.com/astaxie/beego/logs"
	"github.com/astaxie/beego/utils/captcha"
)

type LoginController struct {
	beego.Controller
}

var ipRecord sync.Map
var cpt *captcha.Captcha

type record struct {
	hasLoginFailTimes int
	lastLoginTime     time.Time
}

// InitCaptcha registers the captcha image route and the create_captcha template
// function. The image URL prefix must include web_base_url, otherwise the login
// page would request the captcha from the site root and fail behind a reverse
// proxy that only forwards the web_base_url sub path.
//
// It must be called after beego.LoadAppConfig, so it cannot live in init():
// package init() runs before main() loads conf/nps.conf, where web_base_url
// would still be empty.
func InitCaptcha() {
	store := cache.NewMemoryCache()
	base := strings.TrimSuffix(beego.AppConfig.String("web_base_url"), "/")
	if base != "" && !strings.HasPrefix(base, "/") {
		base = "/" + base
	}
	cpt = captcha.NewWithFilter(base+"/captcha/", store)
}

func adminTOTPSecret() string {
	return strings.TrimSpace(beego.AppConfig.String("totp_secret"))
}

func adminTOTPEnabled() bool {
	return crypt.IsValidTOTPSecret(adminTOTPSecret())
}

func (self *LoginController) Index() {
	webBaseUrl := beego.AppConfig.String("web_base_url")
	// 启用管理员 TOTP 时禁止空凭据隐式登录（否则会绕过二次验证）
	if !adminTOTPEnabled() && self.doLogin("", "", "", false) {
		self.Redirect(webBaseUrl+"/index/index", 302)
	}
	self.Data["web_base_url"] = webBaseUrl
	globalSettings := file.GetDb().GetGlobalSettings()
	self.Data["register_allow"] = globalSettings.AllowUserRegister
	self.Data["captcha_open"] = globalSettings.OpenCaptcha
	self.Data["totp_enabled"] = adminTOTPEnabled()
	self.Data["version"] = version.VERSION
	self.Data["web_welcome"] = file.EffectiveWebWelcome()
	self.Data["web_subtitle"] = file.EffectiveWebSubtitle()
	self.Data["app_name"] = file.EffectiveAppName()
	self.TplName = "login/index.html"
}

func (self *LoginController) Verify() {
	username := self.GetString("username")
	password := self.GetString("password")
	totpCode := strings.TrimSpace(self.GetString("totp"))
	captchaOpen := file.GetDb().GetGlobalSettings().OpenCaptcha
	if captchaOpen {
		if !cpt.VerifyReq(self.Ctx.Request) {
			self.Data["json"] = map[string]interface{}{"status": 0, "msg": "the verification code is wrong, please get it again and try again"}
			self.ServeJSON()
			return
		}
	}
	if self.doLogin(username, password, totpCode, true) {
		self.Data["json"] = map[string]interface{}{"status": 1, "msg": "login success"}
	} else {
		msg := "username or password incorrect"
		if adminTOTPEnabled() {
			msg = "username, password or authenticator code incorrect"
		}
		self.Data["json"] = map[string]interface{}{"status": 0, "msg": msg}
	}
	self.ServeJSON()
}

func (self *LoginController) doLogin(username, password, totpCode string, explicit bool) bool {
	clearIprecord()
	ip, _, _ := net.SplitHostPort(self.Ctx.Request.RemoteAddr)
	if v, ok := ipRecord.Load(ip); ok {
		vv := v.(*record)
		if (time.Now().Unix() - vv.lastLoginTime.Unix()) >= 60 {
			vv.hasLoginFailTimes = 0
		}
		if vv.hasLoginFailTimes >= 10 {
			return false
		}
	}

	cfgUser := beego.AppConfig.String("web_username")
	cfgPass := beego.AppConfig.String("web_password")
	noAuth := cfgUser == "" && cfgPass == ""
	// 非无认证模式：空凭据直接失败，避免隐式登录对每个客户端跑 bcrypt
	if !noAuth && (username == "" || password == "") {
		return false
	}

	checkAdminTOTP := func() bool {
		secret := adminTOTPSecret()
		if secret == "" {
			return true
		}
		ok, err := crypt.ValidateTOTPCode(secret, totpCode)
		return err == nil && ok
	}
	recordLoginFail := func() {
		if v, load := ipRecord.LoadOrStore(ip, &record{hasLoginFailTimes: 1, lastLoginTime: time.Now()}); load && explicit {
			vv := v.(*record)
			vv.lastLoginTime = time.Now()
			vv.hasLoginFailTimes += 1
			ipRecord.Store(ip, vv)
		}
	}

	var auth bool
	if noAuth {
		if !checkAdminTOTP() {
			recordLoginFail()
			return false
		}
		self.SetSession("isAdmin", true)
		self.DelSession("clientId")
		self.DelSession("username")
		auth = true
		server.Bridge.Register.Store(common.GetIpByAddr(self.Ctx.Input.IP()), time.Now().Add(time.Hour*time.Duration(2)))
	} else if username == cfgUser && common.VerifyPassword(cfgPass, password) {
		// 管理员口令正确后强制校验 TOTP；失败不得落入普通用户/客户端登录分支
		if !checkAdminTOTP() {
			recordLoginFail()
			return false
		}
		self.SetSession("isAdmin", true)
		self.DelSession("clientId")
		self.DelSession("username")
		auth = true
		server.Bridge.Register.Store(common.GetIpByAddr(self.Ctx.Input.IP()), time.Now().Add(time.Hour*time.Duration(2)))
		// 存量明文管理员密码，首次成功登录后迁为 bcrypt（不静默改用户密码内容）
		migrateAdminPasswordToBcrypt(password)
	}

	allowUserLogin := file.GetDb().GetGlobalSettings().AllowUserLogin
	if allowUserLogin && !auth {
		file.GetDb().JsonDb.Clients.Range(func(key, value interface{}) bool {
			v := value.(*file.Client)
			if !v.Status || v.NoDisplay {
				return true
			}
			if v.WebUserName == "" && v.WebPassword == "" {
				if username != "user" || v.VerifyKey != password {
					return true
				}
				auth = true
			}
			// 先比用户名再验密码，避免对全表做 bcrypt
			if !auth && v.WebUserName == username && common.VerifyPassword(v.WebPassword, password) {
				auth = true
				if !common.IsBcryptHash(v.WebPassword) {
					if h, err := common.HashPassword(password); err == nil {
						v.WebPassword = h
						file.GetDb().JsonDb.Clients.Store(v.Id, v)
						file.GetDb().JsonDb.StoreClientsToJsonFile()
					}
				}
			}
			if auth {
				self.SetSession("isAdmin", false)
				self.SetSession("clientId", v.Id)
				self.SetSession("username", v.WebUserName)
				return false
			}
			return true
		})
	}
	if auth {
		// 登录成功后重置会话，降低会话固定风险
		self.SessionRegenerateID()
		self.SetSession("auth", true)
		ipRecord.Delete(ip)
		return true
	}
	if v, load := ipRecord.LoadOrStore(ip, &record{hasLoginFailTimes: 1, lastLoginTime: time.Now()}); load && explicit {
		vv := v.(*record)
		vv.lastLoginTime = time.Now()
		vv.hasLoginFailTimes += 1
		ipRecord.Store(ip, vv)
	}
	return false
}

func migrateAdminPasswordToBcrypt(plain string) {
	cfg := beego.AppConfig.String("web_password")
	if common.IsBcryptHash(cfg) {
		return
	}
	h, err := common.HashPassword(plain)
	if err != nil {
		logs.Warn("migrate admin password: hash error %v", err)
		return
	}
	confFile := filepath.Join(common.GetRunPath(), "conf", "nps.conf")
	if err := common.ReplaceIniKeyValue(confFile, "web_password", h); err != nil {
		logs.Warn("migrate admin password: write conf error %v", err)
		return
	}
	_ = beego.AppConfig.Set("web_password", h)
	logs.Info("管理员面板密码已迁移为 bcrypt（nps.conf）")
}

func (self *LoginController) Register() {
	if self.Ctx.Request.Method == "GET" {
		self.Data["web_base_url"] = beego.AppConfig.String("web_base_url")
		self.Data["version"] = version.VERSION
		globalSettings := file.GetDb().GetGlobalSettings()
		self.Data["captcha_open"] = globalSettings.OpenCaptcha
		self.Data["register_allow"] = globalSettings.AllowUserRegister
		self.Data["email_verify"] = registrationEmailVerifyEnabled()
		self.Data["email_code_cooldown"] = int(registrationEmailCodeCooldown() / time.Second)
		self.Data["web_welcome"] = file.EffectiveWebWelcome()
		self.Data["web_subtitle"] = file.EffectiveWebSubtitle()
		self.Data["app_name"] = file.EffectiveAppName()
		self.TplName = "login/register.html"
	} else {
		if !file.GetDb().GetGlobalSettings().AllowUserRegister {
			self.Data["json"] = map[string]interface{}{"status": 0, "msg": "register is not allow"}
			self.ServeJSON()
			return
		}
		// 注册限频（与登录失败计数同表；默认每 IP 每分钟最多 5 次尝试）
		ip, _, _ := net.SplitHostPort(self.Ctx.Request.RemoteAddr)
		if ip == "" {
			ip = self.Ctx.Input.IP()
		}
		if !allowRegisterAttempt(ip) {
			self.Data["json"] = map[string]interface{}{"status": 0, "msg": "register too frequent, try later"}
			self.ServeJSON()
			return
		}
		if file.GetDb().GetGlobalSettings().OpenCaptcha {
			if cpt == nil || !cpt.VerifyReq(self.Ctx.Request) {
				self.Data["json"] = map[string]interface{}{"status": 0, "msg": "the verification code is wrong, please get it again and try again"}
				self.ServeJSON()
				return
			}
		}
		if self.GetString("username") == "" || self.GetString("password") == "" || self.GetString("username") == beego.AppConfig.String("web_username") {
			self.Data["json"] = map[string]interface{}{"status": 0, "msg": "please check your input"}
			self.ServeJSON()
			return
		}
		emailAddress := ""
		if registrationEmailVerifyEnabled() {
			var err error
			emailAddress, err = normalizeRegisterEmail(self.GetString("email"))
			if err != nil || emailAddress == "" {
				self.Data["json"] = map[string]interface{}{"status": 0, "msg": "请填写有效邮箱"}
				self.ServeJSON()
				return
			}
			if !registrationEmailCodes.verify(emailAddress, self.GetString("email_code"), time.Now()) {
				self.Data["json"] = map[string]interface{}{"status": 0, "msg": "邮箱验证码错误或已过期"}
				self.ServeJSON()
				return
			}
		}
		webPass := self.GetString("password")
		if h, err := common.HashPassword(webPass); err == nil {
			webPass = h
		}
		t := &file.Client{
			Status:      true,
			Cnf:         &file.Config{},
			WebUserName: self.GetString("username"),
			WebPassword: webPass,
			Remark:      emailAddress,
			Flow:        &file.Flow{},
		}
		if err := file.GetDb().NewClient(t); err != nil {
			self.Data["json"] = map[string]interface{}{"status": 0, "msg": err.Error()}
		} else {
			self.Data["json"] = map[string]interface{}{"status": 1, "msg": "register success"}
		}
		self.ServeJSON()
	}
}

// allowRegisterAttempt 注册限频；max 次 / 60s，默认 5。
func allowRegisterAttempt(ip string) bool {
	max := file.EffectiveRegisterRateLimit()
	if max <= 0 {
		return true
	}
	now := time.Now()
	if v, ok := registerIPRecord.Load(ip); ok {
		rec := v.(*record)
		if now.Unix()-rec.lastLoginTime.Unix() >= 60 {
			rec.hasLoginFailTimes = 0
		}
		if rec.hasLoginFailTimes >= max {
			return false
		}
		rec.hasLoginFailTimes++
		rec.lastLoginTime = now
		return true
	}
	registerIPRecord.Store(ip, &record{hasLoginFailTimes: 1, lastLoginTime: now})
	return true
}

var registerIPRecord sync.Map

func (self *LoginController) Out() {
	self.SetSession("auth", false)
	self.Redirect(beego.AppConfig.String("web_base_url")+"/login/index", 302)
}

func clearIprecord() {
	rand.Seed(time.Now().UnixNano())
	x := rand.Intn(100)
	if x == 1 {
		ipRecord.Range(func(key, value interface{}) bool {
			v := value.(*record)
			if time.Now().Unix()-v.lastLoginTime.Unix() >= 60 {
				ipRecord.Delete(key)
			}
			return true
		})
	}
}
