package main

import "fmt"

// namespace 将省略的 Namespace 归到 default，保持原例子兼容。
func namespace(ns string) string {
	if ns == "" {
		return "default"
	}
	return ns
}

// resourceKey 用空间和名字区分同名资源；default 保持原有短名称。
func resourceKey(ns, name string) string {
	ns = namespace(ns)
	if ns == "default" {
		return name
	}
	return ns + "/" + name
}
func copyValues(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
func (a *APIServer) PutNamespace(name string) { a.store.Namespaces[namespace(name)] = true }
func (a *APIServer) requireNamespace(ns string) error {
	if !a.store.Namespaces[namespace(ns)] {
		return fmt.Errorf("Namespace %s 不存在", ns)
	}
	return nil
}

// PutConfigMap 保存普通配置副本，不与调用方共享 map。
func (a *APIServer) PutConfigMap(ns, name string, data map[string]string) error {
	if err := a.requireNamespace(ns); err != nil {
		return err
	}
	a.store.ConfigMaps[resourceKey(ns, name)] = copyValues(data)
	return nil
}

// PutSecret 模拟敏感配置存储，不打印值；本模型没有加密和细粒度授权。
func (a *APIServer) PutSecret(ns, name string, data map[string]string) error {
	if err := a.requireNamespace(ns); err != nil {
		return err
	}
	a.store.Secrets[resourceKey(ns, name)] = copyValues(data)
	return nil
}

// Prepare 在容器启动/重启时注入配置快照并检查卷，不实现配置热更新。
func (a *APIServer) Prepare(p Pod) (map[string]string, bool, error) {
	env := map[string]string{}
	if p.ConfigMap != "" {
		v, ok := a.store.ConfigMaps[resourceKey(p.Namespace, p.ConfigMap)]
		if !ok {
			return nil, false, fmt.Errorf("ConfigMap 缺失")
		}
		for k, val := range v {
			env[k] = val
		}
	}
	if p.Secret != "" {
		v, ok := a.store.Secrets[resourceKey(p.Namespace, p.Secret)]
		if !ok {
			return nil, false, fmt.Errorf("Secret 缺失")
		}
		for k, val := range v {
			env[k] = val
		}
	}
	mounted := false
	if p.PVC != "" {
		v, ok := a.store.Claims[resourceKey(p.Namespace, p.PVC)]
		if !ok || !v.Bound {
			return nil, false, fmt.Errorf("PVC 未绑定")
		}
		mounted = true
	}
	return env, mounted, nil
}
