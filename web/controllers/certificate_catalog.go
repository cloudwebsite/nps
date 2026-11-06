package controllers

import (
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/file"
)

// certificateCatalogEntry 托管证书目录中的一条记录（不含私钥内容）。
type certificateCatalogEntry struct {
	Scope         string   `json:"scope"`
	RelCertPath   string   `json:"rel_cert_path"`
	RelKeyPath    string   `json:"rel_key_path"`
	FileName      string   `json:"file_name"`
	HasKey        bool     `json:"has_key"`
	Subject       string   `json:"subject"`
	Issuer        string   `json:"issuer"`
	ExpiresAt     string   `json:"expires_at"`
	DaysRemaining int      `json:"days_remaining"`
	Fingerprint   string   `json:"fingerprint"`
	Status        string   `json:"status"`
	Valid         bool     `json:"valid"`
	Expired       bool     `json:"expired"`
	NotYetValid   bool     `json:"not_yet_valid"`
	Bindings      []string `json:"bindings"`
	Referenced    bool     `json:"referenced"`
	CanDelete     bool     `json:"can_delete"`
}

// certificateAcmeSummary ACME 缓存目录摘要；不枚举全部文件以免误删。
type certificateAcmeSummary struct {
	Path      string `json:"path"`
	RelPath   string `json:"rel_path"`
	Exists    bool   `json:"exists"`
	FileCount int    `json:"file_count"`
	Detail    string `json:"detail"`
}

// managedCatalogScopes 允许浏览/安全删除的托管目录（相对 conf/）。
var managedCatalogScopes = []string{"webssl", "bridgessl", "certs"}

// normalizeCatalogPath 统一路径比较：斜杠、去 ./、相对运行目录时尽量相对化。
func normalizeCatalogPath(raw string) string {
	raw = strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	if raw == "" {
		return ""
	}
	raw = strings.TrimPrefix(raw, "./")
	if filepath.IsAbs(raw) || (len(raw) > 1 && raw[1] == ':') {
		run := filepath.ToSlash(filepath.Clean(common.GetRunPath()))
		abs := filepath.ToSlash(filepath.Clean(raw))
		if strings.EqualFold(run, abs) {
			return ""
		}
		prefix := strings.TrimRight(run, "/") + "/"
		if len(abs) > len(prefix) && strings.EqualFold(abs[:len(prefix)], prefix) {
			return strings.TrimPrefix(abs[len(prefix):], "/")
		}
		return strings.ToLower(abs)
	}
	return strings.TrimPrefix(filepath.ToSlash(filepath.Clean(raw)), "/")
}

// pathsReferSame 判断两个配置路径是否指向同一文件。
func pathsReferSame(a, b string) bool {
	na, nb := normalizeCatalogPath(a), normalizeCatalogPath(b)
	if na == "" || nb == "" {
		return false
	}
	if strings.EqualFold(na, nb) {
		return true
	}
	return strings.EqualFold(filepath.Base(na), filepath.Base(nb)) &&
		strings.EqualFold(filepath.Dir(na), filepath.Dir(nb))
}

// collectCertificateBindings 收集 conf 键与 Host 对手工证书路径的引用。
func collectCertificateBindings(confValues map[string]string, hosts []*file.Host) map[string][]string {
	out := make(map[string][]string)
	add := func(path, label string) {
		key := normalizeCatalogPath(path)
		if key == "" {
			return
		}
		for _, existing := range out[key] {
			if existing == label {
				return
			}
		}
		out[key] = append(out[key], label)
	}
	if confValues != nil {
		pairs := []struct{ key, label string }{
			{"web_cert_file", "conf:web_cert_file"},
			{"web_key_file", "conf:web_key_file"},
			{"bridge_cert_file", "conf:bridge_cert_file"},
			{"bridge_key_file", "conf:bridge_key_file"},
		}
		for _, p := range pairs {
			if v := strings.TrimSpace(confValues[p.key]); v != "" {
				add(v, p.label)
			}
		}
	}
	for _, h := range hosts {
		if h == nil {
			continue
		}
		label := fmt.Sprintf("host:%s#%d", h.Host, h.Id)
		if strings.TrimSpace(h.CertFilePath) != "" {
			add(h.CertFilePath, label+" CertFilePath")
		}
		if strings.TrimSpace(h.KeyFilePath) != "" {
			add(h.KeyFilePath, label+" KeyFilePath")
		}
	}
	return out
}

func bindingsForPaths(index map[string][]string, paths ...string) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, p := range paths {
		key := normalizeCatalogPath(p)
		for _, label := range index[key] {
			if _, ok := seen[label]; ok {
				continue
			}
			seen[label] = struct{}{}
			out = append(out, label)
		}
	}
	sort.Strings(out)
	return out
}

func isCertFileName(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".pem" || ext == ".crt" || ext == ".cer"
}

func isKeyFileName(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".key"
}

func inspectLeafCertificateFile(certPath string) (certificateInspection, error) {
	data, err := os.ReadFile(certPath)
	if err != nil {
		return certificateInspection{Message: firstCertLine(err.Error())}, err
	}
	var derBlocks [][]byte
	rest := data
	for {
		block, next := pem.Decode(rest)
		if block == nil {
			break
		}
		rest = next
		if block.Type == "CERTIFICATE" {
			derBlocks = append(derBlocks, block.Bytes)
		}
	}
	if len(derBlocks) == 0 {
		return certificateInspection{Message: "未找到 PEM 证书块"}, errors.New("未找到 PEM 证书块")
	}
	inspection, leaf, err := inspectCertificateChain(derBlocks)
	if err != nil {
		return inspection, err
	}
	if leaf != nil {
		now := time.Now()
		inspection.NotYetValid = now.Before(leaf.NotBefore)
		inspection.Expired = now.After(leaf.NotAfter)
		switch {
		case inspection.NotYetValid:
			inspection.Message = "证书尚未生效"
		case inspection.Expired:
			inspection.Message = "证书已过期"
		case inspection.DaysRemaining < 15:
			inspection.Message = "证书即将到期"
			inspection.Valid = true
		default:
			inspection.Message = "证书可读"
			inspection.Valid = true
		}
	}
	return inspection, nil
}

// scanManagedCertificateDirectory 扫描单个托管目录并配对 .key。
func scanManagedCertificateDirectory(absDir, scope string, bindings map[string][]string) ([]certificateCatalogEntry, error) {
	entries := make([]certificateCatalogEntry, 0)
	st, err := os.Stat(absDir)
	if err != nil {
		if os.IsNotExist(err) {
			return entries, nil
		}
		return nil, err
	}
	if !st.IsDir() {
		return entries, nil
	}
	files, err := os.ReadDir(absDir)
	if err != nil {
		return nil, err
	}

	keysByBase := make(map[string]string)
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		if isKeyFileName(name) {
			base := strings.TrimSuffix(name, filepath.Ext(name))
			keysByBase[base] = name
		}
	}

	seenKey := make(map[string]struct{})
	for _, f := range files {
		if f.IsDir() || !isCertFileName(f.Name()) {
			continue
		}
		name := f.Name()
		base := strings.TrimSuffix(name, filepath.Ext(name))
		relCert := filepath.ToSlash(filepath.Join("conf", scope, name))
		relKey := ""
		hasKey := false
		if kn, ok := keysByBase[base]; ok {
			relKey = filepath.ToSlash(filepath.Join("conf", scope, kn))
			hasKey = true
			seenKey[kn] = struct{}{}
		}
		entry := certificateCatalogEntry{
			Scope:       scope,
			RelCertPath: relCert,
			RelKeyPath:  relKey,
			FileName:    name,
			HasKey:      hasKey,
		}
		absCert := filepath.Join(absDir, name)
		if hasKey {
			inspection, status, inspectErr := inspectConfiguredCertificateFiles(relCert, relKey, "")
			entry.Subject = inspection.Subject
			entry.Issuer = inspection.Issuer
			entry.ExpiresAt = inspection.ExpiresAt
			entry.DaysRemaining = inspection.DaysRemaining
			entry.Fingerprint = inspection.Fingerprint
			entry.Valid = inspection.Valid
			entry.Expired = inspection.Expired
			entry.NotYetValid = inspection.NotYetValid
			entry.Status = status
			if inspectErr != nil && entry.Status == "" {
				entry.Status = firstCertLine(inspectErr.Error())
			}
		} else {
			inspection, inspectErr := inspectLeafCertificateFile(absCert)
			entry.Subject = inspection.Subject
			entry.Issuer = inspection.Issuer
			entry.ExpiresAt = inspection.ExpiresAt
			entry.DaysRemaining = inspection.DaysRemaining
			entry.Fingerprint = inspection.Fingerprint
			entry.Valid = inspection.Valid
			entry.Expired = inspection.Expired
			entry.NotYetValid = inspection.NotYetValid
			entry.Status = inspection.Message
			if inspectErr != nil && entry.Status == "" {
				entry.Status = firstCertLine(inspectErr.Error())
			}
			if entry.Status == "" {
				entry.Status = "仅有证书文件（无配对私钥）"
			}
		}
		entry.Bindings = bindingsForPaths(bindings, relCert, relKey)
		entry.Referenced = len(entry.Bindings) > 0
		entry.CanDelete = !entry.Referenced
		entries = append(entries, entry)
	}

	// 孤立私钥：仍列出，便于清理。
	for base, kn := range keysByBase {
		if _, ok := seenKey[kn]; ok {
			continue
		}
		_ = base
		relKey := filepath.ToSlash(filepath.Join("conf", scope, kn))
		entry := certificateCatalogEntry{
			Scope:       scope,
			RelKeyPath:  relKey,
			FileName:    kn,
			HasKey:      true,
			Status:      "孤立私钥（无配对证书）",
			Bindings:    bindingsForPaths(bindings, relKey),
		}
		entry.Referenced = len(entry.Bindings) > 0
		entry.CanDelete = !entry.Referenced
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Scope != entries[j].Scope {
			return entries[i].Scope < entries[j].Scope
		}
		return entries[i].FileName < entries[j].FileName
	})
	return entries, nil
}

func summarizeAcmeCache(confValues map[string]string) certificateAcmeSummary {
	cachePath := ""
	if confValues != nil {
		cachePath = strings.TrimSpace(confValues["ssl_path"])
	}
	rel := "conf/ssl"
	if cachePath == "" {
		cachePath = filepath.Join(common.GetRunPath(), "conf", "ssl")
	} else if !filepath.IsAbs(cachePath) {
		rel = filepath.ToSlash(cachePath)
		cachePath = filepath.Join(common.GetRunPath(), cachePath)
	} else {
		rel = normalizeCatalogPath(cachePath)
	}
	sum := certificateAcmeSummary{
		Path:    cachePath,
		RelPath: rel,
		Detail:  "ACME 缓存仅展示摘要；请勿手工删除正在使用的证书文件。",
	}
	st, err := os.Stat(cachePath)
	if err != nil || !st.IsDir() {
		sum.Exists = false
		sum.Detail = "ACME 缓存目录尚不存在（未申请过自动证书时正常）。"
		return sum
	}
	sum.Exists = true
	_ = filepath.Walk(cachePath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		sum.FileCount++
		return nil
	})
	return sum
}

// listCertificateCatalog 列出托管目录证书 + ACME 摘要。
func listCertificateCatalog(confValues map[string]string, hosts []*file.Host) ([]certificateCatalogEntry, certificateAcmeSummary, error) {
	bindings := collectCertificateBindings(confValues, hosts)
	var all []certificateCatalogEntry
	for _, scope := range managedCatalogScopes {
		abs := filepath.Join(common.GetRunPath(), "conf", scope)
		part, err := scanManagedCertificateDirectory(abs, scope, bindings)
		if err != nil {
			return nil, certificateAcmeSummary{}, err
		}
		all = append(all, part...)
	}
	return all, summarizeAcmeCache(confValues), nil
}

func isManagedCatalogRelPath(rel string) bool {
	n := normalizeCatalogPath(rel)
	if n == "" {
		return false
	}
	for _, scope := range managedCatalogScopes {
		prefix := "conf/" + scope + "/"
		if strings.HasPrefix(strings.ToLower(n), strings.ToLower(prefix)) {
			cleaned := filepath.Clean(n)
			if strings.Contains(cleaned, "..") {
				return false
			}
			return true
		}
	}
	return false
}

// deleteUnusedManagedCertificateFiles 仅删除未被引用的托管文件（可同时删配对 key）。
func deleteUnusedManagedCertificateFiles(relCertPath, relKeyPath string, bindings map[string][]string) error {
	relCertPath = strings.TrimSpace(relCertPath)
	relKeyPath = strings.TrimSpace(relKeyPath)
	if relCertPath == "" && relKeyPath == "" {
		return errors.New("未指定要删除的证书路径")
	}
	for _, p := range []string{relCertPath, relKeyPath} {
		if p == "" {
			continue
		}
		if !isManagedCatalogRelPath(p) {
			return fmt.Errorf("拒绝删除非托管路径: %s", p)
		}
	}
	refs := bindingsForPaths(bindings, relCertPath, relKeyPath)
	if len(refs) > 0 {
		return fmt.Errorf("仍被引用，拒绝删除: %s", strings.Join(refs, ", "))
	}
	removeOne := func(rel string) error {
		if rel == "" {
			return nil
		}
		abs := common.ResolveConfigPath(rel)
		// 二次确认解析后仍在托管目录内
		runConf := filepath.Clean(filepath.Join(common.GetRunPath(), "conf"))
		absClean := filepath.Clean(abs)
		relToConf, err := filepath.Rel(runConf, absClean)
		if err != nil || strings.HasPrefix(relToConf, "..") {
			return fmt.Errorf("路径越界: %s", rel)
		}
		scope := strings.Split(filepath.ToSlash(relToConf), "/")[0]
		okScope := false
		for _, s := range managedCatalogScopes {
			if strings.EqualFold(s, scope) {
				okScope = true
				break
			}
		}
		if !okScope {
			return fmt.Errorf("拒绝删除非托管路径: %s", rel)
		}
		if err := os.Remove(absClean); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := removeOne(relCertPath); err != nil {
		return err
	}
	if err := removeOne(relKeyPath); err != nil {
		return err
	}
	return nil
}

func hostListSnapshot() []*file.Host {
	var hosts []*file.Host
	if db := file.GetDb(); db != nil && db.JsonDb != nil {
		db.JsonDb.Hosts.Range(func(_, value interface{}) bool {
			if h, ok := value.(*file.Host); ok && h != nil {
				hosts = append(hosts, h)
			}
			return true
		})
	}
	return hosts
}

// Certificates 管理员证书目录页。
func (s *ServerConfigController) Certificates() {
	s.Data["menu"] = "certificates"
	s.SetInfo("certificates")
	confPath := serverConfigPath()
	raw, err := os.ReadFile(confPath)
	values := map[string]string{}
	msg := ""
	if err != nil {
		msg = "无法读取 conf/nps.conf：" + err.Error()
	} else {
		values = confKeyValues(string(raw))
	}
	entries, acme, listErr := listCertificateCatalog(values, hostListSnapshot())
	if listErr != nil {
		msg = "扫描证书目录失败：" + listErr.Error()
	}
	s.Data["conf_path"] = confPath
	s.Data["catalog_msg"] = msg
	s.Data["catalog_entries"] = entries
	s.Data["acme_summary"] = acme
	s.Data["web_cert_status"] = buildWebCertStatus(values)
	s.Data["bridge_cert_status"] = buildBridgeCertStatus(values)
	s.display("serverconfig/certificates")
}

// CatalogValidate POST：校验目录中的一对证书。
func (s *ServerConfigController) CatalogValidate() {
	if s.Ctx.Request.Method != http.MethodPost {
		s.AjaxErr("method not allowed")
		return
	}
	certPath := strings.TrimSpace(s.GetString("cert"))
	keyPath := strings.TrimSpace(s.GetString("key"))
	if certPath == "" {
		s.AjaxErr("缺少证书路径")
		return
	}
	if !isManagedCatalogRelPath(certPath) {
		s.AjaxErr("只能校验托管目录中的证书")
		return
	}
	if keyPath != "" && !isManagedCatalogRelPath(keyPath) {
		s.AjaxErr("只能校验托管目录中的私钥")
		return
	}
	if keyPath != "" {
		inspection, status, err := inspectConfiguredCertificateFiles(certPath, keyPath, "")
		if err != nil {
			inspection.Message = err.Error()
		} else if status != "" {
			inspection.Message = status
		}
		s.Data["json"] = map[string]interface{}{"status": 1, "msg": inspection.Message, "data": inspection}
		s.ServeJSON()
		s.StopRun()
		return
	}
	inspection, err := inspectLeafCertificateFile(common.ResolveConfigPath(certPath))
	if err != nil && inspection.Message == "" {
		inspection.Message = err.Error()
	}
	s.Data["json"] = map[string]interface{}{"status": 1, "msg": inspection.Message, "data": inspection}
	s.ServeJSON()
	s.StopRun()
}

// CatalogDelete POST：删除未被引用的托管证书/私钥。
func (s *ServerConfigController) CatalogDelete() {
	if s.Ctx.Request.Method != http.MethodPost {
		s.AjaxErr("method not allowed")
		return
	}
	certPath := strings.TrimSpace(s.GetString("cert"))
	keyPath := strings.TrimSpace(s.GetString("key"))
	raw, err := os.ReadFile(serverConfigPath())
	values := map[string]string{}
	if err == nil {
		values = confKeyValues(string(raw))
	}
	bindings := collectCertificateBindings(values, hostListSnapshot())
	if err := deleteUnusedManagedCertificateFiles(certPath, keyPath, bindings); err != nil {
		s.auditPanel("certificate.catalog_delete", "certificate", 0, 0, "failure", err.Error(), map[string]string{
			"cert": certPath, "key": keyPath,
		})
		s.AjaxErr(err.Error())
		return
	}
	s.auditPanel("certificate.catalog_delete", "certificate", 0, 0, "success", "", map[string]string{
		"cert": certPath, "key": keyPath,
	})
	s.AjaxOk("已删除未引用的托管证书文件")
}

// CatalogBind POST：将托管证书对写入面板或 Bridge 的 conf 路径键（需重启生效）。
func (s *ServerConfigController) CatalogBind() {
	if s.Ctx.Request.Method != http.MethodPost {
		s.AjaxErr("method not allowed")
		return
	}
	target := strings.ToLower(strings.TrimSpace(s.GetString("target")))
	certPath := strings.TrimSpace(s.GetString("cert"))
	keyPath := strings.TrimSpace(s.GetString("key"))
	if certPath == "" || keyPath == "" {
		s.AjaxErr("证书与私钥路径都必须填写")
		return
	}
	if !isManagedCatalogRelPath(certPath) || !isManagedCatalogRelPath(keyPath) {
		s.AjaxErr("只能绑定托管目录中的证书对")
		return
	}
	inspection, status, err := inspectConfiguredCertificateFiles(certPath, keyPath, "")
	if err != nil || !inspection.Valid {
		msg := status
		if err != nil {
			msg = firstCertLine(err.Error())
		}
		s.AjaxErr("证书不可用，无法绑定: " + msg)
		return
	}
	var certKey, keyKey, label, auditAction string
	switch target {
	case "panel", "web":
		certKey, keyKey, label, auditAction = "web_cert_file", "web_key_file", "面板", "certificate.catalog_bind_panel"
	case "bridge":
		certKey, keyKey, label, auditAction = "bridge_cert_file", "bridge_key_file", "Bridge", "certificate.catalog_bind_bridge"
	default:
		s.AjaxErr("target 必须是 panel 或 bridge")
		return
	}
	path := serverConfigPath()
	if _, err := os.Stat(path); err != nil {
		s.AjaxErr("读取 nps.conf 失败: " + err.Error())
		return
	}
	changed, writeErr := writeServerConfigUpdates(path, []confUpdate{
		{Key: certKey, Value: certPath},
		{Key: keyKey, Value: keyPath},
	})
	if writeErr != nil {
		s.auditPanel(auditAction, "nps.conf", 0, 0, "failure", writeErr.Error(), nil)
		s.AjaxErr("绑定失败: " + writeErr.Error())
		return
	}
	msg := label + " 证书已绑定，需要重启 NPS 后生效"
	if len(changed) == 0 {
		msg = label + " 证书绑定未发生变化"
	}
	s.auditPanel(auditAction, "nps.conf", 0, 0, "success", "", map[string]string{
		"cert": certPath, "key": keyPath,
	})
	s.Data["json"] = map[string]interface{}{
		"status":           1,
		"msg":              msg,
		"changed":          changed,
		"restart_required": len(changed) > 0,
	}
	s.ServeJSON()
	s.StopRun()
}
