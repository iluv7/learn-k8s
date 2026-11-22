// 用内存状态演示职责分工，不连接真实 MySQL，也不实现分布式选举协议。
package main

import "fmt"

// InnoDBCluster 是 CR；真实 CRD 负责向 API Server 注册这种资源及字段。
type InnoDBCluster struct {
	Instances int
}

type Member struct {
	Name            string
	Running, Joined bool
	Position        int // 用递增数字模拟已同步的数据位置。
}

type Database struct {
	Members []*Member
	Primary string
}

func (db *Database) PrimaryMember() *Member {
	for _, m := range db.Members {
		if m.Name == db.Primary {
			return m
		}
	}
	return nil
}

// Reconcile 模拟 Operator：初始化集群，并协调恢复成员追赶数据、重新加入。
// 为保持例子最小，只支持固定三个成员，不实现扩缩容或版本升级。
func Reconcile(cr InnoDBCluster, db *Database) {
	if len(db.Members) == 0 {
		for i := 0; i < cr.Instances; i++ {
			db.Members = append(db.Members, &Member{Name: fmt.Sprintf("mysql-%d", i), Running: true, Joined: true})
		}
		db.Primary = db.Members[0].Name
		fmt.Println("Operator: 初始化三个成员和复制关系")
	}
	primary := db.PrimaryMember()
	if primary == nil || !primary.Running || !primary.Joined {
		return
	}
	for _, m := range db.Members {
		if m.Running && !m.Joined {
			// Operator 发起协调，实际同步由数据库恢复/复制机制执行。
			m.Position = primary.Position
			m.Joined = true
			fmt.Println("Operator: 协调", m.Name, "同步数据并作为副本重新加入")
		}
	}
}

// Elect 模拟 Group Replication：由数据库侧确认多数派并选择新主。
// 简化为从存活成员中选数据位置最大的一个，不能作为真实选举算法。
func (db *Database) Elect() {
	var candidate *Member
	online := 0
	for _, m := range db.Members {
		if m.Running && m.Joined {
			online++
			if candidate == nil || m.Position > candidate.Position {
				candidate = m
			}
		}
	}
	if online <= len(db.Members)/2 {
		db.Primary = ""
		return
	}
	current := db.PrimaryMember()
	if current != nil && current.Running && current.Joined {
		return
	}
	db.Primary = candidate.Name
	fmt.Println("MySQL: 选出新主", db.Primary)
}

// Restart 模拟 kubelet：只恢复进程，不擅自恢复主库角色或加入复制组。
func Restart(m *Member) {
	m.Running = true
	fmt.Println("kubelet: 重启", m.Name, "容器，等待数据库恢复")
}

type Router struct{ Destination string }

// Refresh 模拟 Router 自行读取数据库拓扑；不修改 Ingress 或 Service。
func (r *Router) Refresh(db *Database) {
	r.Destination = ""
	if m := db.PrimaryMember(); m != nil && m.Running && m.Joined {
		r.Destination = m.Name
	}
}
func (r *Router) Connect() string {
	if r.Destination == "" {
		return "连接失败：暂时没有可用主库"
	}
	return "Java → 固定 Service → Router → " + r.Destination
}

func main() {
	cr := InnoDBCluster{Instances: 3}
	db := &Database{}
	router := &Router{}
	Reconcile(cr, db)
	router.Refresh(db)
	fmt.Println(router.Connect())

	old := db.PrimaryMember()
	old.Running = false
	old.Joined = false
	router.Refresh(db)
	fmt.Println(router.Connect()) // 新主尚未选出时有短暂不可写窗口。
	db.Elect()
	router.Refresh(db)
	fmt.Println(router.Connect())

	// 新主继续接受写入，由 MySQL 复制到在线副本；旧主落后。
	for _, m := range db.Members {
		if m.Running && m.Joined {
			m.Position = 10
		}
	}
	Restart(old)
	Reconcile(cr, db)
	fmt.Printf("恢复结果：主库=%s，%s 已加入=%t，数据位置=%d\n", db.Primary, old.Name, old.Joined, old.Position)
}
