package main

import "testing"

// 同名部署互不覆盖；配置只能从 Pod 自己的 Namespace 读取。
func TestNamespaceAndDependencies(t *testing.T) {
	c := NewCluster()
	d := Deployment{Name: "java", Namespace: "a", Image: "java:test", Replicas: 1, CPU: 500, Memory: 256, ConfigMap: "config", Secret: "secret", PVC: "data"}
	if c.API.Apply(d, "demo-admin") == nil {
		t.Fatal("接受了不存在的空间")
	}
	c.API.PutNamespace("a")
	c.API.PutNamespace("b")
	if err := c.API.Apply(d, "demo-admin"); err != nil {
		t.Fatal(err)
	}
	other := d
	other.Namespace = "b"
	if err := c.API.Apply(other, "demo-admin"); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{"LOG_LEVEL": "INFO"}
	_ = c.API.PutConfigMap("a", "config", values)
	values["LOG_LEVEL"] = "changed"
	_ = c.API.PutSecret("a", "secret", map[string]string{"PASSWORD": "demo"})
	_ = c.API.PutPVC("a", "data", 10)
	c.Tick()
	for _, p := range c.API.Pods() {
		if p.Namespace == "a" {
			if !p.Ready || !p.Mounted || p.Env["PASSWORD"] != "demo" || p.Env["LOG_LEVEL"] != "INFO" {
				t.Fatal("依赖注入失败")
			}
			p.Env["PASSWORD"] = "mutated"
		} else if p.Ready {
			t.Fatal("跨空间读取了依赖")
		}
	}
	// 补齐 b 的依赖后，下一轮即可启动。
	_ = c.API.PutConfigMap("b", "config", map[string]string{})
	_ = c.API.PutSecret("b", "secret", map[string]string{})
	_ = c.API.PutPVC("b", "data", 10)
	c.Tick()
	for _, p := range c.API.Pods() {
		if !p.Ready {
			t.Fatal("补齐依赖后未启动")
		}
		if p.Namespace == "a" && p.Env["PASSWORD"] != "demo" {
			t.Fatal("读取值修改了存储")
		}
	}
	if len(c.API.Deployments()) != 2 || len(c.API.Pods()) != 2 {
		t.Fatal("同名资源冲突")
	}
}

// 卷数据跟随 PVC，而不是跟随某个 Pod 的身份。
func TestPVCAndConfigSnapshot(t *testing.T) {
	c := NewCluster()
	d := Deployment{Name: "java", Image: "java:test", Replicas: 2, CPU: 500, Memory: 256, ConfigMap: "config", PVC: "data"}
	_ = c.API.PutConfigMap("default", "config", map[string]string{"VERSION": "v1"})
	_ = c.API.PutPVC("default", "data", 10)
	_ = c.API.Apply(d, "demo-admin")
	c.Tick()
	old := c.API.Pods()[0]
	if err := c.API.WriteFile(old, "hello.txt", "hello"); err != nil {
		t.Fatal(err)
	}
	if v, err := c.API.ReadFile(c.API.Pods()[1], "hello.txt"); err != nil || v != "hello" {
		t.Fatal("副本未共享卷")
	}
	_ = c.API.PutConfigMap("default", "config", map[string]string{"VERSION": "v2"})
	c.Tick()
	if c.API.Pods()[0].Env["VERSION"] != "v1" {
		t.Fatal("运行中环境变量不应热更新")
	}
	c.API.DeletePod(old.Name)
	c.Tick()
	found := false
	for _, p := range c.API.Pods() {
		if p.Name != old.Name && p.Env["VERSION"] == "v2" {
			found = true
			if v, err := c.API.ReadFile(p, "hello.txt"); err != nil || v != "hello" {
				t.Fatal("重建后卷数据丢失")
			}
		}
	}
	if !found {
		t.Fatal("新 Pod 未读取新配置")
	}
	if c.API.PutPVC("default", "bad", 0) == nil {
		t.Fatal("接受无效容量")
	}
}

// 指标驱动期望副本变化；缺少指标时保持，缩容受最小副本数约束。
func TestHPA(t *testing.T) {
	c := fixture(t)
	if err := c.API.PutHPA("default", "java", HPA{Target: "java", Min: 2, Max: 5, TargetCPU: 60}); err != nil {
		t.Fatal(err)
	}
	c.Tick()
	if c.API.Deployments()[0].Replicas != 3 {
		t.Fatal("无指标时不应扩缩")
	}
	for _, p := range c.API.Pods() {
		_ = c.API.SetCPU(p.Namespace, p.Name, 500)
	}
	c.Tick()
	if c.API.Deployments()[0].Replicas != 5 || len(c.API.Pods()) != 5 {
		t.Fatal("未扩容到上限")
	}
	pending := 0
	for _, p := range c.API.Pods() {
		if p.Node == "" {
			pending++
		}
		_ = c.API.SetCPU(p.Namespace, p.Name, 50)
	}
	if pending != 1 {
		t.Fatal("扩容没有保留资源不足的 Pod")
	}
	c.Tick()
	if c.API.Deployments()[0].Replicas != 2 || len(c.API.Pods()) != 2 {
		t.Fatal("未缩容到下限")
	}
	if c.API.PutHPA("default", "bad", HPA{Min: 3, Max: 1, TargetCPU: 60}) == nil {
		t.Fatal("接受无效 HPA")
	}
}
