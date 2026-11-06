package proxy

import (
	"testing"

	"ehang.io/nps/lib/file"
)

func TestHostProxyProtocolVersion(t *testing.T) {
	cases := []struct {
		name string
		host *file.Host
		want string
	}{
		{name: "nil", host: nil, want: ""},
		{name: "off", host: &file.Host{Target: &file.Target{ProxyProtocol: 0}}, want: ""},
		{name: "v1", host: &file.Host{Target: &file.Target{ProxyProtocol: 1}}, want: "V1"},
		{name: "v2", host: &file.Host{Target: &file.Target{ProxyProtocol: 2}}, want: "V2"},
		{name: "invalid", host: &file.Host{Target: &file.Target{ProxyProtocol: 3}}, want: ""},
	}
	for _, tc := range cases {
		if got := hostProxyProtocolVersion(tc.host); got != tc.want {
			t.Fatalf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}
