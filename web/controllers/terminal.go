package controllers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/server"
	"github.com/astaxie/beego/logs"
	"github.com/gorilla/websocket"
)

var shellSessions sync.Map

const (
	wsPingPeriod = 30 * time.Second
	wsPongWait   = 90 * time.Second
	wsWriteWait  = 10 * time.Second
	wsMaxMessage = 1 << 20
	wsMaxCols    = 500
	wsMaxRows    = 200
)

func clampPTY(v, def, max int) int {
	if v <= 0 {
		return def
	}
	if v > max {
		return max
	}
	return v
}

func webTerminalEnabled() bool {
	return file.EffectiveAllowWebTerminal()
}

type TerminalController struct {
	BaseController
}

type wsBinaryWriter struct{ c *websocket.Conn }

func (w wsBinaryWriter) Write(p []byte) (int, error) {
	if err := w.c.WriteMessage(websocket.BinaryMessage, p); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (s *TerminalController) Prepare() {
	s.BaseController.Prepare()
	if webTerminalEnabled() {
		return
	}
	// 浏览页展示说明；接口与 WebSocket 仍拒绝
	if s.actionName == "index" {
		return
	}
	s.Ctx.Output.SetStatus(403)
	s.Data["json"] = map[string]interface{}{"code": 0, "msg": "web terminal disabled"}
	s.ServeJSON()
	s.StopRun()
}

func (s *TerminalController) Index() {
	s.Ctx.Output.Header("Cache-Control", "no-store, no-cache, must-revalidate")
	s.Data["menu"] = "terminal"
	s.Data["client_id"] = s.GetIntNoErr("client_id")
	s.SetInfo("terminal")
	if !webTerminalEnabled() {
		s.display("terminal/disabled")
		return
	}
	s.display("terminal/index")
}

// GetCmds 返回快捷命令。
func (s *TerminalController) GetCmds() {
	s.Ctx.Output.Header("Cache-Control", "no-store")
	s.Data["json"] = file.GetDb().GetQuickCmds()
	s.ServeJSON()
}

// SaveCmds 保存快捷命令（仅管理员）。
func (s *TerminalController) SaveCmds() {
	if admin, _ := s.GetSession("isAdmin").(bool); !admin {
		s.Ctx.Output.SetStatus(403)
		s.Data["json"] = map[string]interface{}{"code": 0, "msg": "forbidden"}
		s.ServeJSON()
		return
	}
	var q file.QuickCmds
	b, err := io.ReadAll(io.LimitReader(s.Ctx.Request.Body, 64<<10))
	if err != nil || len(b) == 0 {
		s.Data["json"] = map[string]interface{}{"code": 0, "msg": "empty body"}
		s.ServeJSON()
		return
	}
	if err := json.Unmarshal(b, &q); err != nil {
		s.Data["json"] = map[string]interface{}{"code": 0, "msg": "bad request"}
		s.ServeJSON()
		return
	}
	if q.Customs == nil {
		q.Customs = []file.QuickCmdItem{}
	}
	if q.Hidden == nil {
		q.Hidden = []string{}
	}
	file.GetDb().SaveQuickCmds(&q)
	s.Data["json"] = map[string]interface{}{"code": 1}
	s.ServeJSON()
}

func (s *TerminalController) GetClients() {
	s.Ctx.Output.Header("Cache-Control", "no-store")
	list, _ := server.GetClientList(0, 1000, "", "", "id", 0)
	type item struct {
		Id        int    `json:"id"`
		Remark    string `json:"remark"`
		IsConnect bool   `json:"is_connect"`
	}
	items := make([]item, 0, len(list))
	for _, c := range list {
		if c == nil || !c.Status {
			continue
		}
		if !s.canOperateTerminalClient(c.Id) {
			continue
		}
		items = append(items, item{Id: c.Id, Remark: c.Remark, IsConnect: c.IsConnect})
	}
	s.Data["json"] = map[string]interface{}{"code": 1, "data": items}
	s.ServeJSON()
}

func (s *TerminalController) canOperateTerminalClient(cid int) bool {
	if admin, ok := s.GetSession("isAdmin").(bool); ok && admin {
		return true
	}
	mine, ok := s.GetSession("clientId").(int)
	return ok && mine == cid
}

func (s *TerminalController) checkWsOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	switch u.Hostname() {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	return u.Host == r.Host
}

func (s *TerminalController) Ws() {
	s.EnableRender = false
	clientId, _ := strconv.Atoi(s.Ctx.Request.URL.Query().Get("client_id"))
	cols := clampPTY(s.GetIntNoErr("cols"), 80, wsMaxCols)
	rows := clampPTY(s.GetIntNoErr("rows"), 24, wsMaxRows)
	if clientId <= 0 || !s.canOperateTerminalClient(clientId) {
		s.Ctx.Output.SetStatus(403)
		return
	}
	up := websocket.Upgrader{CheckOrigin: s.checkWsOrigin}
	ws, err := up.Upgrade(s.Ctx.ResponseWriter, s.Ctx.Request, nil)
	if err != nil {
		s.Ctx.Output.SetStatus(400)
		return
	}
	defer ws.Close()

	ws.SetReadLimit(wsMaxMessage)
	_ = ws.SetReadDeadline(time.Now().Add(wsPongWait))
	ws.SetPongHandler(func(string) error {
		return ws.SetReadDeadline(time.Now().Add(wsPongWait))
	})
	pingDone := make(chan struct{})
	defer close(pingDone)
	go func() {
		tk := time.NewTicker(wsPingPeriod)
		defer tk.Stop()
		for {
			select {
			case <-pingDone:
				return
			case <-tk.C:
				if err := ws.WriteControl(websocket.PingMessage, nil, time.Now().Add(wsWriteWait)); err != nil {
					_ = ws.Close()
					return
				}
			}
		}
	}()

	client, err := file.GetDb().GetClient(clientId)
	if err != nil || client == nil {
		_ = ws.WriteMessage(websocket.TextMessage, []byte("client not found"))
		return
	}
	shellID := fmt.Sprintf("%d-%d", clientId, time.Now().UnixNano())
	shellSessions.Store(shellID, clientId)
	defer shellSessions.Delete(shellID)
	link := conn.NewLink("shell", "", client.Cnf.Crypt, client.Cnf.Compress, "", false, "")
	link.Cols = cols
	link.Rows = rows
	link.ShellID = shellID
	t, err := server.Bridge.SendLinkInfo(clientId, link, nil)
	if err != nil {
		_ = ws.WriteMessage(websocket.TextMessage, []byte("tunnel error: "+err.Error()))
		return
	}
	defer t.Close()

	username := "admin"
	if u := s.GetSession("username"); u != nil {
		username = fmt.Sprint(u)
	}
	logs.Info("TERMINAL AUDIT: user [%s] opened shell on client id [%d] remark [%s]", username, clientId, client.Remark)

	go func() {
		n, _ := io.Copy(wsBinaryWriter{ws}, t)
		if n > 0 && client.Flow != nil {
			client.Flow.Add(0, n)
		}
		_ = ws.Close()
	}()

	for {
		mt, data, err := ws.ReadMessage()
		if err != nil {
			break
		}
		_ = ws.SetReadDeadline(time.Now().Add(wsPongWait))
		if mt == websocket.BinaryMessage {
			n, werr := t.Write(data)
			if n > 0 && client.Flow != nil {
				client.Flow.Add(int64(n), 0)
			}
			if werr != nil {
				break
			}
		} else if mt == websocket.TextMessage {
			var r struct {
				Type string `json:"type"`
				Cols int    `json:"cols"`
				Rows int    `json:"rows"`
			}
			if json.Unmarshal(data, &r) == nil && r.Type == "resize" && r.Cols > 0 && r.Rows > 0 {
				if cid, ok := shellSessions.Load(shellID); ok {
					_ = server.Bridge.SendShellResize(cid.(int), shellID,
						clampPTY(r.Cols, 80, wsMaxCols), clampPTY(r.Rows, 24, wsMaxRows))
				}
			}
		}
	}
}
