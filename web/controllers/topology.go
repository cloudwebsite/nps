package controllers

import (
	"ehang.io/nps/lib/file"
	"ehang.io/nps/server"
)

// TopologyController 精简拓扑页。
type TopologyController struct {
	BaseController
}

func (s *TopologyController) Index() {
	// 方法 Data() 会遮蔽嵌入的 Controller.Data 字段，须经 Controller 访问。
	s.Controller.Data["menu"] = "topology"
	s.SetInfo("topology")
	s.display("topology/index")
}

// Data JSON 快照；管理员看全量，普通用户仅本客户端。
// 路由仍为 /topology/data；实现里勿写 s.Data，否则解析为方法自身。
func (s *TopologyController) Data() {
	isAdmin := s.Controller.Data["isAdmin"] == true
	var allow map[int]struct{}
	if !isAdmin {
		allow = map[int]struct{}{}
		if id, ok := s.Ctx.Input.GetData("client_id").(int); ok && id > 0 {
			allow[id] = struct{}{}
		}
		// 多用户面板：按 UserId 归属的客户端
		if uid, ok := s.GetSession("userId").(int); ok && uid > 0 {
			file.GetDb().JsonDb.Clients.Range(func(_, value interface{}) bool {
				c, ok := value.(*file.Client)
				if ok && c != nil && c.UserId == uid {
					allow[c.Id] = struct{}{}
				}
				return true
			})
		}
	}
	s.Controller.Data["json"] = server.GetTopologyData(allow, isAdmin)
	s.ServeJSON()
}
