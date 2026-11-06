package file

import (
	"encoding/json"
	"os"
	"path/filepath"

	"ehang.io/nps/lib/common"
	"github.com/astaxie/beego/logs"
)

// QuickCmdItem 终端快捷命令项。
type QuickCmdItem struct {
	Label string `json:"label"`
	Cmd   string `json:"cmd"`
}

// QuickCmds 面板持久化的快捷命令。
type QuickCmds struct {
	Customs []QuickCmdItem `json:"customs"`
	Hidden  []string       `json:"hidden"`
}

func (s *JsonDb) quickCmdsPath() string {
	if s.QuickCmdsFilePath != "" {
		return s.QuickCmdsFilePath
	}
	return filepath.Join(s.RunPath, "conf", "quick_cmds.json")
}

func (s *JsonDb) LoadQuickCmdsFromJsonFile() {
	s.quickMu.Lock()
	defer s.quickMu.Unlock()
	s.Quick = &QuickCmds{Customs: []QuickCmdItem{}, Hidden: []string{}}
	path := s.quickCmdsPath()
	if !common.FileExists(path) {
		return
	}
	b, err := common.ReadAllFromFile(path)
	if err != nil {
		logs.Warn("load quick_cmds: %v", err)
		return
	}
	var q QuickCmds
	if err := json.Unmarshal(b, &q); err != nil {
		logs.Warn("parse quick_cmds: %v", err)
		return
	}
	if q.Customs == nil {
		q.Customs = []QuickCmdItem{}
	}
	if q.Hidden == nil {
		q.Hidden = []string{}
	}
	s.Quick = &q
}

func (s *JsonDb) StoreQuickCmdsToJsonFile() {
	s.quickMu.Lock()
	q := s.Quick
	if q == nil {
		q = &QuickCmds{Customs: []QuickCmdItem{}, Hidden: []string{}}
	}
	b, err := json.MarshalIndent(q, "", "  ")
	s.quickMu.Unlock()
	if err != nil {
		logs.Warn("marshal quick_cmds: %v", err)
		return
	}
	path := s.quickCmdsPath()
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		logs.Warn("write quick_cmds tmp: %v", err)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(path)
		if err2 := os.Rename(tmp, path); err2 != nil {
			logs.Warn("rename quick_cmds: %v", err2)
		}
	}
}

// SaveQuickCmds 保存快捷命令（仅管理员应调用）。
func (s *DbUtils) SaveQuickCmds(q *QuickCmds) {
	if q == nil {
		q = &QuickCmds{Customs: []QuickCmdItem{}, Hidden: []string{}}
	}
	if q.Customs == nil {
		q.Customs = []QuickCmdItem{}
	}
	if q.Hidden == nil {
		q.Hidden = []string{}
	}
	s.JsonDb.quickMu.Lock()
	s.JsonDb.Quick = q
	s.JsonDb.quickMu.Unlock()
	s.JsonDb.StoreQuickCmdsToJsonFile()
}

func (s *DbUtils) GetQuickCmds() *QuickCmds {
	s.JsonDb.quickMu.Lock()
	defer s.JsonDb.quickMu.Unlock()
	if s.JsonDb.Quick == nil {
		s.JsonDb.Quick = &QuickCmds{Customs: []QuickCmdItem{}, Hidden: []string{}}
	}
	return s.JsonDb.Quick
}
