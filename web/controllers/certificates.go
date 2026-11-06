package controllers

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ehang.io/nps/lib/common"
)

// certificateInspection 校验结果（不含私钥与服务器绝对路径）。
type certificateInspection struct {
	Valid              bool     `json:"valid"`
	Message            string   `json:"message"`
	Subject            string   `json:"subject"`
	Issuer             string   `json:"issuer"`
	NotBefore          string   `json:"not_before"`
	ExpiresAt          string   `json:"expires_at"`
	DaysRemaining      int      `json:"days_remaining"`
	HostnameOK         bool     `json:"hostname_ok"`
	SANs               []string `json:"sans"`
	Fingerprint        string   `json:"fingerprint"`
	PublicKeyAlgorithm string   `json:"public_key_algorithm"`
	ChainLength        int      `json:"chain_length"`
	SelfSigned         bool     `json:"self_signed"`
	Expired            bool     `json:"expired"`
	NotYetValid        bool     `json:"not_yet_valid"`
}

func managedCertificateDirectory(scope string) (string, string) {
	directory := "certs"
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "bridge":
		directory = "bridgessl"
	case "web":
		directory = "webssl"
	}
	return filepath.Join(common.GetRunPath(), "conf", directory), directory
}

func sanitizeCertificateToken(token string) string {
	token = strings.TrimSpace(token)
	var builder strings.Builder
	for _, r := range token {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			builder.WriteRune(r)
		}
		if builder.Len() >= 64 {
			break
		}
	}
	return builder.String()
}

func randomCertificateToken() string {
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buffer)
}

func firstCertLine(errText string) string {
	errText = strings.TrimSpace(errText)
	if i := strings.IndexByte(errText, '\n'); i >= 0 {
		return strings.TrimSpace(errText[:i])
	}
	return errText
}

func writeCertificateFileAtomic(path string, data []byte, mode os.FileMode) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return fmt.Errorf("创建证书目录失败: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".cert-*.tmp")
	if err != nil {
		return fmt.Errorf("写入证书失败: %w", err)
	}
	temporaryName := temporary.Name()
	defer func() { _ = os.Remove(temporaryName) }()
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("写入证书失败: %w", err)
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("写入证书失败: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("写入证书失败: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("写入证书失败: %w", err)
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return fmt.Errorf("写入证书失败: %w", err)
	}
	return nil
}

func decodeCertificateData(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if strings.Contains(raw, "-----BEGIN") {
		return []byte(raw), nil
	}
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		data, err = base64.RawStdEncoding.DecodeString(raw)
	}
	if err != nil {
		return nil, errors.New("证书内容不是有效的 Base64 编码")
	}
	return data, nil
}

func inspectCertificatePair(certPEM, keyPEM []byte, hostname string, wildcard bool) (certificateInspection, error) {
	result := certificateInspection{}
	if len(strings.TrimSpace(string(certPEM))) == 0 || len(strings.TrimSpace(string(keyPEM))) == 0 {
		result.Message = "证书与私钥都必须填写"
		return result, errors.New(result.Message)
	}
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		result.Message = firstCertLine(err.Error())
		return result, fmt.Errorf("证书与私钥不匹配或格式错误: %s", result.Message)
	}
	result, certificate, err := inspectCertificateChain(pair.Certificate)
	if err != nil {
		return result, err
	}
	now := time.Now()
	result.NotYetValid = now.Before(certificate.NotBefore)
	result.Expired = now.After(certificate.NotAfter)
	if result.NotYetValid || result.Expired {
		result.Message = "证书当前未生效或已过期"
		return result, errors.New(result.Message)
	}
	probeHost := strings.TrimSpace(hostname)
	if wildcard {
		probeHost = "probe." + strings.TrimPrefix(probeHost, "*.")
	}
	if probeHost != "" {
		if err := certificate.VerifyHostname(probeHost); err != nil {
			result.Message = "证书不覆盖域名 " + probeHost
			return result, errors.New(result.Message)
		}
		result.HostnameOK = true
	}
	result.Valid = true
	result.Message = "证书有效"
	return result, nil
}

func inspectCertificateChain(chain [][]byte) (certificateInspection, *x509.Certificate, error) {
	result := certificateInspection{SANs: []string{}}
	if len(chain) == 0 {
		result.Message = "证书内容为空"
		return result, nil, errors.New(result.Message)
	}
	certificate, err := x509.ParseCertificate(chain[0])
	if err != nil {
		result.Message = firstCertLine(err.Error())
		return result, nil, fmt.Errorf("无法解析证书: %s", result.Message)
	}
	result.Subject = certificate.Subject.String()
	result.Issuer = certificate.Issuer.String()
	result.NotBefore = certificate.NotBefore.Local().Format("2006-01-02 15:04:05")
	result.ExpiresAt = certificate.NotAfter.Local().Format("2006-01-02 15:04:05")
	result.DaysRemaining = int(time.Until(certificate.NotAfter).Hours() / 24)
	result.SANs = append([]string(nil), certificate.DNSNames...)
	for _, address := range certificate.IPAddresses {
		result.SANs = append(result.SANs, address.String())
	}
	digest := sha256.Sum256(certificate.Raw)
	result.Fingerprint = hex.EncodeToString(digest[:])
	result.PublicKeyAlgorithm = certificate.PublicKeyAlgorithm.String()
	result.ChainLength = len(chain)
	result.SelfSigned = len(certificate.RawIssuer) > 0 &&
		len(certificate.RawSubject) > 0 &&
		string(certificate.RawIssuer) == string(certificate.RawSubject) &&
		certificate.CheckSignature(certificate.SignatureAlgorithm, certificate.RawTBSCertificate, certificate.Signature) == nil
	return result, certificate, nil
}

func certificateMaterialFromRequest(request *http.Request, mode string) ([]byte, []byte, error) {
	if request == nil {
		return nil, nil, errors.New("请求无效")
	}
	if err := request.ParseForm(); err != nil {
		return nil, nil, fmt.Errorf("读取证书请求失败: %w", err)
	}
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "path":
		certPath := strings.TrimSpace(request.FormValue("cert"))
		keyPath := strings.TrimSpace(request.FormValue("key"))
		if certPath == "" || keyPath == "" {
			return nil, nil, errors.New("证书路径与私钥路径都必须填写")
		}
		certPEM, certErr := common.ReadAllFromFile(common.ResolveConfigPath(certPath))
		if certErr != nil {
			return nil, nil, fmt.Errorf("读取证书文件失败: %s", firstCertLine(certErr.Error()))
		}
		keyPEM, keyErr := common.ReadAllFromFile(common.ResolveConfigPath(keyPath))
		if keyErr != nil {
			return nil, nil, fmt.Errorf("读取私钥文件失败: %s", firstCertLine(keyErr.Error()))
		}
		return certPEM, keyPEM, nil
	case "upload", "paste":
		certPEM, certErr := decodeCertificateData(request.FormValue("cert"))
		if certErr != nil {
			return nil, nil, certErr
		}
		keyPEM, keyErr := decodeCertificateData(request.FormValue("key"))
		if keyErr != nil {
			return nil, nil, keyErr
		}
		return certPEM, keyPEM, nil
	default:
		return nil, nil, errors.New("证书模式必须是 path、upload 或 paste")
	}
}

// persistCertificatePair 校验后原子写入托管目录；返回相对运行目录的路径。
func persistCertificatePair(scope, id string, certPEM, keyPEM []byte) (string, string, error) {
	if _, err := inspectCertificatePair(certPEM, keyPEM, "", false); err != nil {
		return "", "", err
	}
	token := sanitizeCertificateToken(id)
	if token == "" {
		token = randomCertificateToken()
	}
	baseName := scope + "_" + token
	directory, relativeDirectory := managedCertificateDirectory(scope)
	certPath := filepath.Join(directory, baseName+".pem")
	keyPath := filepath.Join(directory, baseName+".key")
	if err := writeCertificateFileAtomic(certPath, certPEM, 0644); err != nil {
		return "", "", err
	}
	if err := writeCertificateFileAtomic(keyPath, keyPEM, 0600); err != nil {
		_ = os.Remove(certPath)
		return "", "", err
	}
	return filepath.ToSlash(filepath.Join("conf", relativeDirectory, baseName+".pem")),
		filepath.ToSlash(filepath.Join("conf", relativeDirectory, baseName+".key")), nil
}

func inspectConfiguredCertificateFiles(certPath, keyPath, hostname string) (certificateInspection, string, error) {
	certPath, keyPath = strings.TrimSpace(certPath), strings.TrimSpace(keyPath)
	if certPath == "" && keyPath == "" {
		return certificateInspection{Message: "未配置证书"}, "未配置证书", nil
	}
	if certPath == "" || keyPath == "" {
		return certificateInspection{Message: "证书路径和私钥路径必须同时填写"}, "证书配置不完整", errors.New("证书路径和私钥路径必须同时填写")
	}
	certPEM, err := os.ReadFile(common.ResolveConfigPath(certPath))
	if err != nil {
		inspection := certificateInspection{Message: firstCertLine(err.Error())}
		return inspection, "证书文件不可读", err
	}
	keyPEM, err := os.ReadFile(common.ResolveConfigPath(keyPath))
	if err != nil {
		inspection := certificateInspection{Message: firstCertLine(err.Error())}
		return inspection, "私钥文件不可读", err
	}
	inspection, err := inspectCertificatePair(certPEM, keyPEM, hostname, false)
	if err != nil {
		switch {
		case inspection.Expired:
			return inspection, "证书已过期", err
		case inspection.NotYetValid:
			return inspection, "证书尚未生效", err
		case hostname != "":
			return inspection, "证书不覆盖指定域名", err
		default:
			return inspection, "证书不可用", err
		}
	}
	if inspection.DaysRemaining < 15 {
		return inspection, "证书即将到期", nil
	}
	return inspection, "证书有效", nil
}
