package common

import (
	"path/filepath"
	"testing"
)

func TestResolveConfigPath(t *testing.T) {
	original := ConfPath
	ConfPath = filepath.Join(t.TempDir(), "nps-root")
	t.Cleanup(func() { ConfPath = original })

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{name: "relative", input: "conf/webssl/fullchain.pem", want: filepath.Join(ConfPath, "conf/webssl/fullchain.pem")},
		{name: "trimmed", input: "  conf/a.pem  ", want: filepath.Join(ConfPath, "conf/a.pem")},
		{name: "inline pem", input: "-----BEGIN CERTIFICATE-----\nA\n-----END CERTIFICATE-----", want: "-----BEGIN CERTIFICATE-----\nA\n-----END CERTIFICATE-----"},
		{name: "empty", input: "", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ResolveConfigPath(tc.input); got != tc.want {
				t.Fatalf("ResolveConfigPath(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
