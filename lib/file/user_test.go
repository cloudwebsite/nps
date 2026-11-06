package file

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrateUsersFromClientsCreatesOwnersWhenNoUsersFile(t *testing.T) {
	dir := t.TempDir()
	db := &DbUtils{JsonDb: NewJsonDb(dir)}
	c := &Client{
		Id: 1, VerifyKey: "vk1", Status: true,
		WebUserName: "alice", WebPassword: "secret",
		Remark: "r1", Flow: &Flow{},
	}
	db.JsonDb.Clients.Store(c.Id, c)

	if err := db.MigrateUsersFromClients(); err != nil {
		t.Fatal(err)
	}
	if c.UserId == 0 {
		t.Fatal("expected UserId assigned")
	}
	u, err := db.GetUser(c.UserId)
	if err != nil || u.UserName != "alice" {
		t.Fatalf("user=%v err=%v", u, err)
	}
}

func TestMigrateDoesNotCreateWhenUsersFileExists(t *testing.T) {
	dir := t.TempDir()
	db := &DbUtils{JsonDb: NewJsonDb(dir)}
	if err := os.MkdirAll(filepath.Dir(db.JsonDb.UserFilePath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(db.JsonDb.UserFilePath, []byte("{}"), 0o640); err != nil {
		t.Fatal(err)
	}
	c := &Client{
		Id: 2, VerifyKey: "vk2", Status: true,
		WebUserName: "bob", WebPassword: "pw",
		Flow: &Flow{},
	}
	db.JsonDb.Clients.Store(c.Id, c)
	if err := db.MigrateUsersFromClients(); err != nil {
		t.Fatal(err)
	}
	if c.UserId != 0 {
		t.Fatal("must not create new users when users.json already exists")
	}
}

	func TestVerifyUserLoginNameAndAPIKeyFields(t *testing.T) {
		dir := t.TempDir()
		db := &DbUtils{JsonDb: NewJsonDb(dir)}
		u := &User{UserName: "u1", Password: "p", Status: true}
		if err := db.NewUser(u); err != nil {
			t.Fatal(err)
		}
		if db.VerifyUserLoginName("u1", 0) {
			t.Fatal("duplicate should fail")
		}
		if !db.VerifyUserLoginName("u1", u.Id) {
			t.Fatal("same id ok")
		}
		if !db.IsUserActive(u.Id) {
			t.Fatal("active")
		}
	}

	func TestUserQuotaChecks(t *testing.T) {
		dir := t.TempDir()
		db := &DbUtils{JsonDb: NewJsonDb(dir)}
		u := &User{UserName: "quota", Password: "p", Status: true, MaxClientNum: 1, MaxTunnelNum: 1}
		if err := db.NewUser(u); err != nil {
			t.Fatal(err)
		}
		c1 := &Client{Id: 1, VerifyKey: "vk1", Status: true, UserId: u.Id, Flow: &Flow{}}
		db.JsonDb.Clients.Store(c1.Id, c1)
		if err := db.CheckCanAddClient(u.Id); err == nil {
			t.Fatal("expected client quota error")
		}
		if err := db.CheckCanAddTunnelOrHost(u.Id); err != nil {
			t.Fatal(err)
		}
		db.JsonDb.Tasks.Store(1, &Tunnel{Id: 1, Client: c1, Flow: &Flow{}})
		if err := db.CheckCanAddTunnelOrHost(u.Id); err == nil {
			t.Fatal("expected tunnel quota error")
		}
		u.Status = false
		db.JsonDb.Users.Store(u.Id, u)
		if db.IsUserUsable(u.Id) {
			t.Fatal("disabled user should be unusable")
		}
	}

	func TestDelUserClearsClientOwner(t *testing.T) {
		dir := t.TempDir()
		db := &DbUtils{JsonDb: NewJsonDb(dir)}
		u := &User{UserName: "owner", Password: "p", Status: true}
		if err := db.NewUser(u); err != nil {
			t.Fatal(err)
		}
		c := &Client{Id: 9, VerifyKey: "vk9", Status: true, UserId: u.Id, Flow: &Flow{}}
		db.JsonDb.Clients.Store(c.Id, c)
		if err := db.DelUser(u.Id); err != nil {
			t.Fatal(err)
		}
		if _, err := db.GetUser(u.Id); err == nil {
			t.Fatal("user should be gone")
		}
		if c.UserId != 0 {
			t.Fatalf("client UserId=%d, want 0", c.UserId)
		}
	}
