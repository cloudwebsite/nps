package acl

import "testing"

func TestMatchIPExactAndCIDR(t *testing.T) {
	if MatchIP("1.2.3.4", nil) || MatchIP("1.2.3.4", []string{}) {
		t.Fatal("empty must not match")
	}
	if !MatchIP("1.2.3.4", []string{"1.2.3.4"}) {
		t.Fatal("exact")
	}
	if !MatchIP("10.1.2.3", []string{"10.0.0.0/8"}) {
		t.Fatal("cidr")
	}
	if MatchIP("11.0.0.1", []string{"10.0.0.0/8", "# comment"}) {
		t.Fatal("outside cidr")
	}
	if !HasActiveRules([]string{"", "#x", "1.1.1.1"}) {
		t.Fatal("active")
	}
	if HasActiveRules([]string{"", "# only"}) {
		t.Fatal("comments only")
	}
}

func TestParseIPFromAddr(t *testing.T) {
	if got := ParseIPFromAddr("1.2.3.4:80"); got != "1.2.3.4" {
		t.Fatalf("got %q", got)
	}
}
