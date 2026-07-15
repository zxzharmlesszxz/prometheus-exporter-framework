package app

import (
	"strings"
	"testing"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
)

func TestExporterInfoFromProjectMetadata(t *testing.T) {
	t.Parallel()

	info := ExporterInfoFromProjectMetadata(ProjectMetadata{
		ExporterName:         "prometheus-demo-exporter",
		ExporterDescription:  "Prometheus Demo Exporter",
		FeatureName:          "demo",
		MetricNamespace:      "demo_exporter",
		DefaultListenAddress: ":9888",
	}, smokeSpecFeature{
		spec: SmokeSpec{
			ServerArgs:    []string{"--demo.target=example.net"},
			WantMetrics:   []string{"demo_exporter_target_up 1"},
			RejectMetrics: []string{"demo_exporter_target_up 0"},
		},
	})

	if info.Name != "prometheus-demo-exporter" {
		t.Fatalf("Name = %q", info.Name)
	}
	if info.Description != "Prometheus Demo Exporter" {
		t.Fatalf("Description = %q", info.Description)
	}
	if info.FeatureName != "demo" {
		t.Fatalf("FeatureName = %q", info.FeatureName)
	}
	if info.MetricNamespace != "demo_exporter" {
		t.Fatalf("MetricNamespace = %q", info.MetricNamespace)
	}
	if info.DefaultListenAddress != ":9888" {
		t.Fatalf("DefaultListenAddress = %q", info.DefaultListenAddress)
	}
	if info.Metrics.BuildInfo != "demo_exporter_build_info" {
		t.Fatalf("Metrics.BuildInfo = %q", info.Metrics.BuildInfo)
	}
	if info.Metrics.CollectionDurationSeconds != "demo_exporter_collection_duration_seconds" {
		t.Fatalf("Metrics.CollectionDurationSeconds = %q", info.Metrics.CollectionDurationSeconds)
	}
	if info.Metrics.LastCollectionSuccess != "demo_exporter_last_collection_success" {
		t.Fatalf("Metrics.LastCollectionSuccess = %q", info.Metrics.LastCollectionSuccess)
	}
	if !hasTestString(info.Smoke.ForbiddenUsageNames, "demo_exporter") {
		t.Fatalf("Smoke.ForbiddenUsageNames = %v", info.Smoke.ForbiddenUsageNames)
	}
	if !hasTestString(info.Smoke.ServerArgs, "--demo.target=example.net") {
		t.Fatalf("Smoke.ServerArgs = %v", info.Smoke.ServerArgs)
	}
	if !hasTestString(info.Smoke.WantMetrics, "demo_exporter_last_collection_success 1") {
		t.Fatalf("Smoke.WantMetrics = %v", info.Smoke.WantMetrics)
	}
	if !hasTestString(info.Smoke.WantMetrics, "demo_exporter_target_up 1") {
		t.Fatalf("Smoke.WantMetrics = %v", info.Smoke.WantMetrics)
	}
	if !hasTestString(info.Smoke.RejectMetrics, "demo_exporter_target_up 0") {
		t.Fatalf("Smoke.RejectMetrics = %v", info.Smoke.RejectMetrics)
	}
}

func TestExporterInfoFromProjectMetadataRequiresValues(t *testing.T) {
	t.Parallel()

	if _, err := ExporterInfoFromProjectMetadataErr(ProjectMetadata{
		ExporterName:         "prometheus-demo-exporter",
		ExporterDescription:  "Prometheus Demo Exporter",
		FeatureName:          "",
		MetricNamespace:      "demo_exporter",
		DefaultListenAddress: ":9888",
	}); err == nil || !strings.Contains(err.Error(), "ProjectMetadata.FeatureName") {
		t.Fatalf("ExporterInfoFromProjectMetadataErr() error = %v, want feature name error", err)
	}

	requirePanicContains(t, "ProjectMetadata.FeatureName", func() {
		_ = ExporterInfoFromProjectMetadata(ProjectMetadata{
			ExporterName:         "prometheus-demo-exporter",
			ExporterDescription:  "Prometheus Demo Exporter",
			FeatureName:          "",
			MetricNamespace:      "demo_exporter",
			DefaultListenAddress: ":9888",
		})
	})
}

func TestExporterInfoFromProjectMetadataAcceptsHostPortListenAddress(t *testing.T) {
	t.Parallel()

	info := ExporterInfoFromProjectMetadata(ProjectMetadata{
		ExporterName:         "prometheus-demo-exporter",
		ExporterDescription:  "Prometheus Demo Exporter",
		FeatureName:          "demo",
		MetricNamespace:      "demo_exporter",
		DefaultListenAddress: "127.0.0.1:9888",
	})
	if info.DefaultListenAddress != "127.0.0.1:9888" {
		t.Fatalf("DefaultListenAddress = %q", info.DefaultListenAddress)
	}
}

func TestExporterInfoFromProjectMetadataRejectsInvalidListenAddress(t *testing.T) {
	t.Parallel()

	requirePanicContains(t, "must be :port or host:port", func() {
		_ = ExporterInfoFromProjectMetadata(ProjectMetadata{
			ExporterName:         "prometheus-demo-exporter",
			ExporterDescription:  "Prometheus Demo Exporter",
			FeatureName:          "demo",
			MetricNamespace:      "demo_exporter",
			DefaultListenAddress: "9888",
		})
	})
}

func TestExporterInfoFromProjectMetadataRejectsInvalidMetricNamespace(t *testing.T) {
	t.Parallel()

	if _, err := ExporterInfoFromProjectMetadataErr(ProjectMetadata{
		ExporterName:         "prometheus-demo-exporter",
		ExporterDescription:  "Prometheus Demo Exporter",
		FeatureName:          "demo",
		MetricNamespace:      "demo-exporter",
		DefaultListenAddress: ":9888",
	}); err == nil || !strings.Contains(err.Error(), `invalid metric namespace "demo-exporter"`) {
		t.Fatalf("ExporterInfoFromProjectMetadataErr() error = %v, want invalid metric namespace error", err)
	}
}

type smokeSpecFeature struct {
	spec SmokeSpec
}

func (f smokeSpecFeature) RegisterFlags(*kingpin.Application) {}

func (f smokeSpecFeature) RegisterCollectors(FeatureContext, *prometheus.Registry) error {
	return nil
}

func (f smokeSpecFeature) SmokeSpec() SmokeSpec {
	return f.spec
}

func requirePanicContains(t *testing.T, want string, fn func()) {
	t.Helper()

	defer func() {
		got := recover()
		if got == nil {
			t.Fatalf("panic = nil, want substring %q", want)
		}
		message, ok := got.(string)
		if !ok {
			t.Fatalf("panic = %T(%v), want string containing %q", got, got, want)
		}
		if !strings.Contains(message, want) {
			t.Fatalf("panic = %q, want substring %q", message, want)
		}
	}()
	fn()
}

func hasTestString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
