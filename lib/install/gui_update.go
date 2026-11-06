package install

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/version"
)

// guiAssetName 与 release.yml 产物一致：npc-gui-{os}-{arch}.zip
func guiAssetName() string {
	return fmt.Sprintf("npc-gui-%s-%s.zip", runtime.GOOS, runtime.GOARCH)
}

type guiReleaseMeta struct {
	TagName string `json:"tag_name"`
}

// updateNpcGUI 下载 GUI zip 并替换当前 npc-gui 可执行文件。
func updateNpcGUI() UpdateResult {
	res := UpdateResult{Channel: ChannelGUI}
	latest, err := fetchLatestVersion()
	if err != nil {
		res.Err = err
		res.Message = "获取最新版本失败：" + err.Error()
		return res
	}
	res.Latest = latest
	if compareVersion(version.VERSION, latest) >= 0 {
		res.Skipped = true
		res.Message = "当前已是最新版本"
		return res
	}
	tag := latest
	if !strings.HasPrefix(tag, "v") {
		// fetchLatestVersion 返回的是 tag_name，一般已带 v；兼容无前缀
		_ = tag
	}
	// 再拉一次 metadata 仅校验；下载 URL 一律走镜像前缀
	if _, err := fetchGUIReleaseTag(); err != nil {
		res.Err = err
		res.Message = "获取 GUI 发布信息失败：" + err.Error()
		return res
	}
	asset := guiAssetName()
	url := AssetDownloadURL(latest, asset)
	tempDir := filepath.Join(common.GetAppPath(), "temp", "npc-gui-update")
	_ = os.RemoveAll(tempDir)
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		res.Err = err
		res.Message = "创建临时目录失败：" + err.Error()
		return res
	}
	zipPath := filepath.Join(tempDir, asset)
	if err := downloadToFile(url, zipPath); err != nil {
		res.Err = err
		res.Message = "下载 GUI 更新包失败：" + err.Error()
		return res
	}
	exePath := common.GetExecutablePath()
	if err := replaceExecutableFromZip(zipPath, exePath, tempDir); err != nil {
		res.Err = err
		res.Message = "替换 GUI 失败：" + err.Error()
		return res
	}
	_ = os.RemoveAll(tempDir)
	res.Updated = true
	res.Message = "GUI 已更新到 " + latest
	return res
}

func fetchGUIReleaseTag() (string, error) {
	resp, err := metadataClient.Get(getReleaseAPIURL())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	var rl guiReleaseMeta
	if err := json.Unmarshal(b, &rl); err != nil {
		return "", err
	}
	if rl.TagName == "" {
		return "", fmt.Errorf("无法解析最新版本号")
	}
	return rl.TagName, nil
}

func downloadToFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d %s", resp.StatusCode, url)
	}
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, resp.Body)
	return err
}

func replaceExecutableFromZip(zipPath, exePath, tempDir string) error {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer zr.Close()
	want := filepath.Base(exePath)
	var src io.ReadCloser
	var found string
	for _, f := range zr.File {
		name := filepath.Base(f.Name)
		if strings.EqualFold(name, want) || strings.EqualFold(name, "npc-gui") || strings.EqualFold(name, "npc-gui.exe") {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			src = rc
			found = name
			break
		}
		// macOS .app 内可执行文件
		if strings.Contains(f.Name, ".app/Contents/MacOS/") && !f.FileInfo().IsDir() {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			src = rc
			found = filepath.Base(f.Name)
			break
		}
	}
	if src == nil {
		return fmt.Errorf("更新包中未找到可执行文件 %s", want)
	}
	defer src.Close()
	newBin := filepath.Join(tempDir, "new-"+found)
	out, err := os.Create(newBin)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		return err
	}
	out.Close()
	chMod(newBin, 0755)
	return replaceExecutable(newBin, exePath)
}
