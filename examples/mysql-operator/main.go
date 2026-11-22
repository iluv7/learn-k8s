// 端到端内存模拟：不连接真实 K8s 或 MySQL。
package main

import (
	"embed"
	"fmt"
)

// 文件内使用 JSON 形式的合法 YAML，便于标准库直接解析，不引入 YAML 依赖。
//
//go:embed crd.yaml cluster.yaml
var manifests embed.FS

func main() {
	api := NewAPI()
	// 模拟 kubectl apply：读本地文件，然后提交给 API Server。
	for _, file := range []string{"crd.yaml", "cluster.yaml"} {
		data, err := manifests.ReadFile(file)
		if err != nil {
			panic(err)
		}
		fmt.Println("kubectl apply -f", file)
		if err := api.Apply(data); err != nil {
			panic(err)
		}
	}
	db := &Database{}
	operator := &Operator{API: api, DB: db}
	router := &Router{}
	// 第一轮只创建工作负载，不由 Operator 冒充 kubelet 启动进程。
	operator.Reconcile()
	api.RunWorkloads(db)
	operator.Reconcile() // Pod 启动后，再协调数据库初始化、成员加入。
	router.Refresh(db)
	fmt.Println(router.Connect())

	old := db.PrimaryMember()
	old.Running = false
	old.Joined = false
	router.Refresh(db)
	fmt.Println(router.Connect())
	db.Elect() // MySQL 侧选主。
	router.Refresh(db)
	fmt.Println(router.Connect())
	for _, m := range db.Members {
		if m.Running && m.Joined {
			m.Position = 10
		}
	}
	Restart(old)         // kubelet 侧只重启进程。
	operator.Reconcile() // Operator 协调 MySQL 同步数据、重新加入。
	fmt.Printf("恢复结果：主库=%s，旧主已加入=%t，数据位置=%d\n", db.Primary, old.Joined, old.Position)
}
