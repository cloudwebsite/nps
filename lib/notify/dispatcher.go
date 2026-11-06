package notify

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
)

// Config 通知分发配置。
type Config struct {
	Enable               bool
	TitlePrefix          string
	MinInterval          time.Duration
	ErrorPageMinInterval time.Duration
	QueueSize            int
	HTTPTimeout          time.Duration
	NotifyPublicClient   bool
	WecomWebhook         string
	TelegramBotToken     string
	TelegramChatID       string
	WebhookURL           string
	WebhookHeader        string
}

type Dispatcher struct {
	cfg       Config
	channels  []Channel
	limiter   *RateLimiter
	queue     chan Event
	dropCount atomic.Int64
	wg        sync.WaitGroup
	onceClose sync.Once
	closed    atomic.Bool
}

var (
	defaultMu   sync.RWMutex
	defaultDisp *Dispatcher
)

// InitFromConfig 从 nps.conf 初始化全局 Dispatcher；失败只 Warn。
func InitFromConfig() {
	if err := ReplaceDefault(loadConfigFromBeego()); err != nil {
		logs.Warn("notify init: %v", err)
	}
}

// ReplaceDefault 热替换全局 Dispatcher（保存 global 后调用）。
func ReplaceDefault(cfg Config) error {
	d, err := NewDispatcher(cfg)
	if err != nil {
		return err
	}
	defaultMu.Lock()
	if defaultDisp != nil {
		defaultDisp.Close()
	}
	defaultDisp = d
	defaultMu.Unlock()
	if d == nil {
		return nil
	}
	names := make([]string, 0, len(d.channels))
	for _, ch := range d.channels {
		names = append(names, ch.Name())
	}
	logs.Info("notify=on：通道 %v，去抖 %v（错误页 %v）", names, d.cfg.MinInterval, d.cfg.ErrorPageMinInterval)
	return nil
}

func loadConfigFromBeego() Config {
	timeoutSec := beego.AppConfig.DefaultInt("notify_http_timeout", 5)
	if timeoutSec <= 0 {
		timeoutSec = 5
	}
	minSec := beego.AppConfig.DefaultInt("notify_min_interval", 30)
	if minSec < 0 {
		minSec = 0
	}
	errPageSec := beego.AppConfig.DefaultInt("notify_error_page_min_interval", 300)
	if errPageSec < 0 {
		errPageSec = 0
	}
	queue := beego.AppConfig.DefaultInt("notify_queue_size", 64)
	if queue <= 0 {
		queue = 64
	}
	prefix := strings.TrimSpace(beego.AppConfig.String("notify_title_prefix"))
	if prefix == "" {
		prefix = "[nps]"
	}
	return Config{
		Enable:               beego.AppConfig.DefaultBool("notify_enable", false),
		TitlePrefix:          prefix,
		MinInterval:          time.Duration(minSec) * time.Second,
		ErrorPageMinInterval: time.Duration(errPageSec) * time.Second,
		QueueSize:            queue,
		HTTPTimeout:          time.Duration(timeoutSec) * time.Second,
		NotifyPublicClient:   beego.AppConfig.DefaultBool("notify_public_client", false),
		WecomWebhook:         beego.AppConfig.String("notify_wecom_webhook"),
		TelegramBotToken:     beego.AppConfig.String("notify_telegram_bot_token"),
		TelegramChatID:       beego.AppConfig.String("notify_telegram_chat_id"),
		WebhookURL:           beego.AppConfig.String("notify_webhook_url"),
		WebhookHeader:        beego.AppConfig.String("notify_webhook_header"),
	}
}

// NewDispatcher 构建分发器；未启用或无通道时返回 (nil, nil)。
func NewDispatcher(cfg Config) (*Dispatcher, error) {
	if !cfg.Enable {
		return nil, nil
	}
	if cfg.HTTPTimeout <= 0 {
		cfg.HTTPTimeout = 5 * time.Second
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = 64
	}
	if cfg.TitlePrefix == "" {
		cfg.TitlePrefix = "[nps]"
	}
	client := &http.Client{Timeout: cfg.HTTPTimeout}
	var channels []Channel
	if strings.TrimSpace(cfg.WecomWebhook) != "" {
		channels = append(channels, newWecomChannel(cfg.WecomWebhook, client))
	}
	if strings.TrimSpace(cfg.TelegramBotToken) != "" && strings.TrimSpace(cfg.TelegramChatID) != "" {
		channels = append(channels, newTelegramChannel(cfg.TelegramBotToken, cfg.TelegramChatID, client))
	}
	if strings.TrimSpace(cfg.WebhookURL) != "" {
		channels = append(channels, newWebhookChannel(cfg.WebhookURL, cfg.WebhookHeader, client))
	}
	if len(channels) == 0 {
		logs.Warn("notify_enable=true 但未配置任何通道，通知保持关闭")
		return nil, nil
	}
	d := &Dispatcher{
		cfg:      cfg,
		channels: channels,
		limiter:  newRateLimiter(),
		queue:    make(chan Event, cfg.QueueSize),
	}
	d.wg.Add(1)
	go d.loop()
	return d, nil
}

func (d *Dispatcher) loop() {
	defer d.wg.Done()
	for e := range d.queue {
		d.dispatch(e)
	}
}

func (d *Dispatcher) dispatch(e Event) {
	ctx, cancel := context.WithTimeout(context.Background(), d.cfg.HTTPTimeout)
	defer cancel()
	for _, ch := range d.channels {
		if err := ch.Send(ctx, e); err != nil {
			logs.Warn("notify %s 发送失败 type=%s: %v", ch.Name(), e.Type, err)
		}
	}
}

// Close 停止队列与 worker。
func (d *Dispatcher) Close() {
	if d == nil {
		return
	}
	d.onceClose.Do(func() {
		d.closed.Store(true)
		close(d.queue)
		d.wg.Wait()
	})
}

// Emit 异步投递；未启用或队列满时直接返回（满则计数 Warn）。
func Emit(e Event) {
	defaultMu.RLock()
	d := defaultDisp
	defaultMu.RUnlock()
	if d == nil {
		return
	}
	d.Emit(e)
}

// Emit 投递到本分发器。
func (d *Dispatcher) Emit(e Event) {
	if d == nil || d.closed.Load() {
		return
	}
	e.Type = strings.TrimSpace(e.Type)
	if e.Type == "" {
		return
	}
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	if strings.TrimSpace(e.Title) == "" {
		e.Title = d.defaultTitle(e.Type)
	} else if d.cfg.TitlePrefix != "" && !strings.HasPrefix(e.Title, d.cfg.TitlePrefix) {
		e.Title = strings.TrimSpace(d.cfg.TitlePrefix + " " + e.Title)
	}
	min := d.cfg.MinInterval
	if e.Type == TypeErrorPage {
		if d.cfg.ErrorPageMinInterval > 0 {
			min = d.cfg.ErrorPageMinInterval
		}
	}
	key := rateKey(e)
	if !d.limiter.Allow(key, min) {
		return
	}
	select {
	case d.queue <- e:
	default:
		n := d.dropCount.Add(1)
		logs.Warn("notify 队列已满，丢弃事件 type=%s count=%d", e.Type, n)
	}
}

func rateKey(e Event) string {
	switch e.Type {
	case TypeClientConnect, TypeClientDisconnect:
		return fmt.Sprintf("%s:%d", e.Type, e.ClientID)
	case TypeHealthDown, TypeHealthUp:
		return fmt.Sprintf("%s:%d:%s", e.Type, e.ClientID, e.Target)
	case TypeErrorPage:
		return fmt.Sprintf("%s:%s", e.Type, e.Host)
	default:
		return e.Type
	}
}

func (d *Dispatcher) defaultTitle(typ string) string {
	prefix := d.cfg.TitlePrefix
	switch typ {
	case TypeClientConnect:
		return prefix + " 客户端上线"
	case TypeClientDisconnect:
		return prefix + " 客户端下线"
	case TypeHealthDown:
		return prefix + " 健康检查失败"
	case TypeHealthUp:
		return prefix + " 健康检查恢复"
	case TypeErrorPage:
		return prefix + " 访问错误页"
	default:
		return prefix + " " + typ
	}
}

// NotifyPublicClient 是否通知公开客户端上下线。
func NotifyPublicClient() bool {
	defaultMu.RLock()
	d := defaultDisp
	defaultMu.RUnlock()
	if d == nil {
		return false
	}
	return d.cfg.NotifyPublicClient
}

// Enabled 全局通知是否可用。
func Enabled() bool {
	defaultMu.RLock()
	d := defaultDisp
	defaultMu.RUnlock()
	return d != nil
}

// Dropped 测试用：丢弃计数。
func (d *Dispatcher) Dropped() int64 {
	if d == nil {
		return 0
	}
	return d.dropCount.Load()
}

// SetDefaultForTest 测试注入。
func SetDefaultForTest(d *Dispatcher) {
	defaultMu.Lock()
	if defaultDisp != nil && defaultDisp != d {
		defaultDisp.Close()
	}
	defaultDisp = d
	defaultMu.Unlock()
}
