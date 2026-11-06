package controllers

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestValidUserAPIToken(t *testing.T) {
	token := userAPITokenPrefix + "abc"
	sum := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(sum[:])
	if !validUserAPIToken(hash, token) {
		t.Fatal("expected valid")
	}
	if validUserAPIToken(hash, "npsu_other") {
		t.Fatal("tampered")
	}
	if validUserAPIToken("", token) {
		t.Fatal("empty hash")
	}
}

func TestExtractBearerOrToken(t *testing.T) {
	if got := extractBearerOrToken("Bearer npsu_x", ""); got != "npsu_x" {
		t.Fatalf("got %q", got)
	}
	if got := extractBearerOrToken("", "npsu_q"); got != "npsu_q" {
		t.Fatalf("got %q", got)
	}
}

func TestGenerateUserAPIToken(t *testing.T) {
	token, hash, err := generateUserAPIToken()
	if err != nil || !validUserAPIToken(hash, token) {
		t.Fatalf("token=%q hash=%q err=%v", token, hash, err)
	}
}
