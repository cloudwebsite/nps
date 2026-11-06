package common

import (
	"crypto/subtle"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword 生成面板密码的 bcrypt 哈希。
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	return string(b), err
}

// IsBcryptHash 判断存值是否已是 bcrypt。
func IsBcryptHash(stored string) bool {
	return strings.HasPrefix(stored, "$2")
}

// VerifyPassword 兼容 bcrypt 与存量明文：
//   - 存值为 $2 开头 → bcrypt 校验
//   - 否则按明文常量时间比较
func VerifyPassword(stored, plain string) bool {
	if stored == "" {
		return plain == ""
	}
	if IsBcryptHash(stored) {
		return bcrypt.CompareHashAndPassword([]byte(stored), []byte(plain)) == nil
	}
	u, p := []byte(stored), []byte(plain)
	if len(u) != len(p) {
		return false
	}
	return subtle.ConstantTimeCompare(u, p) == 1
}
