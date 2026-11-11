package main

import (
	"fmt"
	"math"
)

// HPA 只实现 CPU requests 利用率伸缩；省略真实 HPA 的容忍区间和稳定窗口。
type HPA struct {
	Namespace           string // 保存时由 API 设置，目标资源必须在同一空间。
	Target              string
	Min, Max, TargetCPU int
}

func (a *APIServer) PutHPA(ns, name string, h HPA) error {
	if err := a.requireNamespace(ns); err != nil {
		return err
	}
	if h.Target == "" || h.Min < 1 || h.Max < h.Min || h.TargetCPU <= 0 {
		return fmt.Errorf("HPA 配置不合法")
	}
	h.Namespace = namespace(ns)
	a.store.HPAs[resourceKey(ns, name)] = h
	return nil
}

// SetCPU 模拟指标服务采集的数据，单位 m；没有指标时不自行猜测用量。
func (a *APIServer) SetCPU(ns, name string, milli int) error {
	key := resourceKey(ns, name)
	if _, ok := a.store.Pods[key]; !ok || milli < 0 {
		return fmt.Errorf("Pod 不存在或指标不合法")
	}
	a.store.Metrics[key] = milli
	return nil
}
func (a *APIServer) ReconcileHPA() {
	for _, key := range keys(a.store.HPAs) {
		h := a.store.HPAs[key]
		// HPA 与目标 Deployment 必须位于同一 Namespace。
		for _, d := range a.Deployments() {
			if d.Namespace != h.Namespace || d.Name != h.Target {
				continue
			}
			count, used, requested := 0, 0, 0
			missing := false
			for _, p := range a.Pods() {
				if p.Namespace != d.Namespace || p.App != d.Name {
					continue
				}
				if p.Phase != "Running" || !p.Ready {
					continue
				}
				v, ok := a.store.Metrics[resourceKey(p.Namespace, p.Name)]
				if !ok {
					missing = true
					continue
				}
				count++
				used += v
				requested += p.CPU
			}
			desired := d.Replicas
			// 简化策略：忽略未就绪 Pod；就绪 Pod 缺指标则本轮不计算。
			if count > 0 && !missing && requested > 0 {
				desired = int(math.Ceil(float64(d.Replicas) * float64(used) * 100 / float64(requested*h.TargetCPU)))
			}
			if desired < h.Min {
				desired = h.Min
			}
			if desired > h.Max {
				desired = h.Max
			}
			if desired != d.Replicas {
				a.log("HPA: %s/%s 副本 %d → %d", d.Namespace, d.Name, d.Replicas, desired)
				d.Replicas = desired
				_ = a.Apply(d, "demo-admin")
			}
		}
	}
}
