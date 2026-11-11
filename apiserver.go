// 步骤 1—2：统一接收资源读写请求，执行简化认证、校验和保存。
package main

import (
	"fmt"
	"sort"
)

type APIServer struct {
	store  Store
	Events []string
}

func NewAPIServer() *APIServer {
	return &APIServer{store: Store{
		Namespaces: map[string]bool{"default": true}, ConfigMaps: map[string]map[string]string{}, Secrets: map[string]map[string]string{}, Claims: map[string]PVC{}, HPAs: map[string]HPA{}, Metrics: map[string]int{},
		Deployments: map[string]Deployment{}, ReplicaSets: map[string]ReplicaSet{}, Pods: map[string]Pod{}, Services: map[string]Service{}, Ingresses: map[string]Ingress{},
	}}
}
func (a *APIServer) log(format string, args ...any) {
	a.Events = append(a.Events, fmt.Sprintf(format, args...))
}

// cloneDeployment 深拷贝探针映射，避免调用者绕过 API 修改已保存状态。
func cloneDeployment(d Deployment) Deployment {
	probes := make(map[string]string, len(d.Probes))
	for k, v := range d.Probes {
		probes[k] = v
	}
	d.Probes = probes
	return d
}

// Apply 对应简化的 kubectl 提交入口；内部组件调用其他读写方法，不模拟真实 RBAC。
func (a *APIServer) Apply(d Deployment, token string) error {
	if token != "demo-admin" {
		return fmt.Errorf("认证/授权失败")
	}
	if d.Name == "" || d.Image == "" || d.Replicas < 0 || d.CPU <= 0 || d.Memory <= 0 {
		return fmt.Errorf("名称、镜像、副本数或 requests 不合法")
	}
	if d.RestartPolicy != "" && d.RestartPolicy != "Always" {
		return fmt.Errorf("本模型仅支持 Always")
	}
	d.Namespace = namespace(d.Namespace)
	if err := a.requireNamespace(d.Namespace); err != nil {
		return err
	}
	d.RestartPolicy = "Always"
	a.store.Deployments[resourceKey(d.Namespace, d.Name)] = cloneDeployment(d)
	a.log("API Server: 保存 Deployment/%s", d.Name)
	return nil
}

// keys 对 map 的键排序，使演示和调度结果可重复。
func keys[T any](m map[string]T) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}
func (a *APIServer) Deployments() []Deployment {
	var out []Deployment
	for _, k := range keys(a.store.Deployments) {
		out = append(out, cloneDeployment(a.store.Deployments[k]))
	}
	return out
}
func (a *APIServer) ReplicaSets() []ReplicaSet {
	var out []ReplicaSet
	for _, k := range keys(a.store.ReplicaSets) {
		r := a.store.ReplicaSets[k]
		r.Spec = cloneDeployment(r.Spec)
		out = append(out, r)
	}
	return out
}
func (a *APIServer) PutReplicaSet(name string, r ReplicaSet) {
	r.Spec = cloneDeployment(r.Spec)
	a.store.ReplicaSets[name] = r
}
func (a *APIServer) Pods() []Pod {
	var out []Pod
	for _, k := range keys(a.store.Pods) {
		p := a.store.Pods[k]
		p.Env = copyValues(p.Env)
		out = append(out, p)
	}
	return out
}
func (a *APIServer) PutPod(p Pod) {
	p.Namespace = namespace(p.Namespace)
	p.Env = copyValues(p.Env)
	a.store.Pods[resourceKey(p.Namespace, p.Name)] = p
	a.log("API Server: Pod/%s node=%s phase=%s ready=%t restarts=%d", p.Name, p.Node, p.Phase, p.Ready, p.Restarts)
}
func (a *APIServer) DeletePod(name string) {
	delete(a.store.Pods, name)
	a.log("API Server: 删除 Pod/%s", name)
}
func (a *APIServer) PutService(name string, s Service) { a.store.Services[name] = s }
func (a *APIServer) PutIngress(name string, i Ingress) { a.store.Ingresses[name] = i }
func (a *APIServer) Service(name string) (Service, bool) {
	s, ok := a.store.Services[name]
	return s, ok
}
func (a *APIServer) Ingress(name string) (Ingress, bool) {
	i, ok := a.store.Ingresses[name]
	return i, ok
}
