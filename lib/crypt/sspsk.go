package crypt

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/url"
	"strings"
)

// SSKeySize2022 是 2022-blake3-aes-128-gcm 要求的 PSK 字节数（SIP022）。
const SSKeySize2022 = 16

// NewShadowsocksPSK 生成 SS2022 随机 PSK（16 字节 → 标准 base64，通常 24 字符）。
func NewShadowsocksPSK() string {
	buf := make([]byte, SSKeySize2022)
	if _, err := rand.Read(buf); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return base64.StdEncoding.EncodeToString(buf)
}

func decodePSK16(s string) ([]byte, bool) {
	for _, enc := range []*base64.Encoding{
		base64.StdEncoding, base64.RawStdEncoding,
		base64.URLEncoding, base64.RawURLEncoding,
	} {
		if b, err := enc.DecodeString(s); err == nil && len(b) == SSKeySize2022 {
			return b, true
		}
	}
	return nil, false
}

// NormalizeShadowsocksPSK 规整面板「密码」为可用 SS2022 PSK。
// 空 → 随机生成；已是 16 字节 base64 → 归一标准编码；其它 → 口令派生（强度取决于口令）。
func NormalizeShadowsocksPSK(input string) string {
	s := strings.TrimSpace(input)
	if s == "" {
		return NewShadowsocksPSK()
	}
	if raw, ok := decodePSK16(s); ok {
		return base64.StdEncoding.EncodeToString(raw)
	}
	if unescaped, err := url.PathUnescape(s); err == nil && unescaped != s {
		if raw, ok := decodePSK16(unescaped); ok {
			return base64.StdEncoding.EncodeToString(raw)
		}
	}
	h := sha256.Sum256([]byte("nps/shadowsocks-2022/psk\x00" + s))
	return base64.StdEncoding.EncodeToString(h[:SSKeySize2022])
}
