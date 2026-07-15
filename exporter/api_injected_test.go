package exporter

import (
	"strings"
	"testing"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
)

func TestFacadeInjectedMetadataUsesRootPackageVariables(t *testing.T) {
	withFacadeInjectedMetadata(t, ProjectMetadata{
		ExporterName:         "prometheus-facade-exporter",
		ExporterDescription:  "Prometheus Facade Exporter",
		FeatureName:          "facade",
		MetricNamespace:      "facade_exporter",
		DefaultListenAddress: ":9123",
	})

	if InjectedExporterName() != "prometheus-facade-exporter" {
		t.Fatalf("InjectedExporterName() = %q", InjectedExporterName())
	}
	if InjectedExporterDescription() != "Prometheus Facade Exporter" {
		t.Fatalf("InjectedExporterDescription() = %q", InjectedExporterDescription())
	}
	if InjectedFeatureName() != "facade" {
		t.Fatalf("InjectedFeatureName() = %q", InjectedFeatureName())
	}
	if InjectedMetricNamespace() != "facade_exporter" {
		t.Fatalf("InjectedMetricNamespace() = %q", InjectedMetricNamespace())
	}
	if InjectedDefaultListenAddress() != ":9123" {
		t.Fatalf("InjectedDefaultListenAddress() = %q", InjectedDefaultListenAddress())
	}

	cfg := ConfigFromInjectedProject(CollectorFeature{Name: "facade"})
	if cfg.Name != "prometheus-facade-exporter" {
		t.Fatalf("ConfigFromInjectedProject().Name = %q", cfg.Name)
	}
	if cfg.Namespace != "facade_exporter" {
		t.Fatalf("ConfigFromInjectedProject().Namespace = %q", cfg.Namespace)
	}
	if cfg.DefaultListenAddress != ":9123" {
		t.Fatalf("ConfigFromInjectedProject().DefaultListenAddress = %q", cfg.DefaultListenAddress)
	}
	cfg, err := ConfigFromInjectedProjectErr(CollectorFeature{Name: "facade"})
	if err != nil {
		t.Fatalf("ConfigFromInjectedProjectErr() error = %v", err)
	}
	if cfg.Name != "prometheus-facade-exporter" {
		t.Fatalf("ConfigFromInjectedProjectErr().Name = %q", cfg.Name)
	}

	info := ExporterInfoFromInjectedProject(facadeInjectedSmokeFeature{})
	if info.Name != "prometheus-facade-exporter" {
		t.Fatalf("ExporterInfoFromInjectedProject().Name = %q", info.Name)
	}
	if !hasFacadeTestString(info.Smoke.WantMetrics, "facade_exporter_custom_metric 1") {
		t.Fatalf("ExporterInfoFromInjectedProject().Smoke.WantMetrics = %v", info.Smoke.WantMetrics)
	}
	info, err = ExporterInfoFromInjectedProjectErr(facadeInjectedSmokeFeature{})
	if err != nil {
		t.Fatalf("ExporterInfoFromInjectedProjectErr() error = %v", err)
	}
	if info.Name != "prometheus-facade-exporter" {
		t.Fatalf("ExporterInfoFromInjectedProjectErr().Name = %q", info.Name)
	}
}

func TestFacadeInjectedMetadataErrRejectsInvalidListenAddress(t *testing.T) {
	withFacadeInjectedMetadata(t, ProjectMetadata{
		ExporterName:         "prometheus-facade-exporter",
		ExporterDescription:  "Prometheus Facade Exporter",
		FeatureName:          "facade",
		MetricNamespace:      "facade_exporter",
		DefaultListenAddress: "9123",
	})

	if _, err := InjectedProjectMetadataErr(); err == nil {
		t.Fatal("InjectedProjectMetadataErr() error = nil, want invalid listen address error")
	} else if got := err.Error(); !strings.Contains(got, "invalid Makefile-injected exporter metadata") || !strings.Contains(got, "must be :port or host:port") {
		t.Fatalf("InjectedProjectMetadataErr() error = %q, want injected listen address context", got)
	}

	if _, err := ConfigFromInjectedProjectErr(); err == nil {
		t.Fatal("ConfigFromInjectedProjectErr() error = nil, want invalid listen address error")
	}
	if _, err := ExporterInfoFromInjectedProjectErr(); err == nil {
		t.Fatal("ExporterInfoFromInjectedProjectErr() error = nil, want invalid listen address error")
	}
}

func TestFacadeInjectedMetadataErrRejectsInvalidMetricNamespace(t *testing.T) {
	withFacadeInjectedMetadata(t, ProjectMetadata{
		ExporterName:         "prometheus-facade-exporter",
		ExporterDescription:  "Prometheus Facade Exporter",
		FeatureName:          "facade",
		MetricNamespace:      "facade-exporter",
		DefaultListenAddress: ":9123",
	})

	if _, err := InjectedProjectMetadataErr(); err == nil {
		t.Fatal("InjectedProjectMetadataErr() error = nil, want invalid metric namespace error")
	} else if got := err.Error(); !strings.Contains(got, "invalid Makefile-injected exporter metadata") || !strings.Contains(got, `invalid metric namespace "facade-exporter"`) {
		t.Fatalf("InjectedProjectMetadataErr() error = %q, want injected metric namespace context", got)
	}

	if _, err := ConfigFromInjectedProjectErr(); err == nil {
		t.Fatal("ConfigFromInjectedProjectErr() error = nil, want invalid metric namespace error")
	}
	if _, err := ExporterInfoFromInjectedProjectErr(); err == nil {
		t.Fatal("ExporterInfoFromInjectedProjectErr() error = nil, want invalid metric namespace error")
	}
	requireFacadePanicContains(t, "invalid Makefile-injected exporter metadata", func() {
		_ = InjectedMetricNamespace()
	})
}

func withFacadeInjectedMetadata(t *testing.T, metadata ProjectMetadata) {
	t.Helper()

	oldExporterName := injectedExporterName
	oldExporterDescription := injectedExporterDescription
	oldFeatureName := injectedFeatureName
	oldMetricNamespace := injectedMetricNamespace
	oldListenAddress := injectedListenAddress
	t.Cleanup(func() {
		injectedExporterName = oldExporterName
		injectedExporterDescription = oldExporterDescription
		injectedFeatureName = oldFeatureName
		injectedMetricNamespace = oldMetricNamespace
		injectedListenAddress = oldListenAddress
	})

	injectedExporterName = metadata.ExporterName
	injectedExporterDescription = metadata.ExporterDescription
	injectedFeatureName = metadata.FeatureName
	injectedMetricNamespace = metadata.MetricNamespace
	injectedListenAddress = metadata.DefaultListenAddress
}

type facadeInjectedSmokeFeature struct{}

func (facadeInjectedSmokeFeature) RegisterFlags(*kingpin.Application) {}

func (facadeInjectedSmokeFeature) RegisterCollectors(FeatureContext, *prometheus.Registry) error {
	return nil
}

func (facadeInjectedSmokeFeature) SmokeSpec() SmokeSpec {
	return SmokeSpec{
		WantMetrics: []string{"facade_exporter_custom_metric 1"},
	}
}

func hasFacadeTestString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func requireFacadePanicContains(t *testing.T, want string, fn func()) {
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
