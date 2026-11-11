// ReplicaSet 控制器：维持声明的 Pod 副本数，删除后补建的新 Pod 使用新名字。
package main

import (
	"fmt"
)

func (c *ControllerManager) reconcileReplicaSet() {
	// 步骤 4、9：比较副本数量，缩容时删除，缺少时创建；不因未就绪而重复补副本。
	for _, r := range c.API.ReplicaSets() {
		owner := r.Owner + "-rs"
		var pods []Pod
		for _, p := range c.API.Pods() {
			if p.Owner == owner && p.Namespace == r.Spec.Namespace {
				pods = append(pods, p)
			}
		}
		for i := r.Spec.Replicas; i < len(pods); i++ {
			c.API.DeletePod(resourceKey(pods[i].Namespace, pods[i].Name))
		}
		for i := len(pods); i < r.Spec.Replicas; i++ {
			c.sequence++
			d := r.Spec
			c.API.PutPod(Pod{Namespace: d.Namespace, ConfigMap: d.ConfigMap, Secret: d.Secret, PVC: d.PVC, Name: fmt.Sprintf("%s-%d", owner, c.sequence), Owner: owner, App: d.Name, Image: d.Image, CPU: d.CPU, Memory: d.Memory, Phase: "Pending", Alive: true, ReadinessOK: true})
		}
	}
}
