// 资源模型：声明期望状态，以及 Pod 的实际运行状态。
package main

type Deployment struct {
	Namespace     string            `json:"namespace"`
	ConfigMap     string            `json:"configMap"`
	Secret        string            `json:"secret"`
	PVC           string            `json:"pvc"`
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	Replicas      int               `json:"replicas"`
	CPU           int               `json:"cpu"`
	Memory        int               `json:"memory"`
	RestartPolicy string            `json:"restartPolicy"`
	Probes        map[string]string `json:"probes"`
}
type ReplicaSet struct {
	Owner string
	Spec  Deployment
}
type Pod struct {
	Namespace, ConfigMap, Secret, PVC    string
	Env                                  map[string]string // 启动时注入的配置快照；Secret 不输出到日志。
	Mounted                              bool              // PVC 是否已挂载。
	Name, Owner, App, Image, Node, Phase string
	CPU, Memory, Restarts                int
	Alive, Ready, ReadinessOK            bool
}
type Service struct{ Selector string }
type Ingress struct{ Host, Path, Service string }
