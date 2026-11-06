package web

import (
	"strings"
	"testing"
)

func TestNormalizeWebSecureEntry(t *testing.T) {
	cases := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"", "", false},
		{"  ", "", false},
		{"/s/demoToken", "/s/demoToken", false},
		{"/s/demoToken/", "/s/demoToken", false},
		{"/", "", true},
		{"s/no-slash", "", true},
		{"/s/../admin", "", true},
		{"/" + strings.Repeat("a", 70), "", true},
	}
	for _, tc := range cases {
		got, err := NormalizeWebSecureEntry(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("NormalizeWebSecureEntry(%q) expected error", tc.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("NormalizeWebSecureEntry(%q) unexpected err: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("NormalizeWebSecureEntry(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
}
