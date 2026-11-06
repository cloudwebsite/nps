package notify

import "time"

// Event 运维通知事件。
type Event struct {
	Type     string            `json:"type"`
	Title    string            `json:"title"`
	Body     string            `json:"body"`
	Time     time.Time         `json:"time"`
	ClientID int               `json:"client_id"`
	Host     string            `json:"host"`
	Target   string            `json:"target"`
	Addr     string            `json:"addr"`
	Details  map[string]string `json:"details,omitempty"`
}

// 稳定事件类型（与方案 §5.2 对齐）。
const (
	TypeClientConnect    = "client.connect"
	TypeClientDisconnect = "client.disconnect"
	TypeHealthDown       = "health.down"
	TypeHealthUp         = "health.up"
	TypeErrorPage        = "error_page"
)
