package main

import (
	"strings"
	"testing"
)

func fixture(t *testing.T) *Cluster {
	t.Helper()
	c := NewCluster()
	if err := c.API.Apply(Deployment{Name: "java", Image: "java:demo", Replicas: 3, CPU: 500, Memory: 256}, "demo-admin"); err != nil {
		t.Fatal(err)
	}
	c.Tick()
	return c
}
func TestReconcileAndRecovery(t *testing.T) {
	c := fixture(t)
	if len(c.API.Pods()) != 3 {
		t.Fatal("expected three replicas")
	}
	for _, p := range c.API.Pods() {
		if !p.Ready || p.Node == "" {
			t.Fatal("pod not scheduled and ready")
		}
	}
	p := c.API.Pods()[0]
	p.Alive = false
	c.API.PutPod(p)
	c.Tick()
	found := false
	for _, actual := range c.API.Pods() {
		if actual.Name == p.Name {
			found = true
			if actual.Restarts != 1 || !actual.Ready {
				t.Fatal("container not restarted")
			}
		}
	}
	if !found {
		t.Fatal("restart replaced pod")
	}
	c.API.DeletePod(p.Name)
	c.Tick()
	if len(c.API.Pods()) != 3 {
		t.Fatal("replicas not replenished")
	}
	for _, actual := range c.API.Pods() {
		if actual.Name == p.Name {
			t.Fatal("deleted identity reused")
		}
	}
	c.Tick()
	if len(c.API.Pods()) != 3 {
		t.Fatal("reconcile not idempotent")
	}
}
func TestCapacityAndScaleDown(t *testing.T) {
	c := fixture(t)
	d := c.API.Deployments()[0]
	d.Replicas = 5
	if err := c.API.Apply(d, "demo-admin"); err != nil {
		t.Fatal(err)
	}
	c.Tick()
	pending := 0
	for _, p := range c.API.Pods() {
		if p.Node == "" {
			pending++
		}
	}
	if pending != 1 {
		t.Fatalf("pending=%d", pending)
	}
	d.Replicas = 0
	_ = c.API.Apply(d, "demo-admin")
	c.Tick()
	if len(c.API.Pods()) != 0 {
		t.Fatal("scale to zero failed")
	}
	d.Replicas = 3
	_ = c.API.Apply(d, "demo-admin")
	c.Tick()
	for _, p := range c.API.Pods() {
		if !p.Ready {
			t.Fatal("resources not released")
		}
	}
}
func TestIngressAndReadiness(t *testing.T) {
	c := fixture(t)
	c.API.PutService("java", Service{"java"})
	c.API.PutIngress("java", Ingress{"java.local", "/", "java"})
	if !strings.HasPrefix(c.Request("java.local", "/hello"), "200") {
		t.Fatal("routing failed")
	}
	if c.Request("wrong.local", "/") != "404" {
		t.Fatal("host matching failed")
	}
	for _, p := range c.API.Pods() {
		p.ReadinessOK = false
		c.API.PutPod(p)
	}
	c.Tick()
	if c.Request("java.local", "/") != "503" {
		t.Fatal("unready endpoints received traffic")
	}
	for _, p := range c.API.Pods() {
		if p.Restarts != 0 {
			t.Fatal("readiness failure restarted container")
		}
	}
}
func TestValidationAndIsolation(t *testing.T) {
	c := fixture(t)
	d := c.API.Deployments()[0]
	if c.API.Apply(d, "bad") == nil {
		t.Fatal("accepted bad token")
	}
	d.CPU = 0
	if c.API.Apply(d, "demo-admin") == nil {
		t.Fatal("accepted invalid requests")
	}
	d = c.API.Deployments()[0]
	d.Probes = map[string]string{"readiness": "/ready"}
	_ = c.API.Apply(d, "demo-admin")
	d.Probes["readiness"] = "changed"
	if c.API.Deployments()[0].Probes["readiness"] != "/ready" {
		t.Fatal("store aliases caller map")
	}
	p := c.API.Pods()[0]
	p.Ready = false
	if !c.API.Pods()[0].Ready {
		t.Fatal("read mutated store")
	}
}
