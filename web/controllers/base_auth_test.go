package controllers

import (
	"testing"

	"ehang.io/nps/lib/file"
)

func TestIndexHostActionsDoNotMatchTunnelChangeStatus(t *testing.T) {
	if _, ok := indexHostActions["changestatus"]; ok {
		t.Fatal("changestatus must not be treated as host action")
	}
	if _, ok := indexHostActions["edithost"]; !ok {
		t.Fatal("edithost should be host action")
	}
	if _, ok := indexHostActions["delhost"]; !ok {
		t.Fatal("delhost should be host action")
	}
	if _, ok := indexHostActions["copy"]; ok {
		t.Fatal("copy is tunnel action")
	}
}

func TestTunnelAndHostBelongingUseSeparateIDSpaces(t *testing.T) {
	db := file.GetDb()
	clientA := &file.Client{Id: 101, Cnf: &file.Config{}, Flow: &file.Flow{}, Rate: nil}
	clientB := &file.Client{Id: 202, Cnf: &file.Config{}, Flow: &file.Flow{}, Rate: nil}
	db.JsonDb.Clients.Store(clientA.Id, clientA)
	db.JsonDb.Clients.Store(clientB.Id, clientB)
	defer func() {
		db.JsonDb.Clients.Delete(clientA.Id)
		db.JsonDb.Clients.Delete(clientB.Id)
		db.JsonDb.Tasks.Delete(7)
		db.JsonDb.Hosts.Delete(7)
	}()

	// 故意让 Host #7 与 Task #7 重号：A 拥有 Host，B 拥有 Tunnel。
	db.JsonDb.Hosts.Store(7, &file.Host{Id: 7, Client: clientA, Flow: &file.Flow{}, Target: &file.Target{}})
	db.JsonDb.Tasks.Store(7, &file.Tunnel{Id: 7, Client: clientB, Flow: &file.Flow{}, Target: &file.Target{}})

	s := &BaseController{}
	if !s.hostBelongsToMe(7, clientA.Id) {
		t.Fatal("host #7 should belong to client A")
	}
	if s.hostBelongsToMe(7, clientB.Id) {
		t.Fatal("host #7 must not belong to client B")
	}
	if !s.tunnelBelongsToMe(7, clientB.Id) {
		t.Fatal("tunnel #7 should belong to client B")
	}
	if s.tunnelBelongsToMe(7, clientA.Id) {
		t.Fatal("tunnel #7 must not belong to client A (ID collision must not cross-authorize)")
	}
}
