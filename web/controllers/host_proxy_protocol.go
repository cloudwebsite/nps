package controllers

import (
	"fmt"
	"strconv"
	"strings"
)

// parseHostProxyProtocolValue 解析域名 Proxy Protocol：0=关，1=v1，2=v2。
// submitted=false 时保留 current（编辑页未改字段）。
func parseHostProxyProtocolValue(value string, current int, submitted bool) (int, error) {
	value = strings.TrimSpace(value)
	if !submitted || value == "" {
		if current < 0 || current > 2 {
			return 0, nil
		}
		return current, nil
	}
	switch strings.ToLower(value) {
	case "0", "off", "false", "no", "none":
		return 0, nil
	case "1", "v1":
		return 1, nil
	case "2", "v2":
		return 2, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 || n > 2 {
		return 0, fmt.Errorf("无效的 proxy_protocol=%q（允许 0/1/2）", value)
	}
	return n, nil
}

func (s *IndexController) parseHostProxyProtocol(current int) (int, error) {
	value := strings.TrimSpace(s.GetString("proxy_protocol"))
	if value == "" {
		return parseHostProxyProtocolValue("", current, false)
	}
	return parseHostProxyProtocolValue(value, current, true)
}
