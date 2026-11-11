// Deployment 控制器：本示例只维护一个 ReplicaSet，省略滚动升级。
package main

func (c *ControllerManager) reconcileDeployment() {
	// 步骤 3：将 Deployment 的期望配置传递给 ReplicaSet，不直接创建容器。
	for _, d := range c.API.Deployments() {
		c.API.PutReplicaSet(resourceKey(d.Namespace, d.Name+"-rs"), ReplicaSet{d.Name, d})
	}
}
