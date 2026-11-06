package controllers

import "testing"

func TestParseHostProxyProtocolValue(t *testing.T) {
	cases := []struct {
		value     string
		current   int
		submitted bool
		want      int
		wantErr   bool
	}{
		{value: "", current: 2, submitted: false, want: 2},
		{value: "", current: 1, submitted: true, want: 1},
		{value: "0", current: 1, submitted: true, want: 0},
		{value: "v1", current: 0, submitted: true, want: 1},
		{value: "2", current: 0, submitted: true, want: 2},
		{value: "9", current: 0, submitted: true, wantErr: true},
	}
	for _, tc := range cases {
		got, err := parseHostProxyProtocolValue(tc.value, tc.current, tc.submitted)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("value=%q expected error", tc.value)
			}
			continue
		}
		if err != nil {
			t.Fatalf("value=%q unexpected err: %v", tc.value, err)
		}
		if got != tc.want {
			t.Fatalf("value=%q got %d want %d", tc.value, got, tc.want)
		}
	}
}
