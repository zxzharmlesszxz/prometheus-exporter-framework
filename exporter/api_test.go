package exporter_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/version"

	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
)

func TestFacadeConfigAndMetadataHelpers(t *testing.T) {
	t.Parallel()

	feature := exporter.CollectorFeature{
		Name:                      "facade",
		DefaultListenAddressValue: ":9123",
	}

	cfg := exporter.ConfigFromProject(feature)
	if cfg.DefaultListenAddress != ":9123" {
		t.Fatalf("ConfigFromProject().DefaultListenAddress = %q, want :9123", cfg.DefaultListenAddress)
	}

	if got := exporter.ExporterNameFromProject("example.com/team/prometheus-facade-exporter"); got != "facade_exporter" {
		t.Fatalf("ExporterNameFromProject() = %q, want facade_exporter", got)
	}
	if got := exporter.DescriptionFromProject("example.com/team/prometheus-facade-exporter"); got != "Prometheus Facade Exporter" {
		t.Fatalf("DescriptionFromProject() = %q, want Prometheus Facade Exporter", got)
	}

	metrics := exporter.StandardMetricInfo("facade_exporter")
	if metrics.BuildInfo != "facade_exporter_build_info" {
		t.Fatalf("StandardMetricInfo().BuildInfo = %q", metrics.BuildInfo)
	}

	info := exporter.ExporterInfoFromProjectMetadata(exporter.ProjectMetadata{
		ExporterName:         "prometheus-facade-exporter",
		ExporterDescription:  "Prometheus Facade Exporter",
		FeatureName:          "facade",
		MetricNamespace:      "facade_exporter",
		DefaultListenAddress: ":9123",
	}, facadeSmokeFeature{})
	if info.Name != "prometheus-facade-exporter" {
		t.Fatalf("ExporterInfoFromProjectMetadata().Name = %q", info.Name)
	}
	if !hasString(info.Smoke.WantMetrics, "facade_exporter_custom_metric 1") {
		t.Fatalf("ExporterInfoFromProjectMetadata().Smoke.WantMetrics = %v", info.Smoke.WantMetrics)
	}

	info, err := exporter.ExporterInfoFromProjectMetadataErr(exporter.ProjectMetadata{
		ExporterName:         "prometheus-facade-exporter",
		ExporterDescription:  "Prometheus Facade Exporter",
		FeatureName:          "facade",
		MetricNamespace:      "facade_exporter",
		DefaultListenAddress: "127.0.0.1:9123",
	})
	if err != nil {
		t.Fatalf("ExporterInfoFromProjectMetadataErr() error = %v", err)
	}
	if info.DefaultListenAddress != "127.0.0.1:9123" {
		t.Fatalf("ExporterInfoFromProjectMetadataErr().DefaultListenAddress = %q", info.DefaultListenAddress)
	}
	if _, err := exporter.ExporterInfoFromProjectMetadataErr(exporter.ProjectMetadata{
		ExporterName:         "prometheus-facade-exporter",
		ExporterDescription:  "Prometheus Facade Exporter",
		FeatureName:          "facade",
		MetricNamespace:      "facade_exporter",
		DefaultListenAddress: "9123",
	}); err == nil {
		t.Fatal("ExporterInfoFromProjectMetadataErr() error = nil, want invalid listen address error")
	}
}

func TestFacadeCLIAndServerErrors(t *testing.T) {
	preserveVersionMetadata(t)

	if err := exporter.MainErr(exporter.Config{Name: "facade_exporter", DefaultListenAddress: "9888"}); err == nil {
		t.Fatal("MainErr() error = nil, want invalid default listen address error")
	}

	if err := exporter.RunCLIFromProject([]string{"--not-a-real-flag"}); err == nil {
		t.Fatal("RunCLIFromProject() error = nil, want parse error")
	}

	err := exporter.RunCLI(exporter.Config{Name: "facade_exporter"}, []string{"--web.telemetry-path=metrics"})
	if err == nil || !strings.Contains(err.Error(), "invalid --web.telemetry-path") {
		t.Fatalf("RunCLI() error = %v, want telemetry path error", err)
	}

	err = exporter.RunCLIContext(context.Background(), exporter.Config{Name: "facade_exporter"}, []string{"--web.telemetry-path=metrics"})
	if err == nil || !strings.Contains(err.Error(), "invalid --web.telemetry-path") {
		t.Fatalf("RunCLIContext() error = %v, want telemetry path error", err)
	}

	err = exporter.Run(exporter.Options{MetricsPath: "metrics"}, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid metrics path") {
		t.Fatalf("Run() error = %v, want metrics path error", err)
	}

	err = exporter.RunContext(context.Background(), exporter.Options{MetricsPath: "metrics"}, nil)
	if err == nil || !strings.Contains(err.Error(), "invalid metrics path") {
		t.Fatalf("RunContext() error = %v, want metrics path error", err)
	}

	requirePanic(t, func() {
		exporter.MustRun(exporter.Options{MetricsPath: "metrics"}, nil)
	})
}

func TestFacadeHTTPConstructors(t *testing.T) {
	t.Parallel()

	registry := prometheus.NewRegistry()
	if err := exporter.RegisterCollectors(registry, constCollector("facade_handler_value", 7)); err != nil {
		t.Fatalf("RegisterCollectors() error = %v", err)
	}

	handler := exporter.NewHandler(exporter.HandlerOptions{
		Name:        "facade_exporter",
		Description: "Facade exporter",
		MetricsPath: "/facade-metrics",
		Registry:    registry,
	})
	assertServesMetric(t, handler, "/facade-metrics", "facade_handler_value 7")

	if checked, err := exporter.NewHandlerChecked(exporter.HandlerOptions{MetricsPath: "/checked"}); err != nil {
		t.Fatalf("NewHandlerChecked() error = %v, want nil", err)
	} else {
		assertStatus(t, checked, "/healthz", http.StatusOK)
	}

	if _, err := exporter.NewHandlerChecked(exporter.HandlerOptions{MetricsPath: "metrics"}); err == nil {
		t.Fatal("NewHandlerChecked() error = nil, want invalid metrics path error")
	}

	server := exporter.NewServer(exporter.Options{MetricsPath: "/server-metrics"}, registry)
	assertServesMetric(t, server.Handler, "/server-metrics", "facade_handler_value 7")

	if checkedServer, err := exporter.NewServerChecked(exporter.Options{MetricsPath: "/checked-server"}, registry); err != nil {
		t.Fatalf("NewServerChecked() error = %v, want nil", err)
	} else {
		assertServesMetric(t, checkedServer.Handler, "/checked-server", "facade_handler_value 7")
	}

	if srv, err := exporter.NewServerChecked(exporter.Options{MetricsPath: "metrics"}, registry); err == nil || srv != nil {
		t.Fatalf("NewServerChecked() = (%v, %v), want nil server and error", srv, err)
	}
}

func TestFacadeCollectorLifecycleHelpers(t *testing.T) {
	t.Parallel()

	if registry, err := exporter.NewRegistryContext(context.Background(), "facade_context_exporter", nil); err != nil {
		t.Fatalf("NewRegistryContext() error = %v, want nil", err)
	} else if registry == nil {
		t.Fatal("NewRegistryContext() registry = nil, want registry")
	}

	registry := prometheus.NewRegistry()
	collector := &startableCollector{
		collector: constCollector("facade_startable_value", 3),
	}

	if err := exporter.RegisterAndStartCollectors(context.Background(), registry, collector); err != nil {
		t.Fatalf("RegisterAndStartCollectors() error = %v", err)
	}
	if !collector.started {
		t.Fatal("RegisterAndStartCollectors() did not start collector")
	}

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	if !hasMetricFamily(families, "facade_startable_value") {
		t.Fatal("Gather() missing facade_startable_value")
	}

	duplicate := constCollector("facade_duplicate_value", 1)
	if err := exporter.RegisterCollectors(registry, duplicate); err != nil {
		t.Fatalf("RegisterCollectors() initial error = %v", err)
	}
	if err := exporter.RegisterCollectors(registry, duplicate); err == nil {
		t.Fatal("RegisterCollectors() duplicate error = nil, want error")
	}
}

func TestFacadeSnapshotCollector(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	valueDesc := prometheus.NewDesc("facade_snapshot_value", "Facade snapshot value", nil, nil)
	collector := exporter.NewSnapshotCollector(exporter.SnapshotCollectorOptions[facadeSnapshot]{
		Namespace:       "facade_exporter",
		Snapshotter:     facadeSnapshotter{snapshot: facadeSnapshot{AttemptTime: now, Success: true, Value: 11}},
		RefreshInterval: time.Hour,
		StatusFunc: func(snapshot facadeSnapshot) exporter.SnapshotStatus {
			return exporter.SnapshotStatus{
				AttemptTime: snapshot.AttemptTime,
				Success:     snapshot.Success,
			}
		},
		DescribeFunc: func(ch chan<- *prometheus.Desc) {
			ch <- valueDesc
		},
		CollectFunc: func(ch chan<- prometheus.Metric, snapshot facadeSnapshot, _ time.Time) {
			ch <- prometheus.MustNewConstMetric(valueDesc, prometheus.GaugeValue, snapshot.Value)
		},
		Now: func() time.Time { return now },
	})

	registry := prometheus.NewRegistry()
	if err := registry.Register(collector); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	if !hasMetricFamily(families, "facade_snapshot_value") {
		t.Fatal("Gather() missing facade_snapshot_value")
	}
	if !hasMetricFamily(families, "facade_exporter_last_collection_success") {
		t.Fatal("Gather() missing facade_exporter_last_collection_success")
	}
}

func TestFacadeValueAndFileHelpers(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/input.txt"
	if err := os.WriteFile(path, []byte("ok"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if got := exporter.FileMTimeSeconds(path); got == 0 {
		t.Fatal("FileMTimeSeconds() = 0, want non-zero mtime")
	}

	if exporter.BoolFloat(true) != 1 || exporter.BoolFloat(false) != 0 {
		t.Fatal("BoolFloat() returned unexpected values")
	}
	if got := exporter.UnixTimestamp(time.Unix(42, 0)); got != 42 {
		t.Fatalf("UnixTimestamp() = %v, want 42", got)
	}
	if got := exporter.NormalizeDuration(0, time.Minute); got != time.Minute {
		t.Fatalf("NormalizeDuration() = %v, want 1m", got)
	}
}

func TestFacadeVersionHelpers(t *testing.T) {
	preserveVersionMetadata(t)

	v, b, r := exporter.ResolveVersionMetadata("", "", "", "abc123", "(devel)", "", "unknown")
	if v != "dev" || b != "dev" || r != "abc123" {
		t.Fatalf("ResolveVersionMetadata() = (%q, %q, %q), want (dev, dev, abc123)", v, b, r)
	}

	version.Version = "api-version"
	version.Branch = "api-branch"
	version.Revision = "api-revision"
	exporter.HydrateVersionMetadata()
	if version.Version != "api-version" || version.Branch != "api-branch" || version.Revision != "api-revision" {
		t.Fatalf("HydrateVersionMetadata() changed explicit version metadata to (%q, %q, %q)", version.Version, version.Branch, version.Revision)
	}
}

func TestFacadeInjectedWrappersPanicWithoutMetadata(t *testing.T) {
	tests := []struct {
		name string
		fn   func()
	}{
		{name: "InjectedExporterName", fn: func() { _ = exporter.InjectedExporterName() }},
		{name: "InjectedExporterDescription", fn: func() { _ = exporter.InjectedExporterDescription() }},
		{name: "InjectedFeatureName", fn: func() { _ = exporter.InjectedFeatureName() }},
		{name: "InjectedMetricNamespace", fn: func() { _ = exporter.InjectedMetricNamespace() }},
		{name: "InjectedDefaultListenAddress", fn: func() { _ = exporter.InjectedDefaultListenAddress() }},
		{name: "InjectedProjectMetadata", fn: func() { _ = exporter.InjectedProjectMetadata() }},
		{name: "ConfigFromInjectedProject", fn: func() { _ = exporter.ConfigFromInjectedProject() }},
		{name: "ExporterInfoFromInjectedProject", fn: func() { _ = exporter.ExporterInfoFromInjectedProject() }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			requirePanic(t, tc.fn)
		})
	}
}

func TestFacadeInjectedErrWrappersReturnErrorWithoutMetadata(t *testing.T) {
	tests := []struct {
		name string
		fn   func() error
	}{
		{name: "InjectedProjectMetadataErr", fn: func() error {
			_, err := exporter.InjectedProjectMetadataErr()
			return err
		}},
		{name: "ConfigFromInjectedProjectErr", fn: func() error {
			_, err := exporter.ConfigFromInjectedProjectErr()
			return err
		}},
		{name: "MainFromInjectedProjectErr", fn: func() error {
			return exporter.MainFromInjectedProjectErr()
		}},
		{name: "ExporterInfoFromInjectedProjectErr", fn: func() error {
			_, err := exporter.ExporterInfoFromInjectedProjectErr()
			return err
		}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.fn(); err == nil {
				t.Fatal("error = nil, want missing injected metadata error")
			}
		})
	}
}

type facadeSmokeFeature struct{}

func (facadeSmokeFeature) RegisterFlags(*kingpin.Application) {}

func (facadeSmokeFeature) RegisterCollectors(exporter.FeatureContext, *prometheus.Registry) error {
	return nil
}

func (facadeSmokeFeature) SmokeSpec() exporter.SmokeSpec {
	return exporter.SmokeSpec{
		WantMetrics: []string{"facade_exporter_custom_metric 1"},
	}
}

type facadeSnapshot struct {
	AttemptTime time.Time
	Success     bool
	Value       float64
}

type facadeSnapshotter struct {
	snapshot facadeSnapshot
}

func (s facadeSnapshotter) Snapshot(context.Context, time.Time) facadeSnapshot {
	return s.snapshot
}

type testCollector struct {
	desc  *prometheus.Desc
	value float64
}

func constCollector(name string, value float64) prometheus.Collector {
	return testCollector{
		desc:  prometheus.NewDesc(name, "Facade test value.", nil, nil),
		value: value,
	}
}

func (c testCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c testCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, c.value)
}

type startableCollector struct {
	collector prometheus.Collector
	started   bool
}

func (c *startableCollector) Describe(ch chan<- *prometheus.Desc) {
	c.collector.Describe(ch)
}

func (c *startableCollector) Collect(ch chan<- prometheus.Metric) {
	c.collector.Collect(ch)
}

func (c *startableCollector) Start(ctx context.Context) {
	c.started = ctx != nil
}

func assertServesMetric(t *testing.T, handler http.Handler, path string, want string) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d, want %d", path, rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), want) {
		t.Fatalf("GET %s body missing %q: %s", path, want, rec.Body.String())
	}
}

func assertStatus(t *testing.T, handler http.Handler, path string, want int) {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != want {
		t.Fatalf("GET %s status = %d, want %d", path, rec.Code, want)
	}
}

func requirePanic(t *testing.T, fn func()) {
	t.Helper()

	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatal("function did not panic")
		}
	}()
	fn()
}

func preserveVersionMetadata(t *testing.T) {
	t.Helper()

	originalVersion := version.Version
	originalBranch := version.Branch
	originalRevision := version.Revision
	t.Cleanup(func() {
		version.Version = originalVersion
		version.Branch = originalBranch
		version.Revision = originalRevision
	})
}

func hasString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
