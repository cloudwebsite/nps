package crypt

import (
	"crypto/tls"
	"net"
	"testing"
	"time"
)

func TestTlsFingerprintThreeStates(t *testing.T) {
	InitTls()
	fp := GetCertFingerprint()
	if fp == "" {
		t.Fatal("empty fingerprint")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		sc := NewTlsServerConn(c)
		_ = sc.SetDeadline(time.Now().Add(2 * time.Second))
		buf := make([]byte, 1)
		_, _ = sc.Read(buf)
	}()

	handshake := func(wantFP string, expectOK bool) {
		SetTlsFingerprint(wantFP)
		defer SetTlsFingerprint("")
		c, err := net.DialTimeout("tcp", ln.Addr().String(), 2*time.Second)
		if err != nil {
			t.Fatalf("dial: %v", err)
		}
		defer c.Close()
		tc := tls.Client(c, TlsDialConfig())
		_ = tc.SetDeadline(time.Now().Add(2 * time.Second))
		err = tc.Handshake()
		if expectOK && err != nil {
			t.Fatalf("expected ok, got %v", err)
		}
		if !expectOK && err == nil {
			t.Fatal("expected fingerprint mismatch")
		}
		if err == nil {
			_, _ = tc.Write([]byte{1})
		}
	}

	handshake("", true)
	handshake(fp, true)
	if len(fp) > 2 {
		handshake("00"+fp[2:], false)
	}
}
