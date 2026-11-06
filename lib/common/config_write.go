package common

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReplaceIniKeyValue 原子替换 ini 中已有 key 的值（用于明文密码→bcrypt 等）。
// 找不到 key 时返回错误；不新增键。
func ReplaceIniKeyValue(path, key, value string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	replaced := false
	prefix := key + "="
	for i, line := range lines {
		trimmed := strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if strings.HasPrefix(trimmed, prefix) {
			lines[i] = prefix + value
			replaced = true
			break
		}
	}
	if !replaced {
		return fmt.Errorf("key %s not found in %s", key, path)
	}
	return writeFileAtomically(path, []byte(strings.Join(lines, "\n")))
}

func writeFileAtomically(path string, data []byte) error {
	target := path
	linkInfo, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if linkInfo.Mode()&os.ModeSymlink != 0 {
		target, err = filepath.EvalSymlinks(path)
		if err != nil {
			return fmt.Errorf("resolve symlink: %w", err)
		}
	}
	info, err := os.Stat(target)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file: %s", target)
	}
	dir := filepath.Dir(target)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(target)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	_ = tmp.Chmod(info.Mode().Perm())
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, target)
}
