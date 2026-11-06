package waf

import (
	"encoding/json"
	"net"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ehang.io/nps/lib/acl"
	"ehang.io/nps/lib/common"
	"github.com/astaxie/beego/logs"
)

// Scope 规则作用域。
const (
	ScopeHTTP = "http"
	ScopeTCP  = "tcp"
	ScopeAll  = "all"
)

// Reason 拦截原因码。
const (
	ReasonUA      = "ua"
	ReasonForeign = "foreign"
	ReasonRule    = "rule:"
	ReasonAllow   = "allow"
)

// Options 来自 nps.conf（改后需重启）。
type Options struct {
	Enable               bool
	UABlacklistEnable    bool
	BlockForeignEnable   bool
	CustomRulesEnable    bool
	BlockForeignUnknown  bool
	CountryHeaders       []string
	RulesPath            string
	ProtectPanel         bool // waf_protect_panel：管理面板是否走同一套 WAF
}

// ProtectPanelEnabled 总开关已开且配置保护管理面板。
func ProtectPanelEnabled() bool {
	o, _ := opts.Load().(Options)
	return o.Enable && o.ProtectPanel
}

// ListedItem 带说明的列表项（UA / IP 例外）；面板小白编辑用。
// Enable 为指针：缺省/省略视为开启，显式 false 为关闭。
type ListedItem struct {
	Value  string `json:"value"`
	Remark string `json:"remark,omitempty"`
	Enable *bool  `json:"enable,omitempty"`
}

// FileConfig 对应 conf/waf.json（可热加载）。
type FileConfig struct {
	UABlacklist        []string     `json:"ua_blacklist"`
	UABlacklistItems   []ListedItem `json:"ua_blacklist_items,omitempty"`
	UABlacklistIsRegex bool         `json:"ua_blacklist_is_regex"`
	ForeignAllowCodes  []string     `json:"foreign_allow_codes"`
	IPAllowlist        []string     `json:"ip_allowlist"`
	IPAllowlistItems   []ListedItem `json:"ip_allowlist_items,omitempty"`
	Rules              []CustomRule `json:"rules"`
}

// CustomRule 自定义 allow/deny。
type CustomRule struct {
	ID       string   `json:"id"`
	Enable   bool     `json:"enable"`
	Action   string   `json:"action"` // allow | deny
	Priority int      `json:"priority"`
	When     RuleWhen `json:"when"`
	Scope    string   `json:"scope"`             // http | tcp | all
	Remark   string   `json:"remark,omitempty"`  // 面板说明
	Builtin  bool     `json:"builtin,omitempty"` // 内置场景：可改可关，面板不可删
}

// RuleWhen 匹配条件（字段均可选）。
type RuleWhen struct {
	Methods    []string          `json:"methods"`
	PathPrefix []string          `json:"path_prefix"`
	PathRegex  []string          `json:"path_regex"`
	Headers    map[string]string `json:"headers"`
	UAContains []string          `json:"ua_contains"`
	UARegex    []string          `json:"ua_regex"`
	IPIn       []string          `json:"ip_in"`
}

// Result 检查结果。
type Result struct {
	Allowed bool
	Reason  string
	RuleID  string
}

type compiledFile struct {
	raw            FileConfig
	uaRegexes      []*regexp.Regexp
	uaSubstr       []string
	allowCodes     map[string]struct{}
	rules          []compiledRule
	modTime        time.Time
	path           string
}

type compiledRule struct {
	CustomRule
	pathRegex []*regexp.Regexp
	uaRegex   []*regexp.Regexp
}

var (
	opts atomic.Value // Options
	file atomic.Value // *compiledFile

	reloadMu   sync.Mutex
	lastWarnAt atomic.Int64 // 缺 geo 时限频 Warn
	started    atomic.Bool
)

func init() {
	opts.Store(Options{})
	file.Store(&compiledFile{allowCodes: map[string]struct{}{"CN": {}}})
}

// Configure 应用 conf 开关并加载规则文件；可重复调用（测试用）。
func Configure(o Options) {
	if strings.TrimSpace(o.RulesPath) == "" {
		o.RulesPath = filepath.Join(common.GetRunPath(), "conf", "waf.json")
	} else {
		o.RulesPath = common.ResolveConfigPath(o.RulesPath)
	}
	if len(o.CountryHeaders) == 0 {
		o.CountryHeaders = []string{"CF-IPCountry"}
	}
	opts.Store(o)
	_ = reloadRules(true)
	if o.Enable && !started.Swap(true) {
		go watchRules()
	}
}

// Enabled 总开关。
func Enabled() bool {
	o, _ := opts.Load().(Options)
	return o.Enable
}

// CurrentOptions 返回当前开关快照。
func CurrentOptions() Options {
	o, _ := opts.Load().(Options)
	return o
}

// CurrentFile 返回当前规则文件快照（只读）。
func CurrentFile() FileConfig {
	cf, _ := file.Load().(*compiledFile)
	if cf == nil {
		return FileConfig{}
	}
	return cf.raw
}

// SaveFile 写回 waf.json 并立即热加载。
func SaveFile(cfg FileConfig) error {
	NormalizeFileConfig(&cfg)
	o, _ := opts.Load().(Options)
	path := o.RulesPath
	if path == "" {
		path = filepath.Join(common.GetRunPath(), "conf", "waf.json")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return reloadRules(true)
}

// RulesPath 当前规则路径。
func RulesPath() string {
	o, _ := opts.Load().(Options)
	return o.RulesPath
}

func watchRules() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		_ = reloadRules(false)
	}
}

func reloadRules(force bool) error {
	reloadMu.Lock()
	defer reloadMu.Unlock()
	o, _ := opts.Load().(Options)
	path := o.RulesPath
	if path == "" {
		return nil
	}
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			empty := &compiledFile{
				path:       path,
				allowCodes: map[string]struct{}{"CN": {}},
				raw: FileConfig{
					ForeignAllowCodes: []string{"CN"},
				},
			}
			file.Store(empty)
			return nil
		}
		return err
	}
	cur, _ := file.Load().(*compiledFile)
	if !force && cur != nil && cur.path == path && cur.modTime.Equal(fi.ModTime()) {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var raw FileConfig
	if len(strings.TrimSpace(string(data))) > 0 {
		if err := json.Unmarshal(data, &raw); err != nil {
			logs.Warn("WAF 规则文件解析失败，保留旧规则: %v", err)
			return err
		}
	}
		NormalizeFileConfig(&raw)
		compiled, err := compileFile(raw, path, fi.ModTime())
		if err != nil {
			logs.Warn("WAF 规则编译失败，保留旧规则: %v", err)
			return err
		}
		file.Store(compiled)
		logs.Info("WAF 规则已加载: %s", path)
		return nil
	}
	
	func compileFile(raw FileConfig, path string, mod time.Time) (*compiledFile, error) {
		cf := &compiledFile{
			raw:        raw,
			path:       path,
			modTime:    mod,
			allowCodes: make(map[string]struct{}),
		}
		if len(raw.ForeignAllowCodes) == 0 {
			cf.allowCodes["CN"] = struct{}{}
		} else {
			for _, c := range raw.ForeignAllowCodes {
				c = strings.ToUpper(strings.TrimSpace(c))
				if c != "" {
					cf.allowCodes[c] = struct{}{}
				}
			}
		}
		uaPatterns := effectiveUABlacklist(raw)
		if raw.UABlacklistIsRegex {
			for _, p := range uaPatterns {
				p = strings.TrimSpace(p)
				if p == "" {
					continue
				}
				re, err := regexp.Compile(p)
				if err != nil {
					return nil, err
				}
				cf.uaRegexes = append(cf.uaRegexes, re)
			}
		} else {
			for _, p := range uaPatterns {
				p = strings.TrimSpace(p)
				if p != "" {
					cf.uaSubstr = append(cf.uaSubstr, strings.ToLower(p))
				}
			}
		}
	rules := append([]CustomRule(nil), raw.Rules...)
	sort.SliceStable(rules, func(i, j int) bool {
		return rules[i].Priority < rules[j].Priority
	})
	for _, r := range rules {
		if !r.Enable {
			continue
		}
		cr := compiledRule{CustomRule: r}
		for _, p := range r.When.PathRegex {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			re, err := regexp.Compile(p)
			if err != nil {
				return nil, err
			}
			cr.pathRegex = append(cr.pathRegex, re)
		}
		for _, p := range r.When.UARegex {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			re, err := regexp.Compile(p)
			if err != nil {
				return nil, err
			}
			cr.uaRegex = append(cr.uaRegex, re)
		}
		scope := strings.ToLower(strings.TrimSpace(r.Scope))
		if scope == "" {
			cr.Scope = ScopeAll
		}
		action := strings.ToLower(strings.TrimSpace(r.Action))
		if action != "allow" && action != "deny" {
			continue
		}
		cr.Action = action
		cf.rules = append(cf.rules, cr)
	}
	return cf, nil
}

// CheckHTTP 域名 HTTP 入口检查（含 UA / 国家头 / 自定义规则）。
// trustCountryHeader：仅当本次请求已按 allow_x_real_ip 信任转发头时为 true。
func CheckHTTP(r *http.Request, clientIP string, trustCountryHeader bool) Result {
	o, _ := opts.Load().(Options)
	if !o.Enable {
		return Result{Allowed: true}
	}
	cf, _ := file.Load().(*compiledFile)
	if cf == nil {
		cf = &compiledFile{allowCodes: map[string]struct{}{"CN": {}}}
	}
	ip := normalizeIP(clientIP)
	ua := ""
	if r != nil {
		ua = r.Header.Get("User-Agent")
	}

	// 1) IP allowlist / allow 规则短路
	if matchIPList(ip, cf.raw.IPAllowlist) {
		return Result{Allowed: true, Reason: ReasonAllow, RuleID: "ip_allowlist"}
	}
	if hit, id := matchCustom(cf, r, ip, ua, ScopeHTTP, "allow"); hit {
		return Result{Allowed: true, Reason: ReasonAllow, RuleID: id}
	}

	// 2) UA 黑名单
	if o.UABlacklistEnable && uaBlocked(cf, ua) {
		return Result{Allowed: false, Reason: ReasonUA}
	}

	// 3) 禁国外
	if o.BlockForeignEnable {
		if res := checkForeign(o, cf, ip, countryFromRequest(r, o.CountryHeaders, trustCountryHeader)); !res.Allowed {
			return res
		}
	}

	// 4) 自定义 deny
	if o.CustomRulesEnable {
		if hit, id := matchCustom(cf, r, ip, ua, ScopeHTTP, "deny"); hit {
			return Result{Allowed: false, Reason: ReasonRule + id, RuleID: id}
		}
	}
	return Result{Allowed: true}
}

// CheckIP TCP 等无 UA 入口：IP allowlist / 禁国外 / 适用自定义规则。
func CheckIP(clientIP string) Result {
	o, _ := opts.Load().(Options)
	if !o.Enable {
		return Result{Allowed: true}
	}
	cf, _ := file.Load().(*compiledFile)
	if cf == nil {
		cf = &compiledFile{allowCodes: map[string]struct{}{"CN": {}}}
	}
	ip := normalizeIP(clientIP)
	if matchIPList(ip, cf.raw.IPAllowlist) {
		return Result{Allowed: true, Reason: ReasonAllow, RuleID: "ip_allowlist"}
	}
	if hit, id := matchCustom(cf, nil, ip, "", ScopeTCP, "allow"); hit {
		return Result{Allowed: true, Reason: ReasonAllow, RuleID: id}
	}
	if o.BlockForeignEnable {
		if res := checkForeign(o, cf, ip, ""); !res.Allowed {
			return res
		}
	}
	if o.CustomRulesEnable {
		if hit, id := matchCustom(cf, nil, ip, "", ScopeTCP, "deny"); hit {
			return Result{Allowed: false, Reason: ReasonRule + id, RuleID: id}
		}
	}
	return Result{Allowed: true}
}

func checkForeign(o Options, cf *compiledFile, ip, countryCode string) Result {
	if ip == "" {
		return Result{Allowed: true}
	}
	// 私网 / 环回始终放行
	if acl.MatchGeoIP("private", ip) {
		return Result{Allowed: true}
	}
	code := strings.ToUpper(strings.TrimSpace(countryCode))
	// 可信国家头明确为允许列表（如 CN）→ 放行；明确其它国家 → 拦截
	if code != "" && code != "XX" {
		if _, ok := cf.allowCodes[code]; ok {
			return Result{Allowed: true}
		}
		return Result{Allowed: false, Reason: ReasonForeign}
	}
	geoReady := acl.GeoDataConfigured() && acl.HasGeoIPCategory("cn")
	if geoReady {
		if acl.MatchGeoIP("cn", ip) {
			return Result{Allowed: true}
		}
		return Result{Allowed: false, Reason: ReasonForeign}
	}
	// 缺 geoip.dat：有 XX/空头且开启「未知拒绝」则拦截；否则跳过禁国外
	if (code == "XX" || code == "") && o.BlockForeignUnknown && countryCode != "" {
		return Result{Allowed: false, Reason: ReasonForeign}
	}
	now := time.Now().Unix()
	prev := lastWarnAt.Load()
	if now-prev >= 60 && lastWarnAt.CompareAndSwap(prev, now) {
		logs.Warn("WAF 禁国外已开启但 geoip.dat 未加载，本项跳过（其余 WAF 仍生效）")
	}
	return Result{Allowed: true}
}

func countryFromRequest(r *http.Request, headers []string, trust bool) string {
	if !trust || r == nil {
		return ""
	}
	for _, h := range headers {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		v := strings.TrimSpace(r.Header.Get(h))
		if v != "" {
			return v
		}
	}
	return ""
}

func uaBlocked(cf *compiledFile, ua string) bool {
	if ua == "" {
		return false
	}
	lower := strings.ToLower(ua)
	for _, s := range cf.uaSubstr {
		if strings.Contains(lower, s) {
			return true
		}
	}
	for _, re := range cf.uaRegexes {
		if re.MatchString(ua) {
			return true
		}
	}
	return false
}

func matchCustom(cf *compiledFile, r *http.Request, ip, ua, scope, action string) (bool, string) {
	o, _ := opts.Load().(Options)
	if action == "deny" && !o.CustomRulesEnable {
		return false, ""
	}
	for _, rule := range cf.rules {
		if rule.Action != action {
			continue
		}
		rs := strings.ToLower(rule.Scope)
		if rs == "" {
			rs = ScopeAll
		}
		if rs != ScopeAll && rs != scope {
			continue
		}
		if !ruleMatches(rule, r, ip, ua, scope) {
			continue
		}
		id := rule.ID
		if id == "" {
			id = "unnamed"
		}
		return true, id
	}
	return false, ""
}

func ruleMatches(rule compiledRule, r *http.Request, ip, ua, scope string) bool {
	w := rule.When
	if len(w.IPIn) > 0 && !matchIPList(ip, w.IPIn) {
		return false
	}
	if scope == ScopeHTTP || scope == ScopeAll {
		if r != nil {
			if len(w.Methods) > 0 {
				ok := false
				m := strings.ToUpper(r.Method)
				for _, want := range w.Methods {
					if strings.ToUpper(strings.TrimSpace(want)) == m {
						ok = true
						break
					}
				}
				if !ok {
					return false
				}
			}
			path := ""
			if r.URL != nil {
				path = r.URL.Path
			}
			if len(w.PathPrefix) > 0 {
				ok := false
				for _, p := range w.PathPrefix {
					if strings.HasPrefix(path, p) {
						ok = true
						break
					}
				}
				if !ok {
					return false
				}
			}
			if len(rule.pathRegex) > 0 {
				ok := false
				for _, re := range rule.pathRegex {
					if re.MatchString(path) {
						ok = true
						break
					}
				}
				if !ok {
					return false
				}
			}
			if len(w.Headers) > 0 {
				for k, v := range w.Headers {
					if !strings.Contains(strings.ToLower(r.Header.Get(k)), strings.ToLower(v)) {
						return false
					}
				}
			}
			if len(w.UAContains) > 0 {
				lower := strings.ToLower(ua)
				ok := false
				for _, s := range w.UAContains {
					if strings.Contains(lower, strings.ToLower(s)) {
						ok = true
						break
					}
				}
				if !ok {
					return false
				}
			}
			if len(rule.uaRegex) > 0 {
				ok := false
				for _, re := range rule.uaRegex {
					if re.MatchString(ua) {
						ok = true
						break
					}
				}
				if !ok {
					return false
				}
			}
		} else if scope == ScopeHTTP {
			// TCP 调用不会用 ScopeHTTP；若规则仅 HTTP 字段且无 r，则不匹配
			if len(w.Methods) > 0 || len(w.PathPrefix) > 0 || len(rule.pathRegex) > 0 ||
				len(w.Headers) > 0 || len(w.UAContains) > 0 || len(rule.uaRegex) > 0 {
				return false
			}
		}
	}
	return true
}

func matchIPList(ip string, rules []string) bool {
	if ip == "" || len(rules) == 0 {
		return false
	}
	return acl.MatchIP(ip, rules)
}

func normalizeIP(ipPort string) string {
	ipPort = strings.TrimSpace(ipPort)
	if ipPort == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(ipPort); err == nil {
		ipPort = host
	}
	ipPort = strings.Trim(ipPort, "[]")
	if addr, err := netip.ParseAddr(ipPort); err == nil {
		return addr.Unmap().String()
	}
	return common.GetIpByAddr(ipPort)
}
