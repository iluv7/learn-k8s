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
