// 组装组件并驱动一轮协调；顺序执行便于学习，不代表真实集群的同步调用链。
package main

type Cluster struct {
	API         *APIServer
	Controllers ControllerManager
	Scheduler   Scheduler
	Kubelets    []Kubelet
}

func NewCluster() *Cluster {
	a := NewAPIServer()
	nodes := []Node{{"worker-a", 1000, 1024}, {"worker-b", 1000, 1024}}
	c := &Cluster{API: a, Controllers: ControllerManager{API: a}, Scheduler: Scheduler{a, nodes}}
	for _, n := range nodes {
		c.Kubelets = append(c.Kubelets, Kubelet{a, n.Name})
	}
	return c
}
func (c *Cluster) Tick() {
	// HPA 先调整期望副本数，再由已有控制器链路执行。
	c.API.ReconcileHPA()
	c.Controllers.Reconcile()
	c.Scheduler.Reconcile()
	for _, k := range c.Kubelets {
		k.Reconcile()
	}
}
