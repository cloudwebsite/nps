package controllers

import (
	"encoding/hex"
	"html"
	"net"
	"sync"
	"time"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/crypt"
	"ehang.io/nps/lib/file"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
)

type AuthController struct {
	beego.Controller
}

func (s *AuthController) GetAuthKey() {
	m := make(map[string]interface{})
	defer func() {
		s.Data["json"] = m
		s.ServeJSON()
	}()
	if cryptKey := beego.AppConfig.String("auth_crypt_key"); len(cryptKey) != 16 {
		m["status"] = 0
		return
	} else {
		b, err := crypt.AesEncrypt([]byte(beego.AppConfig.String("auth_key")), []byte(cryptKey))
		if err != nil {
			m["status"] = 0
			return
		}
		m["status"] = 1
		m["crypt_auth_key"] = hex.EncodeToString(b)
		m["crypt_type"] = "aes cbc"
		return
	}
}

func (s *AuthController) GetTime() {
	m := make(map[string]interface{})
	m["time"] = time.Now().Unix()
	s.Data["json"] = m
	s.ServeJSON()
}

// ipWhiteLimiter：公开登记/白名单接口失败限速（每 IP 每分钟 10 次）。
var (
	ipWhiteMu    sync.Mutex
	ipWhiteFails = map[string]*ipWhiteRec{}
)

type ipWhiteRec struct {
	n    int
	last time.Time
}

func ipWhiteBlocked(ip string) bool {
	ipWhiteMu.Lock()
	defer ipWhiteMu.Unlock()
	r, ok := ipWhiteFails[ip]
	if !ok {
		return false
	}
	if time.Since(r.last) >= time.Minute {
		delete(ipWhiteFails, ip)
		return false
	}
	return r.n >= 10
}

func ipWhiteFail(ip string) {
	ipWhiteMu.Lock()
	defer ipWhiteMu.Unlock()
	r, ok := ipWhiteFails[ip]
	if !ok || time.Since(r.last) >= time.Minute {
		ipWhiteFails[ip] = &ipWhiteRec{n: 1, last: time.Now()}
		return
	}
	r.n++
	r.last = time.Now()
	if len(ipWhiteFails) > 4096 {
		for k, v := range ipWhiteFails {
			if time.Since(v.last) >= time.Minute {
				delete(ipWhiteFails, k)
			}
		}
	}
}

func maskKey(vkey string) string {
	if len(vkey) <= 6 {
		return "***"
	}
	return vkey[:3] + "***" + vkey[len(vkey)-2:]
}

func (s *AuthController) IpWhiteAuth() {
	// NatPunch：仅 POST；去掉 ACAO:*；统一错误文案防 oracle。
	if s.Ctx.Request.Method != "POST" {
		s.Ctx.Output.SetStatus(405)
		s.Data["json"] = map[string]interface{}{"success": false, "message": "method not allowed"}
		s.ServeJSON()
		return
	}
	s.Ctx.ResponseWriter.Header().Set("Cache-Control", "no-store")

	vkey := s.GetString("vkey")
	ip := s.GetString("ip")
	password := s.GetString("pass")
	reqIP := s.Ctx.Input.IP()

	if vkey == "" || password == "" {
		s.Data["json"] = map[string]interface{}{"success": false, "message": "参数错误"}
		s.ServeJSON()
		return
	}
	if ipWhiteBlocked(reqIP) {
		s.Ctx.Output.SetStatus(429)
		s.Data["json"] = map[string]interface{}{"success": false, "message": "尝试过于频繁，请稍后再试"}
		s.ServeJSON()
		return
	}
	if ip == "" {
		ip = reqIP
	} else if net.ParseIP(ip) == nil {
		s.Data["json"] = map[string]interface{}{"success": false, "message": "参数错误"}
		s.ServeJSON()
		return
	}

	c, err := file.GetDb().GetClientByVkey(vkey)
	if err != nil || !c.IpWhite || !c.Status {
		ipWhiteFail(reqIP)
		s.Data["json"] = map[string]interface{}{"success": false, "message": "认证失败"}
		s.ServeJSON()
		logs.Error("客户端IP白名单认证失败:vkey [%s] ip [%s]", maskKey(vkey), ip)
		return
	}
	if !common.ConstantTimeStrEq(c.IpWhitePass, password) {
		ipWhiteFail(reqIP)
		s.Data["json"] = map[string]interface{}{"success": false, "message": "认证失败"}
		s.ServeJSON()
		logs.Error("客户端IP白名单认证失败:vkey [%s] ip [%s]", maskKey(vkey), ip)
		return
	}
	if c.AddIpWhite(ip) {
		_ = file.GetDb().UpdateClient(c)
	}
	s.Data["json"] = map[string]interface{}{"success": true, "message": "授权成功"}
	s.ServeJSON()
	logs.Info("客户端IP白名单认证授权成功:vkey [%s] ip [%s]", maskKey(vkey), ip)
}

func (s *AuthController) getEscapeString(key string) string {
	return html.EscapeString(s.GetString(key))
}
