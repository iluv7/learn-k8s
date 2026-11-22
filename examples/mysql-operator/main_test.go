package main

import "testing"

func TestFailoverAndRejoin(t *testing.T) {
	db := &Database{}
	cr := InnoDBCluster{Instances: 3}
	Reconcile(cr, db)
	old := db.PrimaryMember()
	old.Running = false
	old.Joined = false
	router := &Router{}
	router.Refresh(db)
	if router.Destination != "" {
		t.Fatal("故障主库仍接收连接")
	}
	db.Elect()
	router.Refresh(db)
	if router.Destination == "" || router.Destination == old.Name {
		t.Fatal("未切换到新主")
	}
	primary := db.Primary
	db.PrimaryMember().Position = 10
	Restart(old)
	if old.Joined {
		t.Fatal("kubelet 不应负责加入复制组")
	}
	Reconcile(cr, db)
	if !old.Joined || old.Position != 10 || db.Primary != primary {
		t.Fatal("旧主未追赶数据并作为副本加入")
	}
	// 多数派丢失时停止提供写入口，不能随意提升剩余成员。
	for _, m := range db.Members {
		if m.Name != old.Name {
			m.Running = false
			m.Joined = false
		}
	}
	db.Elect()
	router.Refresh(db)
	if router.Destination != "" {
		t.Fatal("无多数派时仍提供写入口")
	}
}
