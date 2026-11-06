package controllers

import (
	"sort"
	"strings"

	"ehang.io/nps/web/audit"
)

// AuditPanelController 面板审计日志可视化；仅管理员。
type AuditPanelController struct {
	BaseController
}

func (s *AuditPanelController) Prepare() {
	s.BaseController.Prepare()
	if isAdmin, ok := s.GetSession("isAdmin").(bool); !ok || !isAdmin {
		s.denyForbidden()
	}
}

func formatAuditDetails(details map[string]string) string {
	if len(details) == 0 {
		return ""
	}
	keys := make([]string, 0, len(details))
	for k := range details {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	preferred := []string{"addr", "transport", "version", "local_addr", "client_id"}
	ordered := make([]string, 0, len(keys))
	seen := map[string]struct{}{}
	for _, k := range preferred {
		if v, ok := details[k]; ok {
			ordered = append(ordered, k+"="+v)
			seen[k] = struct{}{}
		}
	}
	for _, k := range keys {
		if _, ok := seen[k]; ok {
			continue
		}
		ordered = append(ordered, k+"="+details[k])
	}
	return strings.Join(ordered, " ")
}

func (s *AuditPanelController) List() {
	if s.Ctx.Request.Method == "GET" {
		s.Data["menu"] = "audit"
		s.Data["audit_enabled"] = audit.Path() != ""
		s.Data["audit_path"] = audit.Path()
		s.Data["audit_connection"] = audit.ConnectionEnabled()
		s.SetInfo("audit log")
		s.display("audit/list")
		return
	}
	start, length := s.GetAjaxParams()
	page, err := audit.List(audit.Filter{
		Offset:       start,
		Limit:        length,
		Search:       s.getEscapeString("search"),
		Action:       s.getEscapeString("action"),
		ActorType:    s.getEscapeString("actor_type"),
		ResourceType: s.getEscapeString("resource_type"),
		Result:       s.getEscapeString("result"),
	})
	if err != nil {
		s.AjaxErr(err.Error())
		return
	}
	rows := make([]map[string]interface{}, 0, len(page.Events))
	for _, e := range page.Events {
		if e == nil {
			continue
		}
		details := e.Details
		if details == nil {
			details = map[string]string{}
		}
		rows = append(rows, map[string]interface{}{
			"Id": e.ID, "Time": e.Time.Local().Format("2006-01-02 15:04:05"),
			"ActorType": e.ActorType, "ActorName": e.ActorName, "SourceIP": e.SourceIP,
			"Action": e.Action, "ResourceType": e.ResourceType, "ResourceID": e.ResourceID,
			"OwnerUserID": e.OwnerUserID, "Result": e.Result, "Error": e.Error,
			"Method": e.Method, "Path": e.Path,
			"Details": details, "DetailText": formatAuditDetails(details),
			"ConnAddr": details["addr"], "Transport": details["transport"],
			"Version": details["version"], "LocalAddr": details["local_addr"],
		})
	}
	s.AjaxTable(rows, page.Total, page.Total, map[string]interface{}{
		"enabled":    audit.Path() != "",
		"connection": audit.ConnectionEnabled(),
	})
}
