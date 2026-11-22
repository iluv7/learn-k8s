package main

import "testing"

func TestFailoverAndRejoin(t *testing.T) {
	db := &Database{}
	api := NewAPI()
	for _, file := range []string{"crd.yaml", "cluster.yaml"} {
		data, _ := manifests.ReadFile(file)
		if err := api.Apply(data); err != nil {
			t.Fatal(err)
		}
	}
	operator := &Operator{API: api, DB: db}
	operator.Reconcile()
	if len(db.Members) != 0 {
		t.Fatal("Operator 不应直接启动容器")
	}
	api.RunWorkloads(db)
	operator.Reconcile()
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
	operator.Reconcile()
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

// 验证必须先注册类型，schema 校验成功后才保存；多轮协调不重复创建成员。
func TestCRDToWorkload(t *testing.T) {
	api := NewAPI()
	cr, _ := manifests.ReadFile("cluster.yaml")
	definition, _ := manifests.ReadFile("crd.yaml")
	if api.Apply(cr) == nil {
		t.Fatal("未注册类型却接受 CR")
	}
	if err := api.Apply(definition); err != nil {
		t.Fatal(err)
	}
	invalid := []byte(`{"apiVersion":"demo.learn-k8s.io/v1","kind":"InnoDBCluster","metadata":{"name":"bad"},"spec":{"instances":1}}`)
	if api.Apply(invalid) == nil {
		t.Fatal("schema 未拒绝非法实例数")
	}
	if _, ok := api.Cluster(); ok {
		t.Fatal("无效 CR 被保存")
	}
	if err := api.Apply(cr); err != nil {
		t.Fatal(err)
	}
	db := &Database{}
	operator := &Operator{api, db}
	operator.Reconcile()
	if len(api.Workloads) != 3 {
		t.Fatal("未创建数据库、Router 和 Service 资源")
	}
	api.RunWorkloads(db)
	operator.Reconcile()
	operator.Reconcile()
	if len(db.Members) != 3 || api.Primary != "mysql-0" {
		t.Fatal("初始化或状态上报失败")
	}
}

// 全部成员故障后，仅重启进程不等于数据库恢复；Operator 不能重新引导。
func TestAllMembersRestartWaitsForRecovery(t *testing.T) {
	api := NewAPI()
	for _, file := range []string{"crd.yaml", "cluster.yaml"} {
		data, err := manifests.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if err := api.Apply(data); err != nil {
			t.Fatal(err)
		}
	}
	db := &Database{}
	operator := &Operator{API: api, DB: db}
	operator.Reconcile()
	api.RunWorkloads(db)
	operator.Reconcile()
	// 第一个成员故意不是数据最新的成员，避免错误初始化被测试掩盖。
	positions := []int{8, 10, 10}
	for i, m := range db.Members {
		m.Position = positions[i]
		m.Running = false
		m.Joined = false
	}
	db.Elect()
	for _, m := range db.Members {
		Restart(m)
	}
	// 重建 Operator 对象，模拟控制器重启；初始化历史不能只存在于控制器中。
	operator = &Operator{API: api, DB: db}
	operator.Reconcile()
	operator.Reconcile()
	if db.Primary != "" {
		t.Fatal("全故障后误选第一个成员为主")
	}
	for i, m := range db.Members {
		if m.Joined || m.Position != positions[i] {
			t.Fatal("未确认恢复就加入或覆盖了数据")
		}
	}
	router := &Router{}
	router.Refresh(db)
	if router.Destination != "" {
		t.Fatal("未恢复却提供写入口")
	}
	if api.Primary != "" {
		t.Fatal("状态仍显示旧主")
	}
	// 模拟管理员/数据库完成安全恢复，确认 mysql-1 为主；不在本例实现恢复算法。
	db.Primary = db.Members[1].Name
	db.Members[1].Joined = true
	operator.Reconcile()
	router.Refresh(db)
	for _, m := range db.Members {
		if !m.Joined || m.Position != 10 {
			t.Fatal("确认主库后未完成成员恢复")
		}
	}
	if router.Destination != "mysql-1" || api.Primary != "mysql-1" {
		t.Fatal("恢复后路由或状态未更新")
	}
}
