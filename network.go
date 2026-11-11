// 步骤 7：Service 与 Ingress 的最小数据平面模拟，业务流量不经过 API Server。
package main

import (
	"strings"
)

// Request 模拟 Ingress 域名/路径匹配和 Service 的 Ready 后端选择，不监听真实端口。
func (c *Cluster) Request(host, path string) string {
	return c.RequestInNamespace("default", host, path)
}

// RequestInNamespace 在指定空间查找路由、Service 和 Pod。
func (c *Cluster) RequestInNamespace(ns, host, path string) string {
	ns = namespace(ns)
	i, ok := c.API.Ingress(resourceKey(ns, "java"))
	if !ok || i.Host != host || !strings.HasPrefix(path, i.Path) {
		return "404"
	}
	s, ok := c.API.Service(resourceKey(ns, i.Service))
	if !ok {
		return "503"
	}
	for _, p := range c.API.Pods() {
		if p.Namespace == ns && p.App == s.Selector && p.Ready {
			return "200 Java hello from " + p.Name
		}
	}
	return "503"
}
