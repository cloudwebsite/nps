package controllers

import (
	"encoding/json"
	"html/template"
	"net/http"
	"os"
	"strings"

	"ehang.io/nps/lib/acl"
	"ehang.io/nps/lib/install"
	"ehang.io/nps/lib/waf"
	"github.com/astaxie/beego"
)

// WafController 管理面 WAF 规则页（管理员）；开关只读提示改 conf 需重启。
type WafController struct {
	BaseController
}

func (s *WafController) Prepare() {
	s.BaseController.Prepare()
	if isAdmin, ok := s.GetSession("isAdmin").(bool); !ok || !isAdmin {
		s.denyForbidden()
	}
}

func (s *WafController) Index() {
	s.Data["menu"] = "waf"
	s.SetInfo("waf")
	o := waf.CurrentOptions()
		s.Data["waf_enable"] = o.Enable
		s.Data["waf_protect_panel"] = o.ProtectPanel
		s.Data["waf_ua_blacklist_enable"] = o.UABlacklistEnable
		s.Data["waf_block_foreign_enable"] = o.BlockForeignEnable
		s.Data["waf_custom_rules_enable"] = o.CustomRulesEnable
		s.Data["waf_block_foreign_unknown"] = o.BlockForeignUnknown
		s.Data["waf_country_headers"] = strings.Join(o.CountryHeaders, ",")
		s.Data["waf_rules_path"] = waf.RulesPath()
	s.Data["geoip_path"] = beego.AppConfig.DefaultString("geoip_path", "conf/geoip.dat")
	s.Data["geo_loaded"] = acl.GeoDataConfigured()
	ipPath, _ := acl.GeoFiles()
	s.Data["geo_version"] = install.GeoDataVersion(ipPath)
	s.Data["server_config_url"] = beego.AppConfig.String("web_base_url") + "/serverconfig/index"

	cfg := waf.CurrentFile()
	waf.NormalizeFileConfig(&cfg)
	cfgJSON, _ := json.Marshal(cfg)
	presetsJSON, _ := json.Marshal(waf.BuiltinPresets())
	// 嵌入 <script> 须用 JS 转义，避免 html/template 按 HTML 转义破坏 JSON
	s.Data["waf_cfg_json"] = template.JS(cfgJSON)
	s.Data["waf_presets_json"] = template.JS(presetsJSON)
	raw, _ := json.MarshalIndent(cfg, "", "  ")
	s.Data["waf_json"] = string(raw)
		s.Data["restart_hint"] = "功能开关在「全局参数」保存后立即生效；本页仅编辑 waf.json 规则（热加载）。geoip/规则文件路径仍在服务端配置，改路径需重启。"
	s.display("waf/index")
}

// Save 保存 waf.json（热加载）。
func (s *WafController) Save() {
	if s.Ctx.Request.Method != http.MethodPost {
		s.AjaxErr("method not allowed")
		return
	}
	body := s.GetString("waf_json")
	if strings.TrimSpace(body) == "" {
		s.AjaxErr("规则内容不能为空")
		return
	}
	var cfg waf.FileConfig
	if err := json.Unmarshal([]byte(body), &cfg); err != nil {
		s.AjaxErr("JSON 无效: " + err.Error())
		return
	}
	if err := waf.SaveFile(cfg); err != nil {
		s.auditPanel("waf.rules.save", "waf.json", 0, 0, "failure", err.Error(), nil)
		s.AjaxErr("保存失败: " + err.Error())
		return
	}
	s.auditPanel("waf.rules.save", "waf.json", 0, 0, "success", "", nil)
	s.AjaxOk("规则已保存并热加载")
}

// ApplyPreset 应用内置场景到当前规则并保存。
func (s *WafController) ApplyPreset() {
	if s.Ctx.Request.Method != http.MethodPost {
		s.AjaxErr("method not allowed")
		return
	}
	id := strings.TrimSpace(s.GetString("preset_id"))
	if id == "" {
		s.AjaxErr("缺少 preset_id")
		return
	}
	if id == "preset_block_foreign" {
		s.AjaxErr("请到「服务端配置」打开 waf_block_foreign_enable 并重启；境外运维请先加 IP 例外")
		return
	}
	cfg := waf.CurrentFile()
	if !waf.ApplyPreset(&cfg, id) {
		s.AjaxErr("未知场景: " + id)
		return
	}
	if err := waf.SaveFile(cfg); err != nil {
		s.auditPanel("waf.preset.apply", id, 0, 0, "failure", err.Error(), nil)
		s.AjaxErr("保存失败: " + err.Error())
		return
	}
	s.auditPanel("waf.preset.apply", id, 0, 0, "success", "", map[string]string{"preset_id": id})
	out, _ := json.Marshal(cfg)
	s.Data["json"] = map[string]interface{}{
		"status": 1,
		"msg":    "场景已应用并热加载",
		"data":   json.RawMessage(out),
	}
	s.ServeJSON()
	s.StopRun()
}

// Status 返回当前开关与 Geo 状态。
func (s *WafController) Status() {
	o := waf.CurrentOptions()
	ipPath, sitePath := acl.GeoFiles()
	type fileSt struct {
		Name    string `json:"name"`
		Path    string `json:"path"`
		Exists  bool   `json:"exists"`
		Size    int64  `json:"size"`
		ModTime string `json:"mod_time"`
	}
	files := []fileSt{}
	for _, f := range install.GeoFiles(ipPath, sitePath) {
		row := fileSt{Name: f.Name, Path: f.Path}
		if fi, err := os.Stat(f.Path); err == nil {
			row.Exists = true
			row.Size = fi.Size()
			row.ModTime = fi.ModTime().Local().Format("2006-01-02 15:04:05")
		}
		files = append(files, row)
	}
	s.Data["json"] = map[string]interface{}{
		"status": 1,
		"msg":    "ok",
		"data": map[string]interface{}{
				"enable":                o.Enable,
				"protect_panel":         o.ProtectPanel,
				"ua_blacklist_enable":   o.UABlacklistEnable,
				"block_foreign_enable":  o.BlockForeignEnable,
				"custom_rules_enable":   o.CustomRulesEnable,
				"block_foreign_unknown": o.BlockForeignUnknown,
			"geo_loaded":            acl.GeoDataConfigured(),
			"geo_version":           install.GeoDataVersion(ipPath),
			"geo_files":             files,
			"rules_path":            waf.RulesPath(),
		},
	}
	s.ServeJSON()
	s.StopRun()
}
