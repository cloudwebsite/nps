package file

import (
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ehang.io/nps/lib/rate"
	"github.com/pkg/errors"
)

type Flow struct {
	ExportFlow int64
	InletFlow  int64
	FlowLimit  int64
	sync.RWMutex
}

func (s *Flow) Add(in, out int64) {
	s.Lock()
	defer s.Unlock()
	s.InletFlow += int64(in)
	s.ExportFlow += int64(out)
}

type Config struct {
	U        string
	P        string
	Compress bool
	Crypt    bool
}

type Client struct {
	Cnf             *Config
	Id              int        //id
	VerifyKey       string     //verify key
	Addr            string     // client egress address as seen by server (RemoteAddr)
	LocalAddr       string     // client private/LAN addresses reported by npc
	Remark          string     //remark
	Status          bool       //is allow connect
	IsConnect       bool       //is the client connect
	RateLimit       int        //rate /kb
	Flow            *Flow      //flow setting
	Rate            *rate.Rate //rate limit
	NoStore         bool       //no store to file
	NoDisplay       bool       //no display on web
	MaxConn         int        //the max connection num of client allow
	NowConn         int32      //the connection num of now
	WebUserName     string     //the username of web login
	WebPassword     string     //the password of web login
	ConfigConnAllow bool       //is allow connected by config file
	MaxTunnelNum    int
	Version         string
	// LastUpdate* 远程升级最近一次回传（运行时字段，供面板展示；可落盘）。
	LastUpdateStatus  string `json:"LastUpdateStatus,omitempty"`  // ok|skip|fail|timeout
	LastUpdateChannel string `json:"LastUpdateChannel,omitempty"` // cli|gui
	LastUpdateMsg     string `json:"LastUpdateMsg,omitempty"`
	LastUpdateAt      string `json:"LastUpdateAt,omitempty"`
	BlackIpList []string
	CreateTime  string
	LastOnlineTime string
	IpWhite         bool     // 是否启用ip白名单认证页
	IpWhitePass     string   // ip授权密码
	IpWhiteList     []string // 白名单（ACL；IpWhite=true 时兼作认证放行名单）
	// IpListPriority：0=DenyFirst（默认），1=AllowFirst；旧 JSON 缺字段=0。
	IpListPriority int `json:"IpListPriority,omitempty"`
	ExpireTime     string // 到期时间,留空表示永不过期,格式 2006-01-02 15:04:05
	// UserId 预留给多用户归属。旧 JSON 无此字段时为零值，读入兼容。
	UserId int `json:"UserId,omitempty"`
	sync.RWMutex
}

// AddIpWhite 追加白名单 IP；已存在返回 false。持写锁。
func (c *Client) AddIpWhite(ip string) bool {
	if c == nil || ip == "" {
		return false
	}
	c.Lock()
	defer c.Unlock()
	for _, v := range c.IpWhiteList {
		if v == ip {
			return false
		}
	}
	c.IpWhiteList = append(c.IpWhiteList, ip)
	return true
}

func NewClient(vKey string, noStore bool, noDisplay bool) *Client {
	return &Client{
		Cnf:       new(Config),
		Id:        0,
		VerifyKey: vKey,
		Addr:      "",
		LocalAddr: "",
		Remark:    "",
		Status:    true,
		IsConnect: false,
		RateLimit: 0,
		Flow:      new(Flow),
		Rate:      nil,
		NoStore:   noStore,
		RWMutex:   sync.RWMutex{},
		NoDisplay: noDisplay,
	}
}

func (s *Client) CutConn() {
	atomic.AddInt32(&s.NowConn, 1)
}

func (s *Client) AddConn() {
	atomic.AddInt32(&s.NowConn, -1)
}

func (s *Client) GetConn() bool {
	if s.MaxConn == 0 || int(s.NowConn) < s.MaxConn {
		s.CutConn()
		return true
	}
	return false
}

func (s *Client) HasTunnel(t *Tunnel) (exist bool) {
	GetDb().JsonDb.Tasks.Range(func(key, value interface{}) bool {
		v := value.(*Tunnel)
		if v.Client.Id == s.Id && v.Port == t.Port && t.Port != 0 {
			exist = true
			return false
		}
		return true
	})
	return
}

func (s *Client) GetTunnelNum() (num int) {
	GetDb().JsonDb.Tasks.Range(func(key, value interface{}) bool {
		v := value.(*Tunnel)
		if v.Client.Id == s.Id {
			num++
		}
		return true
	})

	GetDb().JsonDb.Hosts.Range(func(key, value interface{}) bool {
		v := value.(*Host)
		if v.Client.Id == s.Id {
			num++
		}
		return true
	})
	return
}

func (s *Client) HasHost(h *Host) bool {
	var has bool
	GetDb().JsonDb.Hosts.Range(func(key, value interface{}) bool {
		v := value.(*Host)
		if v.Client.Id == s.Id && v.Host == h.Host && h.Location == v.Location {
			has = true
			return false
		}
		return true
	})
	return has
}

type Tunnel struct {
	Id           int
	Port         int
	ServerIp     string
	Mode         string
	Status       bool
	RunStatus    bool
	Client       *Client
	Ports        string
	Flow         *Flow
	Password     string
	Remark       string
	TargetAddr   string
	NoStore      bool
	LocalPath    string
	StripPre     string
	ProtoVersion string
	Target       *Target
	MultiAccount *MultiAccount
	// IdleAutoStopMinutes 无活动连接满该分钟数后自动停用；≤0 关闭。仅 tcp/udp。
	IdleAutoStopMinutes int `json:"IdleAutoStopMinutes,omitempty"`
	// 隧道级源地址 ACL（空=不限制）；规则支持 IP/CIDR/geoip:/geosite:。
	BlackIpList    []string `json:"BlackIpList,omitempty"`
	WhiteIpList    []string `json:"WhiteIpList,omitempty"`
	IpListPriority int      `json:"IpListPriority,omitempty"`
	// HttpProxy / Socks5Proxy：仅 mixProxy 模式使用；两者皆 false 时运行时默认都开。
	HttpProxy   bool `json:"HttpProxy,omitempty"`
	Socks5Proxy bool `json:"Socks5Proxy,omitempty"`
	Health
	sync.RWMutex
}

type Health struct {
	HealthCheckTimeout  int
	HealthMaxFail       int
	HealthCheckInterval int
	HealthNextTime      time.Time
	HealthMap           map[string]int
	HttpHealthUrl       string
	HealthRemoveArr     []string
	HealthCheckType     string
	HealthCheckTarget   string
	sync.RWMutex
}

type Host struct {
	Id           int
	Host         string //host
	HeaderChange string //header change
	// ResponseHeaderChange 响应头改写；空=不改；`-Name` 或 `Name:` 删除；`Name: value` 设置。
	ResponseHeaderChange string `json:"ResponseHeaderChange,omitempty"`
	HostChange           string //host change
	Location             string //url router
	// PathRewrite URL 路径重写；每行 `旧前缀 => 新前缀`，空=不改。
	PathRewrite  string `json:"PathRewrite,omitempty"`
	Remark       string //remark
	Scheme       string //http https all
	CertFilePath string
	KeyFilePath  string
	NoStore      bool
	IsClose      bool
	AutoHttps    bool // 自动 https（301 跳转）
	// AutoSSL ACME 自动申请/续签；默认 false，与手工证书路径并存。
	AutoSSL bool `json:"AutoSSL,omitempty"`
	// Cors Access-Control-Allow-Origin；空=不注入；可为 * 或具体 Origin。
	Cors   string `json:"Cors,omitempty"`
	Flow   *Flow
	Client *Client
	Target *Target //目标
	// HttpsJustProxy TLS 直通：true 时即使配置了证书也按 SNI 原样转发，不在入口终止 TLS。
	// 默认 false：有证书则终止，无证书则仍走透传（与旧行为一致）。
	HttpsJustProxy bool `json:"HttpsJustProxy,omitempty"`
	// 域名级源地址 ACL（空=不限制）；规则支持 IP/CIDR/geoip:/geosite:。
	BlackIpList    []string `json:"BlackIpList,omitempty"`
	WhiteIpList    []string `json:"WhiteIpList,omitempty"`
	IpListPriority int      `json:"IpListPriority,omitempty"`
	Health         `json:"-"`
	sync.RWMutex
}

// User 独立用户实体。无 users.json 时面板行为仍等同旧 yisier。
type User struct {
	Id       int
	UserName string
	Password string
	// APIKeyHash：Bearer Token 的 SHA-256 hex；空表示未签发。
	APIKeyHash string `json:"api_key_hash,omitempty"`
	Status     bool
	Remark     string
	// MaxClientNum / MaxTunnelNum：0=不限，兼容旧记录缺字段。
	MaxClientNum int
	MaxTunnelNum int
	ExpireTime   string
	CreateTime   string
	sync.RWMutex
}

type Target struct {
	nowIndex   int
	TargetStr  string
	TargetArr  []string
	LocalProxy bool
	// ProxyProtocol 向后端写 PROXY 头；0=关，1=v1，2=v2。默认 0。
	ProxyProtocol int `json:"ProxyProtocol,omitempty"`
	sync.RWMutex
}

type MultiAccount struct {
	AccountMap map[string]string // multi account and pwd
}

func (s *Target) GetRandomTarget() (string, error) {
	s.Lock()
	if s.TargetArr == nil {
		arr := strings.Split(s.TargetStr, "\n")
		s.TargetArr = make([]string, 0, len(arr))
		for _, v := range arr {
			v = strings.TrimRight(v, "\r")
			if v != "" {
				s.TargetArr = append(s.TargetArr, v)
			}
		}
	}
	if len(s.TargetArr) == 0 {
		s.Unlock()
		return "", errors.New("all inward-bending targets are offline")
	}
	if s.nowIndex >= len(s.TargetArr)-1 {
		s.nowIndex = -1
	}
	s.nowIndex++
	addr := s.TargetArr[s.nowIndex]
	s.Unlock()
	return addr, nil
}

// Glob 保存管理端「全局参数」，可通过 Web 页面热更新。
// 指针字段用于区分「尚未初始化」和「显式设置为 false/空」：
// 首次启动时会从 nps.conf 迁移，之后以 global.json 为准。
type Glob struct {
	BlackIpList []string
	// WhiteIpList 全局白名单；与 BlackIpList / IpListPriority 组成源地址 ACL。
	WhiteIpList []string `json:"WhiteIpList,omitempty"`
	// IpListPriority：0=DenyFirst（默认），1=AllowFirst。
	IpListPriority int `json:"IpListPriority,omitempty"`
	ServerUrl string
	// AppName 面板产品名（侧栏/登录品牌等）；空=NPS。热更新，存 global.json。
	AppName string `json:"AppName,omitempty"`

	AllowPorts              *string `json:"AllowPorts,omitempty"`
	IpLimit                 *bool   `json:"IpLimit,omitempty"`
	ShowHttpProxyPort       *bool   `json:"ShowHttpProxyPort,omitempty"`
	AllowFlowLimit          *bool   `json:"AllowFlowLimit,omitempty"`
	AllowRateLimit          *bool   `json:"AllowRateLimit,omitempty"`
	AllowConnectionNumLimit *bool   `json:"AllowConnectionNumLimit,omitempty"`
	AllowTunnelNumLimit     *bool   `json:"AllowTunnelNumLimit,omitempty"`
	AllowMultiIp            *bool   `json:"AllowMultiIp,omitempty"`
	AllowLocalProxy         *bool   `json:"AllowLocalProxy,omitempty"`
	AllowUserLogin          *bool   `json:"AllowUserLogin,omitempty"`
	AllowUserRegister       *bool   `json:"AllowUserRegister,omitempty"`
	AllowUserChangeUsername *bool   `json:"AllowUserChangeUsername,omitempty"`
	OpenCaptcha             *bool   `json:"OpenCaptcha,omitempty"`
	SystemInfoDisplay       *bool   `json:"SystemInfoDisplay,omitempty"`
	HttpAddOriginHeader     *bool   `json:"HttpAddOriginHeader,omitempty"`

	// --- 第二批起：原 nps.conf 热更新项（指针=尚未从 conf 迁移）---
	WebWelcome  *string `json:"WebWelcome,omitempty"`
	WebSubtitle *string `json:"WebSubtitle,omitempty"`

	AllowRemoteUpdate *bool `json:"AllowRemoteUpdate,omitempty"`
	AllowXRealIp      *bool `json:"AllowXRealIp,omitempty"`

	WafEnable              *bool   `json:"WafEnable,omitempty"`
	WafProtectPanel        *bool   `json:"WafProtectPanel,omitempty"`
	WafUABlacklistEnable   *bool   `json:"WafUABlacklistEnable,omitempty"`
	WafBlockForeignEnable  *bool   `json:"WafBlockForeignEnable,omitempty"`
	WafCustomRulesEnable   *bool   `json:"WafCustomRulesEnable,omitempty"`
	WafBlockForeignUnknown *bool   `json:"WafBlockForeignUnknown,omitempty"`
	WafCountryHeaders      *string `json:"WafCountryHeaders,omitempty"`

	RegisterRateLimit   *int    `json:"RegisterRateLimit,omitempty"`
	RegisterEmailVerify *bool   `json:"RegisterEmailVerify,omitempty"`
	SmtpHost            *string `json:"SmtpHost,omitempty"`
	SmtpPort            *int    `json:"SmtpPort,omitempty"`
	SmtpSecurity        *string `json:"SmtpSecurity,omitempty"`
	SmtpUser            *string `json:"SmtpUser,omitempty"`
	SmtpPass            *string `json:"SmtpPass,omitempty"`
	SmtpFrom            *string `json:"SmtpFrom,omitempty"`

	AllowWebTerminal *bool `json:"AllowWebTerminal,omitempty"`

	NotifyEnable               *bool   `json:"NotifyEnable,omitempty"`
	NotifyTitlePrefix          *string `json:"NotifyTitlePrefix,omitempty"`
	NotifyMinInterval          *int    `json:"NotifyMinInterval,omitempty"`
	NotifyErrorPageMinInterval *int    `json:"NotifyErrorPageMinInterval,omitempty"`
	NotifyQueueSize            *int    `json:"NotifyQueueSize,omitempty"`
	NotifyHTTPTimeout          *int    `json:"NotifyHTTPTimeout,omitempty"`
	NotifyPublicClient         *bool   `json:"NotifyPublicClient,omitempty"`
	NotifyWecomWebhook         *string `json:"NotifyWecomWebhook,omitempty"`
	NotifyTelegramBotToken     *string `json:"NotifyTelegramBotToken,omitempty"`
	NotifyTelegramChatID       *string `json:"NotifyTelegramChatID,omitempty"`
	NotifyWebhookURL           *string `json:"NotifyWebhookURL,omitempty"`
	NotifyWebhookHeader        *string `json:"NotifyWebhookHeader,omitempty"`

	HttpErrorPageBrand    *string `json:"HttpErrorPageBrand,omitempty"`
	HttpErrorPageBrandURL *string `json:"HttpErrorPageBrandURL,omitempty"`

	MaxClients          *int `json:"MaxClients,omitempty"`
	MaxTunnelsPerClient *int `json:"MaxTunnelsPerClient,omitempty"`
	MaxGlobalConn       *int `json:"MaxGlobalConn,omitempty"`

	ReleaseAPIURL       *string `json:"ReleaseAPIURL,omitempty"`
	ReleaseDownloadBase *string `json:"ReleaseDownloadBase,omitempty"`

	sync.RWMutex
}
