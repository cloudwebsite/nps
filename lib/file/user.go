package file

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"ehang.io/nps/lib/common"
)

// NewUser 创建用户并落盘。密码调用方应已按需哈希（bcrypt）。
func (s *DbUtils) NewUser(u *User) error {
	if u == nil {
		return errors.New("用户记录无效")
	}
	u.UserName = strings.TrimSpace(u.UserName)
	if u.UserName == "" {
		return errors.New("username can not be empty")
	}
	if u.Password == "" {
		return errors.New("password can not be empty")
	}
	if u.MaxClientNum < 0 || u.MaxTunnelNum < 0 {
		return errors.New("用户配额不能小于 0")
	}
	if !s.VerifyUserLoginName(u.UserName, u.Id) {
		return errors.New("username duplicate, please reset")
	}
	if u.Id == 0 {
		u.Id = int(s.JsonDb.GetUserId())
	}
	if u.CreateTime == "" {
		u.CreateTime = time.Now().Format("2006-01-02 15:04:05")
	}
	u.Status = true
	s.JsonDb.Users.Store(u.Id, u)
	s.JsonDb.StoreUsersToJsonFile()
	return nil
}

func (s *DbUtils) UpdateUser(u *User) error {
	if u == nil || u.Id <= 0 {
		return errors.New("用户记录无效")
	}
	u.UserName = strings.TrimSpace(u.UserName)
	if u.UserName == "" || u.Password == "" {
		return errors.New("username/password can not be empty")
	}
	if !s.VerifyUserLoginName(u.UserName, u.Id) {
		return errors.New("username duplicate, please reset")
	}
	s.JsonDb.Users.Store(u.Id, u)
	s.JsonDb.StoreUsersToJsonFile()
	return nil
}

// DelUser 删除用户，并把归属该用户的客户端 UserId 清零（不删客户端）。
func (s *DbUtils) DelUser(id int) error {
	if id <= 0 {
		return errors.New("用户记录无效")
	}
	if _, err := s.GetUser(id); err != nil {
		return err
	}
	changedClients := false
	s.JsonDb.Clients.Range(func(_, value interface{}) bool {
		c, ok := value.(*Client)
		if !ok || c == nil {
			return true
		}
		c.Lock()
		if c.UserId == id {
			c.UserId = 0
			changedClients = true
		}
		c.Unlock()
		return true
	})
	s.JsonDb.Users.Delete(id)
	s.JsonDb.StoreUsersToJsonFile()
	if changedClients {
		s.JsonDb.StoreClientsToJsonFile()
	}
	return nil
}

func (s *DbUtils) GetUser(id int) (*User, error) {
	if v, ok := s.JsonDb.Users.Load(id); ok {
		if user, ok := v.(*User); ok && user != nil {
			return user, nil
		}
		return nil, errors.New("用户记录无效")
	}
	return nil, errors.New("未找到用户")
}

func (s *DbUtils) GetUserByName(username string) (*User, error) {
	username = strings.TrimSpace(username)
	var found *User
	s.JsonDb.Users.Range(func(_, value interface{}) bool {
		u, ok := value.(*User)
		if !ok || u == nil {
			return true
		}
		u.RLock()
		name := u.UserName
		u.RUnlock()
		if name == username {
			found = u
			return false
		}
		return true
	})
	if found == nil {
		return nil, errors.New("未找到用户")
	}
	return found, nil
}

// VerifyUserLoginName 用户名是否未被其它用户占用（excludeId 自身除外）。
func (s *DbUtils) VerifyUserLoginName(username string, excludeId int) bool {
	username = strings.TrimSpace(username)
	if username == "" {
		return false
	}
	ok := true
	s.JsonDb.Users.Range(func(_, value interface{}) bool {
		u, valid := value.(*User)
		if !valid || u == nil {
			return true
		}
		u.RLock()
		id, name := u.Id, u.UserName
		u.RUnlock()
		if name == username && id != excludeId {
			ok = false
			return false
		}
		return true
	})
	return ok
}

// IsUserActive 用户存在且 Status=true。
func (s *DbUtils) IsUserActive(id int) bool {
	u, err := s.GetUser(id)
	if err != nil || u == nil {
		return false
	}
	u.RLock()
	defer u.RUnlock()
	return u.Status
}

// IsUserUsable 用户可用：启用且未过期（ExpireTime 空=不限）。
func (s *DbUtils) IsUserUsable(id int) bool {
	u, err := s.GetUser(id)
	if err != nil || u == nil {
		return false
	}
	u.RLock()
	status, expire := u.Status, u.ExpireTime
	u.RUnlock()
	if !status {
		return false
	}
	if expire == "" {
		return true
	}
	t, err := time.ParseInLocation("2006-01-02 15:04:05", expire, time.Local)
	if err != nil {
		return true
	}
	return time.Now().Before(t)
}

// UserTunnelCount 统计归属用户下所有客户端的隧道+域名数量。
func (s *DbUtils) UserTunnelCount(userId int) int {
	ids := s.UserClientIds(userId)
	if len(ids) == 0 {
		return 0
	}
	n := 0
	s.JsonDb.Tasks.Range(func(_, value interface{}) bool {
		t, ok := value.(*Tunnel)
		if !ok || t == nil || t.NoStore || t.Client == nil {
			return true
		}
		if _, owned := ids[t.Client.Id]; owned {
			n++
		}
		return true
	})
	s.JsonDb.Hosts.Range(func(_, value interface{}) bool {
		h, ok := value.(*Host)
		if !ok || h == nil || h.NoStore || h.Client == nil {
			return true
		}
		if _, owned := ids[h.Client.Id]; owned {
			n++
		}
		return true
	})
	return n
}

// CheckCanAddClient 校验用户客户端配额（userId<=0 跳过）。
func (s *DbUtils) CheckCanAddClient(userId int) error {
	if userId <= 0 {
		return nil
	}
	if !s.IsUserUsable(userId) {
		return errors.New("用户已停用或已过期")
	}
	u, err := s.GetUser(userId)
	if err != nil {
		return err
	}
	u.RLock()
	max := u.MaxClientNum
	u.RUnlock()
	if max <= 0 {
		return nil
	}
	if len(s.UserClientIds(userId)) >= max {
		return errors.New("用户客户端数量超过配额")
	}
	return nil
}

// CheckCanAddTunnelOrHost 校验用户隧道/域名总配额（userId<=0 跳过）。
func (s *DbUtils) CheckCanAddTunnelOrHost(userId int) error {
	if userId <= 0 {
		return nil
	}
	if !s.IsUserUsable(userId) {
		return errors.New("用户已停用或已过期")
	}
	u, err := s.GetUser(userId)
	if err != nil {
		return err
	}
	u.RLock()
	max := u.MaxTunnelNum
	u.RUnlock()
	if max <= 0 {
		return nil
	}
	if s.UserTunnelCount(userId) >= max {
		return errors.New("用户隧道/域名数量超过配额")
	}
	return nil
}

// UserClientIds 返回归属该用户的客户端 ID 集合。
func (s *DbUtils) UserClientIds(userId int) map[int]struct{} {
	ids := make(map[int]struct{})
	if userId <= 0 {
		return ids
	}
	s.JsonDb.Clients.Range(func(_, value interface{}) bool {
		c, ok := value.(*Client)
		if !ok || c == nil {
			return true
		}
		c.RLock()
		uid, cid := c.UserId, c.Id
		c.RUnlock()
		if uid == userId {
			ids[cid] = struct{}{}
		}
		return true
	})
	return ids
}

func (s *DbUtils) GetUserList(start, length int, search string) ([]*User, int) {
	all := make([]*User, 0)
	s.JsonDb.Users.Range(func(_, value interface{}) bool {
		u, ok := value.(*User)
		if !ok || u == nil {
			return true
		}
		if search != "" {
			u.RLock()
			match := strings.Contains(u.UserName, search) || strings.Contains(u.Remark, search) || fmt.Sprintf("%d", u.Id) == search
			u.RUnlock()
			if !match {
				return true
			}
		}
		all = append(all, u)
		return true
	})
	cnt := len(all)
	if length <= 0 {
		return all, cnt
	}
	if start >= cnt {
		return []*User{}, cnt
	}
	end := start + length
	if end > cnt {
		end = cnt
	}
	return all[start:end], cnt
}

// MigrateUsersFromClients 从旧客户端 Web 账号推导用户归属。
// 仅当 users.json 尚不存在时允许「新建用户」；已有文件时只做凭据匹配的 UserId 修复，避免复活已删账号。
func (s *DbUtils) MigrateUsersFromClients() error {
	type credential struct {
		userId   int
		password string
	}
	recoverMissingOwners := !common.FileExists(s.JsonDb.UserFilePath)
	byName := make(map[string]credential)
	byID := make(map[int]struct{})
	s.JsonDb.Users.Range(func(_, value interface{}) bool {
		u, ok := value.(*User)
		if !ok || u == nil {
			return true
		}
		u.RLock()
		userName, userID, password := u.UserName, u.Id, u.Password
		u.RUnlock()
		byName[userName] = credential{userId: userID, password: password}
		byID[userID] = struct{}{}
		return true
	})

	changedUsers := false
	changedClients := false
	passwordMatch := func(stored, submitted string) bool {
		if stored == "" || submitted == "" {
			return false
		}
		if common.ConstantTimeStrEq(stored, submitted) {
			return true
		}
		return common.VerifyPassword(stored, submitted)
	}

	s.JsonDb.Clients.Range(func(_, value interface{}) bool {
		c, ok := value.(*Client)
		if !ok || c == nil {
			return true
		}
		c.RLock()
		clientUserID, webUserName, webPassword, remark, clientID := c.UserId, c.WebUserName, c.WebPassword, c.Remark, c.Id
		c.RUnlock()
		if clientUserID != 0 {
			if _, exists := byID[clientUserID]; exists {
				return true
			}
			if webUserName != "" && webPassword != "" {
				if cred, exists := byName[webUserName]; exists && passwordMatch(cred.password, webPassword) {
					c.Lock()
					c.UserId = cred.userId
					c.Unlock()
					changedClients = true
					return true
				}
			}
			if !recoverMissingOwners || webUserName == "" || webPassword == "" {
				return true
			}
			name := webUserName
			if cred, exists := byName[name]; exists && (!passwordMatch(cred.password, webPassword) || cred.userId != clientUserID) {
				name = fmt.Sprintf("%s_%d", webUserName, clientUserID)
			}
			u := &User{
				Id:         clientUserID,
				UserName:   name,
				Password:   webPassword,
				Status:     true,
				Remark:     remark,
				CreateTime: time.Now().Format("2006-01-02 15:04:05"),
			}
			s.JsonDb.Users.Store(u.Id, u)
			byID[u.Id] = struct{}{}
			byName[name] = credential{userId: u.Id, password: u.Password}
			if clientUserID > int(atomic.LoadInt32(&s.JsonDb.UserIncreaseId)) {
				atomic.StoreInt32(&s.JsonDb.UserIncreaseId, int32(clientUserID))
			}
			changedUsers = true
			return true
		}
		if webUserName == "" || webPassword == "" {
			return true
		}
		cred, ok := byName[webUserName]
		if ok && passwordMatch(cred.password, webPassword) {
			c.Lock()
			c.UserId = cred.userId
			c.Unlock()
			changedClients = true
			return true
		}
		if !recoverMissingOwners {
			return true
		}
		name := webUserName
		if ok && !passwordMatch(cred.password, webPassword) {
			name = fmt.Sprintf("%s_%d", webUserName, clientID)
			ok = false
		}
		if !ok {
			u := &User{
				Id:         int(s.JsonDb.GetUserId()),
				UserName:   name,
				Password:   webPassword,
				Status:     true,
				Remark:     remark,
				CreateTime: time.Now().Format("2006-01-02 15:04:05"),
			}
			s.JsonDb.Users.Store(u.Id, u)
			cred = credential{userId: u.Id, password: u.Password}
			byName[name] = cred
			changedUsers = true
		}
		c.Lock()
		c.UserId = cred.userId
		c.Unlock()
		changedClients = true
		return true
	})
	if changedUsers {
		s.JsonDb.StoreUsersToJsonFile()
	}
	if changedClients {
		s.JsonDb.StoreClientsToJsonFile()
	}
	return nil
}
