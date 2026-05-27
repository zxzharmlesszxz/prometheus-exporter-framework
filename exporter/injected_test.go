package exporter

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/exporter-toolkit/web"
)

func TestInjectedProjectMetadata(t *testing.T) {
	withInjectedMetadata(t, ProjectMetadata{
		ExporterName:         "prometheus-demo-exporter",
		ExporterDescription:  "Prometheus Demo Exporter",
		FeatureName:          "demo",
		MetricNamespace:      "demo_exporter",
		DefaultListenAddress: ":9888",
	})

	metadata := InjectedProjectMetadata()
	if metadata.ExporterName != "prometheus-demo-exporter" {
		t.Fatalf("ExporterName = %q", metadata.ExporterName)
	}
	if metadata.ExporterDescription != "Prometheus Demo Exporter" {
		t.Fatalf("ExporterDescription = %q", metadata.ExporterDescription)
	}
	if metadata.FeatureName != "demo" {
		t.Fatalf("FeatureName = %q", metadata.FeatureName)
	}
	if metadata.MetricNamespace != "demo_exporter" {
		t.Fatalf("MetricNamespace = %q", metadata.MetricNamespace)
	}
	if metadata.DefaultListenAddress != ":9888" {
		t.Fatalf("DefaultListenAddress = %q", metadata.DefaultListenAddress)
	}
}

func TestConfigFromInjectedProject(t *testing.T) {
	withInjectedMetadata(t, ProjectMetadata{
		ExporterName:         "prometheus-demo-exporter",
		ExporterDescription:  "Prometheus Demo Exporter",
		FeatureName:          "demo",
		MetricNamespace:      "demo_exporter",
		DefaultListenAddress: ":9888",
	})

	feature := CollectorFeature{Name: "demo"}
	cfg := ConfigFromInjectedProject(feature)
	if cfg.Name != "prometheus-demo-exporter" {
		t.Fatalf("Name = %q", cfg.Name)
	}
	if cfg.Namespace != "demo_exporter" {
		t.Fatalf("Namespace = %q", cfg.Namespace)
	}
	if cfg.Description != "Prometheus Demo Exporter" {
		t.Fatalf("Description = %q", cfg.Description)
	}
	if cfg.DefaultListenAddress != ":9888" {
		t.Fatalf("DefaultListenAddress = %q", cfg.DefaultListenAddress)
	}
	if len(cfg.Features) != 1 {
		t.Fatalf("Features length = %d, want 1", len(cfg.Features))
	}
}

func TestMainFromInjectedProject(t *testing.T) {
	preserveVersionMetadata(t)
	withInjectedMetadata(t, ProjectMetadata{
		ExporterName:         "prometheus-demo-exporter",
		ExporterDescription:  "Prometheus Demo Exporter",
		FeatureName:          "demo",
		MetricNamespace:      "demo_exporter",
		DefaultListenAddress: ":9888",
	})

	originalArgs := os.Args
	os.Args = []string{
		"/usr/local/bin/renamed-demo-exporter",
		"--log.level=error",
	}
	t.Cleanup(func() {
		os.Args = originalArgs
	})

	feature := CollectorFeature{
		Name: "demo",
		CollectorsFunc: func(ctx FeatureContext) ([]prometheus.Collector, error) {
			if ctx.Namespace != "demo_exporter" {
				t.Fatalf("FeatureContext.Namespace = %q, want demo_exporter", ctx.Namespace)
			}
			return []prometheus.Collector{
				newConstCollector(ctx.Namespace+"_injected_value", "Injected value", 3),
			}, nil
		},
	}

	called := false
	stubListenAndServe(t, func(srv *http.Server, flags *web.FlagConfig, logger *slog.Logger) error {
		called = true
		if flags == nil {
			t.Fatal("ToolkitFlags = nil, want toolkit flags")
		}
		if logger == nil {
			t.Fatal("logger = nil, want logger")
		}

		req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
		rec := httptest.NewRecorder()
		srv.Handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET /metrics status = %d, want %d", rec.Code, http.StatusOK)
		}
		if !strings.Contains(rec.Body.String(), "demo_exporter_injected_value 3") {
			t.Fatalf("GET /metrics body missing injected feature metric: %s", rec.Body.String())
		}

		req = httptest.NewRequest(http.MethodGet, "/", nil)
		rec = httptest.NewRecorder()
		srv.Handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET / status = %d, want %d", rec.Code, http.StatusOK)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "renamed-demo-exporter") {
			t.Fatalf("GET / body missing executable name: %s", body)
		}
		if !strings.Contains(body, "Prometheus Demo Exporter") {
			t.Fatalf("GET / body missing injected description: %s", body)
		}
		return nil
	})

	MainFromInjectedProject(feature)
	if !called {
		t.Fatal("listenAndServe was not called")
	}
}

func TestExporterInfoFromInjectedProject(t *testing.T) {
	withInjectedMetadata(t, ProjectMetadata{
		ExporterName:         "prometheus-demo-exporter",
		ExporterDescription:  "Prometheus Demo Exporter",
		FeatureName:          "demo",
		MetricNamespace:      "demo_exporter",
		DefaultListenAddress: ":9888",
	})

	info := ExporterInfoFromInjectedProject(smokeSpecFeature{
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
	if !hasTestString(info.Smoke.ServerArgs, "--demo.target=example.net") {
		t.Fatalf("Smoke.ServerArgs = %v", info.Smoke.ServerArgs)
	}
	if !hasTestString(info.Smoke.WantMetrics, "demo_exporter_target_up 1") {
		t.Fatalf("Smoke.WantMetrics = %v", info.Smoke.WantMetrics)
	}
	if !hasTestString(info.Smoke.RejectMetrics, "demo_exporter_target_up 0") {
		t.Fatalf("Smoke.RejectMetrics = %v", info.Smoke.RejectMetrics)
	}
}

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
	if info.Metrics.BuildInfo != "demo_exporter_build_info" {
		t.Fatalf("Metrics.BuildInfo = %q", info.Metrics.BuildInfo)
	}
	if info.Metrics.LastCollectionSuccess != "demo_exporter_last_collection_success" {
		t.Fatalf("Metrics.LastCollectionSuccess = %q", info.Metrics.LastCollectionSuccess)
	}
	if !hasTestString(info.Smoke.ForbiddenUsageNames, "demo_exporter") {
		t.Fatalf("Smoke.ForbiddenUsageNames = %v", info.Smoke.ForbiddenUsageNames)
	}
	if !hasTestString(info.Smoke.ServerArgs, "--demo.refresh-interval=100ms") {
		t.Fatalf("Smoke.ServerArgs = %v", info.Smoke.ServerArgs)
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

func TestInjectedProjectMetadataRequiresValues(t *testing.T) {
	withInjectedMetadata(t, ProjectMetadata{
		ExporterName:         "prometheus-demo-exporter",
		ExporterDescription:  "Prometheus Demo Exporter",
		FeatureName:          "",
		MetricNamespace:      "demo_exporter",
		DefaultListenAddress: ":9888",
	})

	requirePanicContains(t, "injectedFeatureName", func() {
		_ = InjectedProjectMetadata()
	})
}

func TestInjectedProjectMetadataRequiresColonListenAddress(t *testing.T) {
	withInjectedMetadata(t, ProjectMetadata{
		ExporterName:         "prometheus-demo-exporter",
		ExporterDescription:  "Prometheus Demo Exporter",
		FeatureName:          "demo",
		MetricNamespace:      "demo_exporter",
		DefaultListenAddress: "9888",
	})

	requirePanicContains(t, "must start with :", func() {
		_ = InjectedProjectMetadata()
	})
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

func withInjectedMetadata(t *testing.T, metadata ProjectMetadata) {
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
