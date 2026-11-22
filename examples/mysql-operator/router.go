package main

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
