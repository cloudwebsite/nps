package controllers

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/big"
	"net"
	"net/mail"
	"strings"
	"sync"
	"time"

	"ehang.io/nps/lib/file"
	npsmail "ehang.io/nps/lib/mail"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
)

const (
	defaultEmailCodeTTLSeconds      = 600
	defaultEmailCodeCooldownSeconds = 60
	maxRegistrationEmailCodes       = 1024
	maxRegistrationEmailAttempts    = 5
)

type registrationEmailCodeRecord struct {
	digest    [sha256.Size]byte
	expiresAt time.Time
	attempts  int
	ip        string
	issuedAt  time.Time
}

type registrationEmailCodeStore struct {
	mu    sync.Mutex
	codes map[string]*registrationEmailCodeRecord
}

func newRegistrationEmailCodeStore() *registrationEmailCodeStore {
	return &registrationEmailCodeStore{codes: make(map[string]*registrationEmailCodeRecord)}
}

func (s *registrationEmailCodeStore) issue(emailAddress, ip string, now time.Time, ttl, cooldown time.Duration) (string, time.Duration, error) {
	emailAddress = strings.ToLower(strings.TrimSpace(emailAddress))
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(now)
	if rec := s.codes[emailAddress]; rec != nil && cooldown > 0 && now.Sub(rec.issuedAt) < cooldown {
		return "", cooldown - now.Sub(rec.issuedAt), fmt.Errorf("发送过于频繁，请稍后再试")
	}
	if len(s.codes) >= maxRegistrationEmailCodes {
		s.cleanupLocked(now)
		if len(s.codes) >= maxRegistrationEmailCodes {
			return "", 0, fmt.Errorf("验证码配额已满，请稍后再试")
		}
	}
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "", 0, err
	}
	code := fmt.Sprintf("%06d", n.Int64())
	s.codes[emailAddress] = &registrationEmailCodeRecord{
		digest:    sha256.Sum256([]byte(code)),
		expiresAt: now.Add(ttl),
		ip:        ip,
		issuedAt:  now,
	}
	return code, 0, nil
}

func (s *registrationEmailCodeStore) discard(emailAddress string) {
	emailAddress = strings.ToLower(strings.TrimSpace(emailAddress))
	s.mu.Lock()
	delete(s.codes, emailAddress)
	s.mu.Unlock()
}

func (s *registrationEmailCodeStore) verify(emailAddress, code string, now time.Time) bool {
	emailAddress = strings.ToLower(strings.TrimSpace(emailAddress))
	code = strings.TrimSpace(code)
	if emailAddress == "" || code == "" {
		return false
	}
	digest := sha256.Sum256([]byte(code))
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := s.codes[emailAddress]
	if rec == nil || !now.Before(rec.expiresAt) {
		delete(s.codes, emailAddress)
		return false
	}
	rec.attempts++
	if rec.attempts > maxRegistrationEmailAttempts {
		delete(s.codes, emailAddress)
		return false
	}
	if subtle.ConstantTimeCompare(rec.digest[:], digest[:]) != 1 {
		return false
	}
	delete(s.codes, emailAddress)
	return true
}

func (s *registrationEmailCodeStore) cleanupLocked(now time.Time) {
	for k, rec := range s.codes {
		if rec == nil || !now.Before(rec.expiresAt) {
			delete(s.codes, k)
		}
	}
}

var registrationEmailCodes = newRegistrationEmailCodeStore()

func registrationEmailVerifyEnabled() bool {
	return file.EffectiveRegisterEmailVerify()
}

func registrationEmailCodeTTL() time.Duration {
	seconds := beego.AppConfig.DefaultInt("register_email_code_ttl", defaultEmailCodeTTLSeconds)
	if seconds < 60 {
		seconds = defaultEmailCodeTTLSeconds
	}
	if seconds > 86400 {
		seconds = 86400
	}
	return time.Duration(seconds) * time.Second
}

func registrationEmailCodeCooldown() time.Duration {
	seconds := beego.AppConfig.DefaultInt("register_email_code_cooldown", defaultEmailCodeCooldownSeconds)
	if seconds < 0 {
		seconds = 0
	}
	if seconds > 86400 {
		seconds = 86400
	}
	return time.Duration(seconds) * time.Second
}

func normalizeRegisterEmail(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	addr, err := mail.ParseAddress(raw)
	if err != nil || addr == nil || addr.Address == "" {
		return "", fmt.Errorf("邮箱格式无效")
	}
	return strings.ToLower(addr.Address), nil
}

// RegisterEmailCode 发送注册邮箱验证码（默认关 register_email_verify）。
func (self *LoginController) RegisterEmailCode() {
	if !fileAllowUserRegister() {
		self.Data["json"] = map[string]interface{}{"status": 0, "msg": "register is not allow"}
		self.ServeJSON()
		return
	}
	if !registrationEmailVerifyEnabled() {
		self.Data["json"] = map[string]interface{}{"status": 0, "msg": "email verify is disabled"}
		self.ServeJSON()
		return
	}
	ip, _, _ := net.SplitHostPort(self.Ctx.Request.RemoteAddr)
	if ip == "" {
		ip = self.Ctx.Input.IP()
	}
	if !allowRegisterAttempt(ip) {
		self.Data["json"] = map[string]interface{}{"status": 0, "msg": "register too frequent, try later"}
		self.ServeJSON()
		return
	}
	emailAddress, err := normalizeRegisterEmail(self.GetString("email"))
	if err != nil || emailAddress == "" {
		self.Data["json"] = map[string]interface{}{"status": 0, "msg": "请填写有效邮箱"}
		self.ServeJSON()
		return
	}
	now := time.Now()
	code, retryAfter, err := registrationEmailCodes.issue(emailAddress, ip, now, registrationEmailCodeTTL(), registrationEmailCodeCooldown())
	if err != nil {
		self.Data["json"] = map[string]interface{}{
			"status":      0,
			"msg":         err.Error(),
			"retry_after": int(retryAfter / time.Second),
		}
		self.ServeJSON()
		return
	}
		smtpCfg, err := npsmail.LoadFromAppConfig()
		if err != nil {
			registrationEmailCodes.discard(emailAddress)
			self.Data["json"] = map[string]interface{}{"status": 0, "msg": "邮件服务未配置: " + err.Error()}
			self.ServeJSON()
			return
		}
		body := fmt.Sprintf("您的 NPS 注册验证码是：%s\r\n\r\n验证码 %d 分钟内有效。若非本人操作，请忽略本邮件。\r\n",
			code, int(registrationEmailCodeTTL()/time.Minute))
		if err := npsmail.SendPlain(smtpCfg, emailAddress, "NPS 注册验证码", body); err != nil {
		registrationEmailCodes.discard(emailAddress)
		logs.Warn("注册验证码发送失败 email=%s err=%v", emailAddress, err)
		self.Data["json"] = map[string]interface{}{"status": 0, "msg": "发送验证码失败"}
		self.ServeJSON()
		return
	}
	self.Data["json"] = map[string]interface{}{
		"status":      1,
		"msg":         "verification code sent",
		"retry_after": int(registrationEmailCodeCooldown() / time.Second),
		"expires_in":  int(registrationEmailCodeTTL() / time.Second),
	}
	self.ServeJSON()
}

func fileAllowUserRegister() bool {
	return file.GetDb().GetGlobalSettings().AllowUserRegister
}

// 供测试观察摘要（不回传明文码）。
func registrationEmailCodeDigestForTest(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	registrationEmailCodes.mu.Lock()
	defer registrationEmailCodes.mu.Unlock()
	rec := registrationEmailCodes.codes[email]
	if rec == nil {
		return ""
	}
	return hex.EncodeToString(rec.digest[:8])
}
