package main

import (
	"testing"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"

	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/exportertest"
)

func TestFeatureRegistersDemoCollector(t *testing.T) {
	t.Parallel()

	feature := &Feature{}
	app := kingpin.New("test", "test")
	feature.RegisterFlags(app)
	if _, err := app.Parse([]string{"--demo.target=node-a"}); err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}

	registry := prometheus.NewRegistry()
	err := feature.RegisterCollectors(framework.FeatureContext{Namespace: "demo_exporter"}, registry)
	if err != nil {
		t.Fatalf("RegisterCollectors() error = %v, want nil", err)
	}

	families := exportertest.Gather(t, registry)
	metric := exportertest.MetricFamily(t, families, "demo_exporter_demo_info")
	if got := metric.GetMetric()[0].GetGauge().GetValue(); got != 1 {
		t.Fatalf("demo_exporter_demo_info = %v, want 1", got)
	}
	if got := metric.GetMetric()[0].GetLabel()[0].GetValue(); got != "node-a" {
		t.Fatalf("demo_exporter_demo_info target = %q, want %q", got, "node-a")
	}
}

func TestFeatureRuntimeConfig(t *testing.T) {
	t.Parallel()

	feature := &Feature{}
	app := kingpin.New("test", "test")
	feature.RegisterFlags(app)
	if _, err := app.Parse([]string{"--demo.target=node-b"}); err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}

	runtimeConfig := feature.RuntimeConfig()
	if len(runtimeConfig) != 2 || runtimeConfig[0] != "demo_target" || runtimeConfig[1] != "node-b" {
		t.Fatalf("RuntimeConfig() = %v, want [demo_target node-b]", runtimeConfig)
	}
}
