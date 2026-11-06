// Package mail 提供可选 SMTP 发信。未配置 smtp_host 时不发送。
package mail

import (
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"ehang.io/nps/lib/file"

	"github.com/astaxie/beego"
)

// Config SMTP 配置（兼容 smtp_user/smtp_pass 与 smtp_username/smtp_password）。
type Config struct {
	Host     string
	Port     int
	Security string // starttls | ssl | none
	Username string
	Password string
	From     string
}

// LoadFromAppConfig 读取 SMTP：优先 global.json（热更新），缺省回退 nps.conf。
func LoadFromAppConfig() (Config, error) {
	gs := file.GetDb().GetGlobalSettings()
	cfg := Config{
		Host:     strings.TrimSpace(gs.SmtpHost),
		Port:     gs.SmtpPort,
		Security: strings.ToLower(strings.TrimSpace(gs.SmtpSecurity)),
		Username: strings.TrimSpace(gs.SmtpUser),
		Password: gs.SmtpPass,
		From:     strings.TrimSpace(gs.SmtpFrom),
	}
	if cfg.Host == "" {
		// 迁移前或未写入 global 时兜底 conf
		user := strings.TrimSpace(beego.AppConfig.String("smtp_user"))
		if user == "" {
			user = strings.TrimSpace(beego.AppConfig.String("smtp_username"))
		}
		pass := beego.AppConfig.String("smtp_pass")
		if strings.TrimSpace(pass) == "" {
			pass = beego.AppConfig.String("smtp_password")
		}
		cfg = Config{
			Host:     strings.TrimSpace(beego.AppConfig.String("smtp_host")),
			Port:     beego.AppConfig.DefaultInt("smtp_port", 587),
			Security: strings.ToLower(strings.TrimSpace(beego.AppConfig.String("smtp_security"))),
			Username: user,
			Password: pass,
			From:     strings.TrimSpace(beego.AppConfig.String("smtp_from")),
		}
	}
	return validateSMTP(cfg)
}

func validateSMTP(cfg Config) (Config, error) {
	if cfg.Host == "" {
		return cfg, errors.New("SMTP 主机未配置")
	}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return cfg, errors.New("SMTP 端口无效")
	}
	if cfg.Security == "" {
		cfg.Security = "starttls"
	}
	switch cfg.Security {
	case "starttls", "ssl", "none":
	default:
		return cfg, errors.New("SMTP 加密方式无效（允许 starttls/ssl/none）")
	}
	from, err := mail.ParseAddress(cfg.From)
	if err != nil || from == nil || from.Address == "" {
		return cfg, errors.New("SMTP 发件人邮箱无效")
	}
	cfg.From = from.Address
	if cfg.Username != "" && cfg.Password == "" {
		return cfg, errors.New("SMTP 用户名已配置，但密码为空")
	}
	return cfg, nil
}

// SendPlain 发送纯文本邮件。
func SendPlain(cfg Config, to, subject, body string) error {
	recipient, err := mail.ParseAddress(strings.TrimSpace(to))
	if err != nil || recipient == nil || recipient.Address == "" {
		return errors.New("收件人邮箱无效")
	}
	client, err := openClient(cfg)
	if err != nil {
		return err
	}
	defer client.Close()

	if cfg.Username != "" {
		var auth smtp.Auth
		if cfg.Security == "none" {
			auth = &plainAuth{username: cfg.Username, password: cfg.Password}
		} else {
			auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
		}
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("SMTP 认证失败: %w", err)
		}
	}
	if err := client.Mail(cfg.From); err != nil {
		return fmt.Errorf("SMTP 发件人被拒绝: %w", err)
	}
	if err := client.Rcpt(recipient.Address); err != nil {
		return fmt.Errorf("SMTP 收件人被拒绝: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP 无法开始发送邮件: %w", err)
	}
	message := strings.Join([]string{
		"From: " + cfg.From,
		"To: " + recipient.Address,
		"Subject: " + mime.QEncoding.Encode("UTF-8", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: base64",
		"",
		base64.StdEncoding.EncodeToString([]byte(body)),
		"",
	}, "\r\n")
	if _, err := writer.Write([]byte(message)); err != nil {
		_ = writer.Close()
		return fmt.Errorf("写入邮件失败: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("发送邮件失败: %w", err)
	}
	_ = client.Quit()
	return nil
}

type plainAuth struct {
	username string
	password string
}

func (a *plainAuth) Start(*smtp.ServerInfo) (string, []byte, error) {
	return "PLAIN", []byte("\x00" + a.username + "\x00" + a.password), nil
}

func (a *plainAuth) Next(_ []byte, more bool) ([]byte, error) {
	if more {
		return nil, errors.New("SMTP 认证协议错误")
	}
	return nil, nil
}

func openClient(cfg Config) (*smtp.Client, error) {
	address := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	dialer := &net.Dialer{Timeout: 15 * time.Second}
	tlsConfig := &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}
	if cfg.Security == "ssl" {
		connection, err := tls.DialWithDialer(dialer, "tcp", address, tlsConfig)
		if err != nil {
			return nil, fmt.Errorf("连接 SMTP TLS 服务器失败: %w", err)
		}
		client, err := smtp.NewClient(connection, cfg.Host)
		if err != nil {
			_ = connection.Close()
			return nil, fmt.Errorf("初始化 SMTP TLS 客户端失败: %w", err)
		}
		return client, nil
	}
	connection, err := dialer.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("连接 SMTP 服务器失败: %w", err)
	}
	client, err := smtp.NewClient(connection, cfg.Host)
	if err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("初始化 SMTP 客户端失败: %w", err)
	}
	if cfg.Security == "starttls" {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			_ = client.Close()
			return nil, errors.New("SMTP 服务器不支持 STARTTLS；465 端口请设 smtp_security=ssl")
		}
		if err := client.StartTLS(tlsConfig); err != nil {
			_ = client.Close()
			return nil, fmt.Errorf("SMTP STARTTLS 失败: %w", err)
		}
	}
	return client, nil
}
