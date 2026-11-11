// 部署演示入口：读声明 → 提交 → 协调 → 访问 → 容器故障 → Pod 删除及补齐。
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func run() error {
	file := "deployment.json"
	if len(os.Args) > 1 {
		file = os.Args[1]
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	var d Deployment
	if err = json.Unmarshal(data, &d); err != nil {
		return err
	}
	c := NewCluster()
	// 示例依赖独立创建；Deployment 只通过名字引用。
	ns := namespace(d.Namespace)
	c.API.PutNamespace(ns)
	if err := c.API.PutConfigMap(ns, "java-config", map[string]string{"LOG_LEVEL": "INFO", "UPLOAD_DIR": "/app/uploads"}); err != nil {
		return err
	}
	if err := c.API.PutSecret(ns, "java-secret", map[string]string{"DB_PASSWORD": "demo-only"}); err != nil {
		return err
	}
	if err := c.API.PutPVC(ns, "java-data", 10); err != nil {
		return err
	}
	if err = c.API.Apply(d, "demo-admin"); err != nil {
		return err
	}
	// 驱动组件将声明的三个副本协调为已调度、已启动的 Pod。
	c.Tick()
	c.API.PutService(resourceKey(ns, "java"), Service{d.Name})
	c.API.PutIngress(resourceKey(ns, "java"), Ingress{"java.local", "/", "java"})
	fmt.Println(c.RequestInNamespace(ns, "java.local", "/hello"))
	pods := c.API.Pods()
	if len(pods) > 0 {
		p := pods[0]
		// 同一 PVC 的文件不依赖旧 Pod 身份；重建后仍能读取。
		if p.Mounted {
			if err := c.API.WriteFile(p, "hello.txt", "Java 上传的示例文件"); err != nil {
				return err
			}
		}
		// 故障一：容器退出，下一轮在同一个 Pod 内重启。
		p.Alive = false
		p.Ready = false
		c.API.PutPod(p)
		// 驱动组件将声明的三个副本协调为已调度、已启动的 Pod。
		c.Tick()
		// 故障二：Pod 被删除，由 ReplicaSet 补建，再调度和启动。
		c.API.DeletePod(resourceKey(p.Namespace, p.Name))
		// 驱动组件将声明的三个副本协调为已调度、已启动的 Pod。
		c.Tick()
	}
	for _, pod := range c.API.Pods() {
		if pod.Mounted {
			content, err := c.API.ReadFile(pod, "hello.txt")
			if err != nil {
				return err
			}
			fmt.Printf("PVC 文件在 Pod 重建后保留: %s\n", content)
			break
		}
	}
	// 用注入指标演示扩容和缩容；不依赖真实 Metrics Server。
	if err := c.API.PutHPA(ns, "java", HPA{Target: d.Name, Min: 3, Max: 5, TargetCPU: 60}); err != nil {
		return err
	}
	for _, pod := range c.API.Pods() {
		_ = c.API.SetCPU(pod.Namespace, pod.Name, 500)
	}
	c.Tick() // 扩容到 5，两个节点最多运行 4 个，剩余 Pending。
	for _, pod := range c.API.Pods() {
		_ = c.API.SetCPU(pod.Namespace, pod.Name, 50)
	}
	c.Tick() // 低负载回到最少 3 个副本。
	for _, e := range c.API.Events {
		fmt.Println(e)
	}
	fmt.Println("最终 Pod:")
	for _, p := range c.API.Pods() {
		fmt.Printf("%s/%s node=%s ready=%t restarts=%d mounted=%t\n", p.Namespace, p.Name, p.Node, p.Ready, p.Restarts, p.Mounted)
	}
	return nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
