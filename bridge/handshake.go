package bridge

import (
	"errors"
	"fmt"
	"time"

	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/crypt"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/lib/version"
	"github.com/astaxie/beego"
)

// authenticateClient 按客户端产品版本与 bridge_secure_handshake 选择握手路径。
func (s *Bridge) authenticateClient(c *conn.Conn, clientVersion string) (int, error) {
	switch {
	case version.ServerSelectsSecureHandshake(clientVersion):
		return s.secureClientHandshake(c, clientVersion)
	default:
		// 未开开关、旧版本、无法解析版本：一律 legacy MD5(vkey)
		return s.legacyClientHandshake(c)
	}
}

func (s *Bridge) legacyClientHandshake(c *conn.Conn) (int, error) {
	if c == nil || c.Conn == nil {
		return 0, errors.New("invalid handshake connection")
	}
	if _, err := c.Write([]byte(crypt.Md5(version.GetVersion()))); err != nil {
		return 0, fmt.Errorf("write legacy server handshake: %w", err)
	}
	_ = c.SetReadDeadline(time.Now().Add(10 * time.Second))
	proof, err := c.GetShortContent(32)
	if err != nil {
		return 0, fmt.Errorf("read legacy client proof: %w", err)
	}
	id, err := file.GetDb().GetIdByVerifyKey(string(proof), c.Conn.RemoteAddr().String())
	if err != nil {
		s.verifyError(c)
		return 0, err
	}
	s.verifySuccess(c)
	return id, nil
}

func (s *Bridge) secureClientHandshake(c *conn.Conn, clientVersion string) (int, error) {
	if c == nil || c.Conn == nil {
		return 0, errors.New("invalid handshake connection")
	}
	db := file.GetDb()
	if db == nil || db.JsonDb == nil {
		return 0, errors.New("client database is unavailable")
	}
	if _, err := c.Write(crypt.SecureHandshakeMarker()); err != nil {
		return 0, fmt.Errorf("write secure handshake marker: %w", err)
	}
	keyID, err := c.GetShortLenContent()
	if err != nil {
		return 0, fmt.Errorf("read client key identifier: %w", err)
	}
	clientRecord, verifyKey, lookupErr := db.GetClientBySecureKeyID(keyID)
	challenge, err := crypt.NewSecureHandshakeChallenge()
	if err != nil {
		return 0, fmt.Errorf("create challenge: %w", err)
	}
	if err := c.WriteLenContent(challenge); err != nil {
		return 0, fmt.Errorf("send challenge: %w", err)
	}
	payload, err := c.GetShortLenContent()
	if err != nil {
		return 0, fmt.Errorf("read client proof: %w", err)
	}
	if lookupErr != nil || clientRecord == nil {
		return 0, errors.New("client authentication failed")
	}
	window := int64(beego.AppConfig.DefaultInt("auth_time_window", 300))
	if err := crypt.VerifySecureHandshakeClientPayload(
		verifyKey, version.GetVersion(), clientVersion, payload, challenge, time.Now().Unix(), window,
	); err != nil {
		return 0, err
	}
	clientRecord.Lock()
	if clientRecord.VerifyKey != verifyKey || !clientRecord.Status {
		clientID := clientRecord.Id
		clientRecord.Unlock()
		return clientID, errors.New("client credentials changed during handshake")
	}
	clientID := clientRecord.Id
	clientRecord.Addr = common.GetIpByAddr(c.Conn.RemoteAddr().String())
	clientRecord.Unlock()

	serverProof, err := crypt.SecureHandshakeServerProof(verifyKey, payload, challenge)
	if err != nil {
		return 0, err
	}
	s.verifySuccess(c)
	if _, err := c.Write(serverProof); err != nil {
		return 0, fmt.Errorf("write server proof: %w", err)
	}
	return clientID, nil
}
