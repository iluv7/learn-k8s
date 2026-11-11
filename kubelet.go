// 步骤 6、8：节点只处理分给自己的 Pod，模拟容器启动、重启与状态上报。
package main

type Kubelet struct {
	API  *APIServer
	Node string
}

func (k *Kubelet) Reconcile() {
	for _, p := range k.API.Pods() {
		// 每个节点上的 kubelet 只负责本节点的 Pod。
		if p.Node != k.Node {
			continue
		}
		if p.Phase == "Pending" || !p.Alive {
			// 启动前读取同一 Namespace 的依赖；缺失时等待下一轮，不误报 Ready。
			env, mounted, err := k.API.Prepare(p)
			if err != nil {
				p.Ready = false
				k.API.PutPod(p)
				k.API.log("kubelet: %s 等待依赖: %v", p.Name, err)
				continue
			}
			p.Env = env
			p.Mounted = mounted
			// 容器退出后按 Always 重启，保留 Pod 名字并增加重启计数。
			if !p.Alive {
				p.Restarts++
			}
			k.API.log("kubelet/%s: 模拟 CRI 准备环境、拉取 %s、启动 %s", k.Node, p.Image, p.Name)
			p.Phase = "Running"
			p.Alive = true
		}
		// ReadinessOK 是注入的探针结果；失败只摘除流量，不触发重启。
		p.Ready = p.Alive && p.ReadinessOK
		k.API.PutPod(p)
	}
}
