package main

import (
	"encoding/json"
	"fmt"
)

// 只建模一个 Namespace 中的一个集群，保留注册、校验、保存、观察的链路。
type CRD struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Spec       struct {
		Group string `json:"group"`
		Names struct {
			Kind string `json:"kind"`
		} `json:"names"`
		Versions []struct {
			Name   string `json:"name"`
			Schema struct {
				OpenAPIV3Schema struct {
					Properties struct {
						Spec struct {
							Properties struct {
								Instances struct {
									Minimum int `json:"minimum"`
									Maximum int `json:"maximum"`
								} `json:"instances"`
							} `json:"properties"`
						} `json:"spec"`
					} `json:"properties"`
				} `json:"openAPIV3Schema"`
			} `json:"schema"`
		} `json:"versions"`
	} `json:"spec"`
}
type InnoDBCluster struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Spec struct {
		Instances int `json:"instances"`
	} `json:"spec"`
}

// Workload 是 Operator 创建的 K8s 工作负载的最小表示。
type Workload struct {
	Kind     string
	Replicas int
}
type API struct {
	definition *CRD
	cluster    *InnoDBCluster
	Workloads  map[string]Workload
	Primary    string // CR status 的最小表示，Operator 观察数据库后上报。
}

func NewAPI() *API { return &API{Workloads: map[string]Workload{}} }

// Apply 模拟 API Server；只解释本示例涉及的 CRD schema，不是通用 OpenAPI 校验器。
func (a *API) Apply(data []byte) error {
	var identity struct {
		Kind string `json:"kind"`
	}
	if err := json.Unmarshal(data, &identity); err != nil {
		return err
	}
	if identity.Kind == "CustomResourceDefinition" {
		var d CRD
		if err := json.Unmarshal(data, &d); err != nil {
			return err
		}
		if d.APIVersion != "apiextensions.k8s.io/v1" || d.Spec.Group == "" || d.Spec.Names.Kind == "" || len(d.Spec.Versions) != 1 {
			return fmt.Errorf("无效 CRD")
		}
		a.definition = &d
		fmt.Println("API Server: 注册类型", d.Spec.Names.Kind)
		return nil
	}
	if a.definition == nil {
		return fmt.Errorf("类型未注册：先提交 CRD")
	}
	var cr InnoDBCluster
	// 本示例只校验类型、版本、名字和 instances 范围。
	if err := json.Unmarshal(data, &cr); err != nil {
		return err
	}
	d := a.definition
	v := d.Spec.Versions[0]
	limits := v.Schema.OpenAPIV3Schema.Properties.Spec.Properties.Instances
	if cr.Kind != d.Spec.Names.Kind || cr.APIVersion != d.Spec.Group+"/"+v.Name {
		return fmt.Errorf("资源类型或版本不匹配")
	}
	if cr.Metadata.Name == "" || cr.Spec.Instances < limits.Minimum || cr.Spec.Instances > limits.Maximum {
		return fmt.Errorf("CR 名称或实例数不符合 schema")
	}
	a.cluster = &cr
	fmt.Println("API Server: 校验并保存 CR", cr.Metadata.Name)
	return nil
}

// Cluster 返回副本，Operator 必须通过 API 观察声明。
func (a *API) Cluster() (InnoDBCluster, bool) {
	if a.cluster == nil {
		return InnoDBCluster{}, false
	}
	return *a.cluster, true
}
func (a *API) EnsureWorkload(name string, w Workload) {
	if a.Workloads[name] == w {
		return
	}
	a.Workloads[name] = w
	fmt.Printf("API Server: 保存 %s/%s replicas=%d\n", w.Kind, name, w.Replicas)
}

// RunWorkloads 压缩模拟内置控制器、调度和 kubelet 的首次启动，不建立数据库复制关系。
func (a *API) RunWorkloads(db *Database) {
	cr, ok := a.Cluster()
	if !ok || len(db.Members) > 0 {
		return
	}
	w, ok := a.Workloads[cr.Metadata.Name+"-mysql"]
	if !ok {
		return
	}
	for i := 0; i < w.Replicas; i++ {
		db.Members = append(db.Members, &Member{Name: fmt.Sprintf("mysql-%d", i), Running: true})
	}
	fmt.Println("K8s: 按工作负载创建 Pod、调度并启动 MySQL 和 Router")
}
