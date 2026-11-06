package install

import (
	"strings"
	"sync"
)

const (
	defaultReleaseAPIURL       = "https://api.github.com/repos/cloudwebsite/nps/releases/latest"
	defaultReleaseDownloadBase = "https://github.com/cloudwebsite/nps/releases/download"
)

var (
	releaseMu           sync.RWMutex
	releaseAPIURL       = defaultReleaseAPIURL
	releaseDownloadBase = defaultReleaseDownloadBase
)

// SetReleaseSource 设置安装/升级下载源。
// apiURL 空=GitHub releases/latest；downloadBase 空=GitHub download 前缀。
// 未配置时行为与现网一致。仅非空参数会覆盖。
func SetReleaseSource(apiURL, downloadBase string) {
	releaseMu.Lock()
	defer releaseMu.Unlock()
	if v := strings.TrimSpace(apiURL); v != "" {
		releaseAPIURL = strings.TrimRight(v, "/")
	}
	if v := strings.TrimSpace(downloadBase); v != "" {
		releaseDownloadBase = strings.TrimRight(v, "/")
	}
}

// ResetReleaseSource 测试用：恢复默认 GitHub 源。
func ResetReleaseSource() {
	releaseMu.Lock()
	defer releaseMu.Unlock()
	releaseAPIURL = defaultReleaseAPIURL
	releaseDownloadBase = defaultReleaseDownloadBase
}

func getReleaseAPIURL() string {
	releaseMu.RLock()
	defer releaseMu.RUnlock()
	return releaseAPIURL
}

func getReleaseDownloadBase() string {
	releaseMu.RLock()
	defer releaseMu.RUnlock()
	return releaseDownloadBase
}

// GetReleaseAPIURL 供 GUI 等包读取当前生效的 Release API。
func GetReleaseAPIURL() string { return getReleaseAPIURL() }

// GetReleaseDownloadBase 供 GUI 等包读取当前生效的下载前缀。
func GetReleaseDownloadBase() string { return getReleaseDownloadBase() }

// ReleaseSourceSnapshot 供面板/诊断展示。
func ReleaseSourceSnapshot() (apiURL, downloadBase string) {
	releaseMu.RLock()
	defer releaseMu.RUnlock()
	return releaseAPIURL, releaseDownloadBase
}

// AssetDownloadURL 按当前 download_base 拼资源地址（镜像优先，不依赖 GitHub browser_download_url）。
func AssetDownloadURL(tag, assetName string) string {
	tag = strings.TrimSpace(tag)
	assetName = strings.TrimSpace(assetName)
	return strings.TrimRight(getReleaseDownloadBase(), "/") + "/" + tag + "/" + assetName
}
