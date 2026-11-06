package runtimeopts

import (
	"strings"
	"time"

	"ehang.io/nps/lib/file"
	"ehang.io/nps/lib/install"
	"ehang.io/nps/lib/notify"
	"ehang.io/nps/lib/waf"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
)

// Apply 按 global.json 快照热应用：WAF 开关、运维通知、Release 源。
// 路径类（geoip / waf_rules_path）仍读 nps.conf，改路径需重启。
func Apply() {
	gs := file.GetDb().GetGlobalSettings()
	applyWAF(gs)
	applyNotify(gs)
	install.SetReleaseSource(gs.ReleaseAPIURL, gs.ReleaseDownloadBase)
}

func applyWAF(gs file.GlobalSettings) {
	headers := make([]string, 0)
	for _, h := range strings.Split(gs.WafCountryHeaders, ",") {
		h = strings.TrimSpace(h)
		if h != "" {
			headers = append(headers, h)
		}
	}
	rulesPath := strings.TrimSpace(beego.AppConfig.String("waf_rules_path"))
	if rulesPath == "" {
		rulesPath = "conf/waf.json"
	}
	waf.Configure(waf.Options{
		Enable:              gs.WafEnable,
		UABlacklistEnable:   gs.WafUABlacklistEnable,
		BlockForeignEnable:  gs.WafBlockForeignEnable,
		CustomRulesEnable:   gs.WafCustomRulesEnable,
		BlockForeignUnknown: gs.WafBlockForeignUnknown,
		ProtectPanel:        gs.WafProtectPanel,
		CountryHeaders:      headers,
		RulesPath:           rulesPath,
	})
}

func applyNotify(gs file.GlobalSettings) {
	timeoutSec := gs.NotifyHTTPTimeout
	if timeoutSec <= 0 {
		timeoutSec = 5
	}
	minSec := gs.NotifyMinInterval
	if minSec < 0 {
		minSec = 0
	}
	errPageSec := gs.NotifyErrorPageMinInterval
	if errPageSec < 0 {
		errPageSec = 0
	}
	queue := gs.NotifyQueueSize
	if queue <= 0 {
		queue = 64
	}
	prefix := strings.TrimSpace(gs.NotifyTitlePrefix)
	if prefix == "" {
		prefix = "[nps]"
	}
	cfg := notify.Config{
		Enable:               gs.NotifyEnable,
		TitlePrefix:          prefix,
		MinInterval:          time.Duration(minSec) * time.Second,
		ErrorPageMinInterval: time.Duration(errPageSec) * time.Second,
		QueueSize:            queue,
		HTTPTimeout:          time.Duration(timeoutSec) * time.Second,
		NotifyPublicClient:   gs.NotifyPublicClient,
		WecomWebhook:         gs.NotifyWecomWebhook,
		TelegramBotToken:     gs.NotifyTelegramBotToken,
		TelegramChatID:       gs.NotifyTelegramChatID,
		WebhookURL:           gs.NotifyWebhookURL,
		WebhookHeader:        gs.NotifyWebhookHeader,
	}
	if err := notify.ReplaceDefault(cfg); err != nil {
		logs.Warn("notify hot reload: %v", err)
	}
}
