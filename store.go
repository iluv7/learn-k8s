// 资源存储定义。此处没有实现真实 etcd 的持久化和一致性协议。
package main

// Store 用内存模拟 etcd；仅由 API Server 读写，退出后数据丢失。
type Store struct {
	Namespaces  map[string]bool
	ConfigMaps  map[string]map[string]string
	Secrets     map[string]map[string]string
	Claims      map[string]PVC
	HPAs        map[string]HPA
	Metrics     map[string]int // 每个 Pod 的模拟 CPU 使用量，单位 m。
	Deployments map[string]Deployment
	ReplicaSets map[string]ReplicaSet
	Pods        map[string]Pod
	Services    map[string]Service
	Ingresses   map[string]Ingress
}
