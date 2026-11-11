// 控制器管理入口：真实 Kubernetes 中控制器通过各自队列独立运行。
package main

type ControllerManager struct {
	API      *APIServer
	sequence int
}

// Reconcile 模拟 Controller Manager 驱动两个独立的控制器进行协调。
func (c *ControllerManager) Reconcile() {
	c.reconcileDeployment()
	c.reconcileReplicaSet()
}
