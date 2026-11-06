package install

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Loyalsoldier/v2ray-rules-dat 发布 geoip.dat / geosite.dat；摘要来自 GitHub Release asset.digest。
const geoRepo = "Loyalsoldier/v2ray-rules-dat"

// geodata.version 与 geoip.dat 同目录，记录上游 release 标签。
const geoVersionFile = "geodata.version"

const geoUpdaterUA = "nps-geodata-updater"

var geoHTTPClient = &http.Client{Timeout: 60 * time.Second}

var geoIPPath, geoSitePath string

// SetGeoDataPaths 记录本机 geodata 落盘路径（供面板更新与 -updategeo 使用）。
func SetGeoDataPaths(ipPath, sitePath string) {
	geoIPPath, geoSitePath = ipPath, sitePath
}

// GeoDataPaths 返回已配置路径；未配置时回落到运行目录 conf/。
func GeoDataPaths(runPath string) (string, string) {
	if geoIPPath != "" || geoSitePath != "" {
		return geoIPPath, geoSitePath
	}
	return filepath.Join(runPath, "conf", "geoip.dat"), filepath.Join(runPath, "conf", "geosite.dat")
}

// GeoFile 上游资源名与本地路径。
type GeoFile struct {
	Name string
	Path string
}

// GeoFiles 返回待维护的两个数据文件（路径可为空）。
func GeoFiles(ipPath, sitePath string) []GeoFile {
	out := make([]GeoFile, 0, 2)
	if strings.TrimSpace(ipPath) != "" {
		out = append(out, GeoFile{Name: "geoip.dat", Path: ipPath})
	}
	if strings.TrimSpace(sitePath) != "" {
		out = append(out, GeoFile{Name: "geosite.dat", Path: sitePath})
	}
	return out
}

// GeoUpdate 为 UpdateGeoData 结果。
type GeoUpdate struct {
	Version string
	Updated []string
}

// GeoDataVersion 读取 ipPath 旁的 geodata.version。
func GeoDataVersion(ipPath string) string {
	if strings.TrimSpace(ipPath) == "" {
		return ""
	}
	content, err := os.ReadFile(filepath.Join(filepath.Dir(ipPath), geoVersionFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(content))
}

func writeGeoDataVersion(ipPath, version string) error {
	if version == "" || strings.TrimSpace(ipPath) == "" {
		return nil
	}
	return os.WriteFile(filepath.Join(filepath.Dir(ipPath), geoVersionFile), []byte(version+"\n"), 0o644)
}

type geoRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name   string `json:"name"`
		Digest string `json:"digest"`
	} `json:"assets"`
}

func fetchGeoRelease() (*geoRelease, error) {
	req, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/"+geoRepo+"/releases/latest", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", geoUpdaterUA)
	resp, err := geoHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("查询 Geo 数据版本失败: HTTP %d", resp.StatusCode)
	}
	release := new(geoRelease)
	if err := json.NewDecoder(resp.Body).Decode(release); err != nil {
		return nil, err
	}
	if release.TagName == "" {
		return nil, errors.New("Geo 数据版本号为空")
	}
	return release, nil
}

func (r *geoRelease) digest(name string) (string, error) {
	for _, asset := range r.Assets {
		if asset.Name != name {
			continue
		}
		sum, ok := strings.CutPrefix(strings.TrimSpace(asset.Digest), "sha256:")
		raw, err := hex.DecodeString(sum)
		if !ok || err != nil || len(raw) != sha256.Size {
			return "", fmt.Errorf("release %s 为 %s 发布了无效摘要", r.TagName, name)
		}
		return strings.ToLower(sum), nil
	}
	return "", fmt.Errorf("release %s 未包含 %s", r.TagName, name)
}

// UpdateGeoData 下载最新 geoip.dat / geosite.dat，按 SHA-256 校验后原子替换。
func UpdateGeoData(ipPath, sitePath string) (*GeoUpdate, error) {
	files := GeoFiles(ipPath, sitePath)
	result := &GeoUpdate{}
	if len(files) == 0 {
		return result, errors.New("未配置 Geo 数据路径")
	}
	release, err := fetchGeoRelease()
	if err != nil {
		return result, err
	}
	result.Version = release.TagName
	for _, file := range files {
		digest, digestErr := release.digest(file.Name)
		if digestErr != nil {
			return result, digestErr
		}
		if sum, sumErr := fileSHA256(file.Path); sumErr == nil && sum == digest {
			fmt.Println(file.Name, "已是最新")
			continue
		}
		urls := []string{
			fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", geoRepo, release.TagName, file.Name),
			fmt.Sprintf("https://cdn.jsdelivr.net/gh/%s@release/%s", geoRepo, file.Name),
			fmt.Sprintf("https://fastly.jsdelivr.net/gh/%s@release/%s", geoRepo, file.Name),
		}
		if err := downloadGeoFile(urls, digest, file.Path); err != nil {
			return result, fmt.Errorf("更新 %s 失败: %w", file.Name, err)
		}
		fmt.Println("已更新", file.Path)
		result.Updated = append(result.Updated, file.Path)
	}
	if err := writeGeoDataVersion(ipPath, release.TagName); err != nil {
		return result, err
	}
	return result, nil
}

func downloadGeoFile(urls []string, digest, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}
	temp := destination + ".download"
	defer func() { _ = os.Remove(temp) }()
	var failures []error
	for _, url := range urls {
		if err := downloadAndVerifyGeoFile(url, digest, temp); err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", url, err))
			continue
		}
		if err := os.Rename(temp, destination); err != nil {
			return err
		}
		return nil
	}
	return errors.Join(failures...)
}

func downloadAndVerifyGeoFile(url, digest, destination string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", geoUpdaterUA)
	resp, err := geoHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	out, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	hasher := sha256.New()
	_, copyErr := io.Copy(io.MultiWriter(out, hasher), io.LimitReader(resp.Body, 512<<20))
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if actual := hex.EncodeToString(hasher.Sum(nil)); actual != digest {
		return fmt.Errorf("SHA-256 校验失败，期望 %s，实际 %s", digest, actual)
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
