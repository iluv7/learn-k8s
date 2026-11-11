package main

import "fmt"

// PVC 模拟已绑定的 RWX 共享卷；数据独立于 Pod，集群进程退出后仍会丢失。
type PVC struct {
	SizeGi int
	Bound  bool
	Files  map[string]string
}

func (a *APIServer) PutPVC(ns, name string, sizeGi int) error {
	if err := a.requireNamespace(ns); err != nil {
		return err
	}
	if sizeGi <= 0 {
		return fmt.Errorf("存储大小必须大于零")
	}
	key := resourceKey(ns, name)
	if _, ok := a.store.Claims[key]; ok {
		return fmt.Errorf("PVC 已存在")
	}
	a.store.Claims[key] = PVC{sizeGi, true, map[string]string{}}
	return nil
}

// WriteFile/ReadFile 用字符串模拟文件内容，不创建宿主机文件，不计算容量占用。
func (a *APIServer) WriteFile(p Pod, path, value string) error {
	actual, ok := a.store.Pods[resourceKey(p.Namespace, p.Name)]
	if !ok || !actual.Mounted {
		return fmt.Errorf("Pod 未挂载存储卷")
	}
	key := resourceKey(actual.Namespace, actual.PVC)
	v, ok := a.store.Claims[key]
	if !ok || !v.Bound {
		return fmt.Errorf("PVC 不可用")
	}
	v.Files[path] = value
	return nil
}
func (a *APIServer) ReadFile(p Pod, path string) (string, error) {
	actual, ok := a.store.Pods[resourceKey(p.Namespace, p.Name)]
	if !ok || !actual.Mounted {
		return "", fmt.Errorf("Pod 未挂载存储卷")
	}
	v, ok := a.store.Claims[resourceKey(actual.Namespace, actual.PVC)]
	if !ok {
		return "", fmt.Errorf("PVC 不存在")
	}
	content, ok := v.Files[path]
	if !ok {
		return "", fmt.Errorf("文件不存在")
	}
	return content, nil
}
