package controllers

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"ehang.io/nps/lib/crypt"
	"ehang.io/nps/lib/file"
	"ehang.io/nps/lib/version"
	"ehang.io/nps/web/audit"
	"github.com/astaxie/beego"
	"github.com/google/uuid"
)

const (
	userAPITokenPrefix = "npsu_"
	apiV1DefaultLimit  = 20
	apiV1MaxLimit      = 200
)

// ApiV1Controller 精简版 /api/v1。
// 凭据：管理员 auth_key+timestamp（±20s）或用户 Bearer npsu_…；不接受浏览器 Session。
type ApiV1Controller struct {
	beego.Controller
	requestID         string
	apiAuthorized     bool // 管理员签名
	userAPIAuthorized bool
	apiUserID         int
}

func (s *ApiV1Controller) Prepare() {
	s.requestID = uuid.NewString()
	if s.Ctx != nil && s.Ctx.ResponseWriter != nil {
		s.Ctx.ResponseWriter.Header().Set("X-Request-ID", s.requestID)
		s.Ctx.ResponseWriter.Header().Set("Cache-Control", "no-store")
	}

	md5Key := strings.TrimSpace(s.GetString("auth_key"))
	timestamp := s.GetIntNoErr("timestamp")
	configKey := strings.TrimSpace(beego.AppConfig.String("auth_key"))
	timeNowUnix := time.Now().Unix()
	if configKey != "" && md5Key != "" && math.Abs(float64(timeNowUnix-int64(timestamp))) <= 20 {
		expected := crypt.Md5(configKey + strconv.Itoa(timestamp))
		s.apiAuthorized = subtle.ConstantTimeCompare([]byte(expected), []byte(md5Key)) == 1
	}

	if !s.apiAuthorized {
		token := extractBearerOrToken(s.Ctx.Request.Header.Get("Authorization"), s.GetString("api_token"))
		if id, ok := lookupUserByAPIToken(token); ok {
			s.userAPIAuthorized = true
			s.apiUserID = id
		}
	}

	// Info 放行；其余需凭据
	_, action := s.GetControllerAndAction()
	if strings.EqualFold(action, "Info") {
		return
	}
	if !s.apiAuthorized && !s.userAPIAuthorized {
		s.apiFail(http.StatusUnauthorized, "unauthorized", "缺少或无效的 API 凭据（auth_key+timestamp 或 Bearer token）")
		return
	}
}

func (s *ApiV1Controller) GetIntNoErr(key string, def ...int) int {
	v, err := s.GetInt(key)
	if err != nil {
		if len(def) > 0 {
			return def[0]
		}
		return 0
	}
	return v
}

func (s *ApiV1Controller) isAdmin() bool {
	return s.apiAuthorized
}

func (s *ApiV1Controller) apiOK(data interface{}) {
	s.Ctx.Output.SetStatus(http.StatusOK)
	s.Data["json"] = map[string]interface{}{
		"status": 1, "code": "ok", "msg": "ok", "request_id": s.requestID, "data": data,
	}
	s.ServeJSON()
	s.StopRun()
}

func (s *ApiV1Controller) apiFail(status int, code, msg string) {
	s.Ctx.Output.SetStatus(status)
	s.Data["json"] = map[string]interface{}{
		"status": 0, "code": "error", "error_code": code, "msg": msg, "request_id": s.requestID,
	}
	s.ServeJSON()
	s.StopRun()
}

// Info GET /api/v1/info — 公开能力说明（无需凭据）
func (s *ApiV1Controller) Info() {
	s.apiOK(map[string]interface{}{
		"product_version": version.VERSION,
		"core_version":    version.GetVersion(),
		"api":             "v1",
		"auth": []string{
			"auth_key+timestamp (admin, ±20s)",
			"Authorization: Bearer npsu_… (user)",
		},
			"endpoints": []string{
				"GET /api/v1/info",
				"GET /api/v1/status",
				"GET|POST /api/v1/client[s][+ /create|/update|/status|/delete]",
				"GET|POST /api/v1/tunnel[s][+ /create|/update|/start|/stop|/delete]",
				"GET|POST /api/v1/host[s][+ /create|/update|/start|/stop|/delete]",
				"GET|POST /api/v1/users[+ /create|/update|/delete|/bind_client|/api_key]",
				"GET /api/v1/audit",
			},
		})
	}

// Status GET /api/v1/status
func (s *ApiV1Controller) Status() {
	s.apiOK(map[string]interface{}{
		"product_version": version.VERSION,
		"core_version":    version.GetVersion(),
		"admin":           s.isAdmin(),
		"user_id":         s.apiUserID,
		"audit_enabled":   audit.Path() != "",
		"audit_connection": audit.ConnectionEnabled(),
		"audit_connection_min_interval_sec": int(audit.ConnectionMinInterval() / time.Second),
	})
}

// Clients GET /api/v1/clients — 管理员看全部；用户仅自己的客户端
func (s *ApiV1Controller) Clients() {
	start := s.GetIntNoErr("offset", 0)
	limit := s.GetIntNoErr("limit", apiV1DefaultLimit)
	if limit <= 0 {
		limit = apiV1DefaultLimit
	}
	if limit > apiV1MaxLimit {
		limit = apiV1MaxLimit
	}
	search := strings.TrimSpace(s.GetString("search"))

	var list []*file.Client
	var total int
	if s.isAdmin() {
		list, total = file.GetDb().GetClientList(start, limit, search, "", "", 0)
	} else {
		allowed := file.GetDb().UserClientIds(s.apiUserID)
		all, _ := file.GetDb().GetClientList(0, 100000, search, "", "", 0)
		filtered := make([]*file.Client, 0)
		for _, c := range all {
			if c == nil {
				continue
			}
			if _, ok := allowed[c.Id]; ok {
				filtered = append(filtered, c)
			}
		}
		total = len(filtered)
		if start >= total {
			list = []*file.Client{}
		} else {
			end := start + limit
			if end > total {
				end = total
			}
			list = filtered[start:end]
		}
	}

	rows := make([]map[string]interface{}, 0, len(list))
	for _, c := range list {
		if c == nil {
			continue
		}
		c.RLock()
		rows = append(rows, map[string]interface{}{
			"id": c.Id, "remark": c.Remark, "status": c.Status,
			"user_id": c.UserId, "verify_key": c.VerifyKey,
			"addr": c.Addr, "version": c.Version,
		})
		c.RUnlock()
	}
	s.apiOK(map[string]interface{}{
		"items": rows, "total": total, "offset": start, "limit": limit,
	})
}

// Users GET /api/v1/users — 仅管理员
func (s *ApiV1Controller) Users() {
	if !s.isAdmin() {
		s.apiFail(http.StatusForbidden, "forbidden", "该接口仅限管理员调用")
		return
	}
	start := s.GetIntNoErr("offset", 0)
	limit := s.GetIntNoErr("limit", apiV1DefaultLimit)
	if limit <= 0 {
		limit = apiV1DefaultLimit
	}
	if limit > apiV1MaxLimit {
		limit = apiV1MaxLimit
	}
	list, total := file.GetDb().GetUserList(start, limit, strings.TrimSpace(s.GetString("search")))
	rows := make([]map[string]interface{}, 0, len(list))
	for _, u := range list {
		u.RLock()
		rows = append(rows, map[string]interface{}{
			"id": u.Id, "username": u.UserName, "status": u.Status,
			"remark": u.Remark, "max_client_num": u.MaxClientNum,
			"max_tunnel_num": u.MaxTunnelNum, "expire_time": u.ExpireTime,
			"api_key_enabled": strings.TrimSpace(u.APIKeyHash) != "",
			"create_time":    u.CreateTime,
		})
		u.RUnlock()
	}
	s.apiOK(map[string]interface{}{
		"items": rows, "total": total, "offset": start, "limit": limit,
	})
}

// ApiKey POST /api/v1/users/api_key — 签发/轮换用户 API Token
// 管理员：传 id；用户 Token：只能轮换自己。
func (s *ApiV1Controller) ApiKey() {
	if s.Ctx.Request.Method != http.MethodPost {
		s.apiFail(http.StatusMethodNotAllowed, "method_not_allowed", "仅支持 POST")
		return
	}
	targetID := s.GetIntNoErr("id")
	if s.isAdmin() {
		if targetID <= 0 {
			s.apiFail(http.StatusBadRequest, "bad_request", "缺少用户 id")
			return
		}
	} else {
		targetID = s.apiUserID
	}
	user, err := file.GetDb().GetUser(targetID)
	if err != nil || user == nil {
		s.apiFail(http.StatusNotFound, "not_found", "用户不存在")
		return
	}
	token, hash, err := generateUserAPIToken()
	if err != nil {
		s.apiFail(http.StatusInternalServerError, "internal", "生成令牌失败")
		return
	}
		user.Lock()
		user.APIKeyHash = hash
		user.Unlock()
		file.GetDb().JsonDb.StoreUsersToJsonFile()
		s.auditAPI("user.api_key.rotate", "user", targetID, targetID, audit.ResultSuccess, "", nil)
		s.apiOK(map[string]interface{}{
			"user_id": targetID,
			"token":   token,
			"hint":    "请立即保存；服务端仅存哈希，无法再次完整回显",
		})
	}

func extractBearerOrToken(authorization, queryToken string) string {
	authorization = strings.TrimSpace(authorization)
	if len(authorization) >= 7 && strings.EqualFold(authorization[:7], "Bearer ") {
		return strings.TrimSpace(authorization[7:])
	}
	return strings.TrimSpace(queryToken)
}

func generateUserAPIToken() (token, hash string, err error) {
	buf := make([]byte, 32)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	token = userAPITokenPrefix + base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(token))
	hash = hex.EncodeToString(sum[:])
	return token, hash, nil
}

func validUserAPIToken(storedHash, token string) bool {
	if strings.TrimSpace(storedHash) == "" || !strings.HasPrefix(token, userAPITokenPrefix) {
		return false
	}
	sum := sha256.Sum256([]byte(token))
	want := hex.EncodeToString(sum[:])
	return subtle.ConstantTimeCompare([]byte(storedHash), []byte(want)) == 1
}

func lookupUserByAPIToken(token string) (int, bool) {
	if strings.TrimSpace(token) == "" {
		return 0, false
	}
	var userID int
	file.GetDb().JsonDb.Users.Range(func(_, value interface{}) bool {
		user, ok := value.(*file.User)
		if !ok || user == nil {
			return true
		}
		user.RLock()
		id, hash, status := user.Id, user.APIKeyHash, user.Status
		user.RUnlock()
		if status && validUserAPIToken(hash, token) {
			userID = id
			return false
		}
		return true
	})
	return userID, userID > 0
}
