// 步骤 5：为未绑定的 Pod 过滤节点、评分，并通过 API Server 保存绑定。
package main

type Node struct {
	Name        string
	CPU, Memory int
}
type Scheduler struct {
	API   *APIServer
	Nodes []Node
}

func (s *Scheduler) Reconcile() {
	for _, p := range s.API.Pods() {
		if p.Node != "" {
			continue
		}
		best := ""
		score := -1.0
		for _, n := range s.Nodes {
			cpu, mem := n.CPU, n.Memory
			// 已绑定但尚未启动的 Pod 也占用 requests，避免超额分配。
			for _, other := range s.API.Pods() {
				if other.Node == n.Name {
					cpu -= other.CPU
					mem -= other.Memory
				}
			}
			// Filter：CPU 单位为毫核，内存单位为 Mi；不使用实时利用率。
			if cpu < p.CPU || mem < p.Memory {
				continue
			}
			// Score：优先选择剩余资源比例大的节点，同分保留先遇到的节点。
			candidate := float64(cpu)/float64(n.CPU) + float64(mem)/float64(n.Memory)
			if candidate > score {
				score = candidate
				best = n.Name
			}
		}
		if best == "" {
			s.API.log("Scheduler: %s 资源不足，保持 Pending", p.Name)
			continue
		}
		s.API.log("Scheduler: Filter → Score → Bind %s 到 %s", p.Name, best)
		// Bind：仅写入节点归属，实际启动留给 kubelet。
		p.Node = best
		s.API.PutPod(p)
	}
}
