package common

import "testing"

func TestVerifyPasswordPlainAndBcrypt(t *testing.T) {
	if !VerifyPassword("secret", "secret") {
		t.Fatal("plain match")
	}
	if VerifyPassword("secret", "wrong") {
		t.Fatal("plain mismatch")
	}
	h, err := HashPassword("secret")
	if err != nil {
		t.Fatal(err)
	}
	if !IsBcryptHash(h) {
		t.Fatal("hash prefix")
	}
	if !VerifyPassword(h, "secret") {
		t.Fatal("bcrypt match")
	}
	if VerifyPassword(h, "wrong") {
		t.Fatal("bcrypt mismatch")
	}
}
