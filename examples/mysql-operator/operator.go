package main

import "fmt"

// Operator 观察 CR，经 API 创建工作负载，并执行数据库专属协调。
// 示例用手动调用 Reconcile 表示多轮观察，省略 watch、队列和并发。
type Operator struct {
	API *API
	DB  *Database
}

func (o *Operator) Reconcile() {
	cr, ok := o.API.Cluster()
	if !ok {
		return
	}
	fmt.Println("Operator: 观察 CR", cr.Metadata.Name)
	o.API.EnsureWorkload(cr.Metadata.Name+"-mysql", Workload{"StatefulSet", cr.Spec.Instances})
	o.API.EnsureWorkload(cr.Metadata.Name+"-router", Workload{"Deployment", 2})
	o.API.EnsureWorkload(cr.Metadata.Name+"-entry", Workload{"Service", 0})
	if len(o.DB.Members) != cr.Spec.Instances {
		return
	} // 先等 K8s 将实例启动。
	if o.DB.Primary == "" {
		// 首次引导和全故障恢复分开：已初始化过的集群必须等待确认恢复。
		if o.DB.Initialized {
			o.API.Primary = ""
			fmt.Println("Operator: 集群已初始化，等待数据库显式恢复主库")
			return
		}
		for _, m := range o.DB.Members {
			if !m.Running {
				return
			}
		}

		o.DB.Primary = o.DB.Members[0].Name
		for _, m := range o.DB.Members {
			m.Joined = true
		}
		o.DB.Initialized = true
		fmt.Println("Operator: 初始化复制集群")
	}
	primary := o.DB.PrimaryMember()
	if primary == nil || !primary.Running || !primary.Joined {
		o.API.Primary = ""
		return
	}
	for _, m := range o.DB.Members {
		if m.Running && !m.Joined {
			// 模拟调用数据库恢复接口，实际数据同步由 MySQL 执行。
			m.Position = primary.Position
			m.Joined = true
			fmt.Println("Operator: 协调", m.Name, "同步数据并重新加入")
		}
	}
	o.API.Primary = o.DB.Primary
}
