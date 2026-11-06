package bridge

import (
	"fmt"
	"strings"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/lib/notify"
)

func emitHealthNotify(typ string, clientID int, target string) {
	if !notify.Enabled() {
		return
	}
	body := fmt.Sprintf("client_id=%d\ntarget=%s", clientID, target)
	notify.Emit(notify.Event{
		Type:     typ,
		ClientID: clientID,
		Target:   target,
		Body:     body,
	})
}

// GetHealthFromClient 接收客户端健康探测结果并维护 TargetArr。
func (s *Bridge) GetHealthFromClient(id int, c *conn.Conn) {
	for {
		if info, status, err := c.GetHealthInfo(); err != nil {
			break
		} else if !status { // status=false：目标不可达，从 TargetArr 摘除
			changed := false
			file.GetDb().JsonDb.Tasks.Range(func(key, value interface{}) bool {
				v := value.(*file.Tunnel)
				if v.Client.Id == id && v.Mode == "tcp" && strings.Contains(v.Target.TargetStr, info) {
					v.Lock()
					if !common.IsArrContains(v.HealthRemoveArr, info) {
						if v.Target.TargetArr == nil || (len(v.Target.TargetArr) == 0 && len(v.HealthRemoveArr) == 0) {
							v.Target.TargetArr = common.TrimArr(strings.Split(v.Target.TargetStr, "\n"))
						}
						v.Target.TargetArr = common.RemoveArrVal(v.Target.TargetArr, info)
						if v.HealthRemoveArr == nil {
							v.HealthRemoveArr = make([]string, 0)
						}
						v.HealthRemoveArr = append(v.HealthRemoveArr, info)
						changed = true
					}
					v.Unlock()
				}
				return true
			})
			file.GetDb().JsonDb.Hosts.Range(func(key, value interface{}) bool {
				v := value.(*file.Host)
				if v.Client.Id == id && strings.Contains(v.Target.TargetStr, info) {
					v.Lock()
					if !common.IsArrContains(v.HealthRemoveArr, info) {
						if v.Target.TargetArr == nil || (len(v.Target.TargetArr) == 0 && len(v.HealthRemoveArr) == 0) {
							v.Target.TargetArr = common.TrimArr(strings.Split(v.Target.TargetStr, "\n"))
						}
						v.Target.TargetArr = common.RemoveArrVal(v.Target.TargetArr, info)
						if v.HealthRemoveArr == nil {
							v.HealthRemoveArr = make([]string, 0)
						}
						v.HealthRemoveArr = append(v.HealthRemoveArr, info)
						changed = true
					}
					v.Unlock()
				}
				return true
			})
			if changed {
				emitHealthNotify(notify.TypeHealthDown, id, info)
			}
		} else { // status=true：目标恢复，加回 TargetArr
			changed := false
			file.GetDb().JsonDb.Tasks.Range(func(key, value interface{}) bool {
				v := value.(*file.Tunnel)
				if v.Client.Id == id && v.Mode == "tcp" && common.IsArrContains(v.HealthRemoveArr, info) && !common.IsArrContains(v.Target.TargetArr, info) {
					v.Lock()
					v.Target.TargetArr = append(v.Target.TargetArr, info)
					v.HealthRemoveArr = common.RemoveArrVal(v.HealthRemoveArr, info)
					v.Unlock()
					changed = true
				}
				return true
			})

			file.GetDb().JsonDb.Hosts.Range(func(key, value interface{}) bool {
				v := value.(*file.Host)
				if v.Client.Id == id && common.IsArrContains(v.HealthRemoveArr, info) && !common.IsArrContains(v.Target.TargetArr, info) {
					v.Lock()
					v.Target.TargetArr = append(v.Target.TargetArr, info)
					v.HealthRemoveArr = common.RemoveArrVal(v.HealthRemoveArr, info)
					v.Unlock()
					changed = true
				}
				return true
			})
			if changed {
				emitHealthNotify(notify.TypeHealthUp, id, info)
			}
		}
	}
	s.DelClient(id)
}
