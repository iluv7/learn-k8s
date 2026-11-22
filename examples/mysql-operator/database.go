package main

import "fmt"

type Member struct {
	Name            string
	Running, Joined bool
	Position        int // 用递增数字模拟已同步的数据位置。
}

type Database struct {
	// 独立记录初始化历史，不随成员退出或 Operator 重启而清空。
	Initialized bool
	Members     []*Member
	Primary     string
}

func (db *Database) PrimaryMember() *Member {
	for _, m := range db.Members {
		if m.Name == db.Primary {
			return m
		}
	}
	return nil
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
