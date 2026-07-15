package featurekit

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/exportertest"
)

type testConfig struct {
	target string
}

type testSnapshot struct {
	attemptTime time.Time
	success     bool
	value       float64
}

type testSnapshotter struct {
	snapshot testSnapshot
}

func (s testSnapshotter) Snapshot(context.Context, time.Time) testSnapshot {
	return s.snapshot
}

type testStartableCollector struct {
	desc   *prometheus.Desc
	value  float64
	starts *atomic.Int32
	ctx    context.Context
}

func newTestStartableCollector(value float64, starts *atomic.Int32) *testStartableCollector {
	return &testStartableCollector{
		desc:   prometheus.NewDesc("demo_value", "Demo value.", nil, nil),
		value:  value,
		starts: starts,
	}
}

func (c *testStartableCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c *testStartableCollector) Collect(ch chan<- prometheus.Metric) {
	ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, c.value)
}

func (c *testStartableCollector) Start(ctx context.Context) {
	c.ctx = ctx
	c.starts.Add(1)
}

func TestFeatureRegistersFlagsRuntimeConfigAndCollectors(t *testing.T) {
	t.Parallel()

	var starts atomic.Int32
	feature := NewFeature(FeatureSpec[testConfig, testSnapshot]{
		FeatureName:             "demo",
		FallbackRefreshInterval: time.Minute,
		Config:                  testConfig{target: "default"},
		RegisterFlagsFunc: func(app *kingpin.Application, ctx FlagContext, config *testConfig) {
			app.Flag(ctx.FeatureName+".target", "Demo target.").Default(config.target).StringVar(&config.target)
		},
		NewSnapshotterFunc: func(ctx CollectorContext[testConfig]) (framework.Snapshotter[testSnapshot], error) {
			if ctx.FeatureName != "demo" {
				t.Fatalf("FeatureName = %q, want demo", ctx.FeatureName)
			}
			if ctx.Config.target != "node-a" {
				t.Fatalf("target = %q, want node-a", ctx.Config.target)
			}
			if ctx.RefreshInterval != 30*time.Second {
				t.Fatalf("RefreshInterval = %v, want 30s", ctx.RefreshInterval)
			}
			return testSnapshotter{snapshot: testSnapshot{success: true, value: 7}}, nil
		},
		NewCollectorFunc: func(featureName string, namespace string, logger *slog.Logger, snapshotter framework.Snapshotter[testSnapshot], refreshInterval time.Duration) framework.StartableCollector {
			if featureName != "demo" {
				t.Fatalf("collector featureName = %q, want demo", featureName)
			}
			if namespace != "demo_exporter" {
				t.Fatalf("namespace = %q, want demo_exporter", namespace)
			}
			if logger == nil {
				t.Fatal("logger = nil, want logger")
			}
			if snapshotter == nil {
				t.Fatal("snapshotter = nil, want snapshotter")
			}
			if refreshInterval != 30*time.Second {
				t.Fatalf("collector refreshInterval = %v, want 30s", refreshInterval)
			}
			return newTestStartableCollector(7, &starts)
		},
		RuntimeConfigFunc: func(ctx RuntimeConfigContext[testConfig]) []any {
			return []any{"target", ctx.Config.target}
		},
		SmokeFunc: func(ctx SmokeContext[testConfig]) SmokeSpec {
			return SmokeSpec{
				ServerArgs:  []string{"--" + ctx.FeatureName + ".target=" + ctx.Config.target},
				WantMetrics: []string{ctx.FeatureName + "_value 1"},
			}
		},
	})
	if got := feature.FeatureName(); got != "demo" {
		t.Fatalf("FeatureName() = %q, want demo", got)
	}

	app := kingpin.New("test", "")
	app.Terminate(func(int) {})
	feature.RegisterFlags(app)
	if _, err := app.Parse([]string{"--demo.refresh-interval=30s", "--demo.target=node-a"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	config := feature.RuntimeConfig()
	if got := exportertest.RuntimeConfigValue(t, config, "refresh_interval"); got != 30*time.Second {
		t.Fatalf("refresh_interval = %v, want 30s", got)
	}
	if got := exportertest.RuntimeConfigValue(t, config, "target"); got != "node-a" {
		t.Fatalf("target = %v, want node-a", got)
	}

	registry := prometheus.NewRegistry()
	err := feature.RegisterCollectors(framework.FeatureContext{
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		Namespace: "demo_exporter",
	}, registry)
	if err != nil {
		t.Fatalf("RegisterCollectors() error = %v", err)
	}
	if got := starts.Load(); got != 1 {
		t.Fatalf("collector starts = %d, want 1", got)
	}
	exportertest.WaitForMetricValue(t, registry, "demo_value", nil, 7)

	smoke := feature.SmokeSpec()
	if got := smoke.ServerArgs; len(got) != 1 || got[0] != "--demo.target=node-a" {
		t.Fatalf("SmokeSpec().ServerArgs = %v, want --demo.target=node-a", got)
	}
	if got := smoke.WantMetrics; len(got) != 1 || got[0] != "demo_value 1" {
		t.Fatalf("SmokeSpec().WantMetrics = %v, want demo_value 1", got)
	}
}

func TestFeatureReportsValidationError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("invalid config")
	feature := NewFeature(FeatureSpec[testConfig, testSnapshot]{
		FeatureName:             "demo",
		FallbackRefreshInterval: time.Minute,
		ValidateConfigFunc: func(testConfig) error {
			return wantErr
		},
		NewCollectorFunc: func(string, string, *slog.Logger, framework.Snapshotter[testSnapshot], time.Duration) framework.StartableCollector {
			t.Fatal("NewCollectorFunc was called")
			return nil
		},
	})

	err := feature.RegisterCollectors(framework.FeatureContext{}, prometheus.NewRegistry())
	if !errors.Is(err, wantErr) {
		t.Fatalf("RegisterCollectors() error = %v, want %v", err, wantErr)
	}
}

func TestFeatureRejectsNilCollectorFromFactory(t *testing.T) {
	t.Parallel()

	feature := NewFeature(FeatureSpec[testConfig, testSnapshot]{
		FeatureName: "demo",
		NewCollectorFunc: func(string, string, *slog.Logger, framework.Snapshotter[testSnapshot], time.Duration) framework.StartableCollector {
			return nil
		},
	})

	err := feature.RegisterCollectors(framework.FeatureContext{Namespace: "demo_exporter"}, prometheus.NewRegistry())
	if err == nil {
		t.Fatal("RegisterCollectors() error = nil, want nil collector error")
	}
	if got, want := err.Error(), "create demo collector: collector factory returned nil"; got != want {
		t.Fatalf("RegisterCollectors() error = %q, want %q", got, want)
	}
}

func TestFeaturePreparesConfigBeforeValidationAndSnapshotter(t *testing.T) {
	t.Parallel()

	var validated bool
	feature := NewFeature(FeatureSpec[testConfig, testSnapshot]{
		FeatureName: "demo",
		Config:      testConfig{target: "raw"},
		PrepareConfigFunc: func(featureName string, config testConfig) (testConfig, error) {
			if featureName != "demo" {
				t.Fatalf("featureName = %q, want demo", featureName)
			}
			if config.target != "raw" {
				t.Fatalf("prepare config target = %q, want raw", config.target)
			}
			config.target = "prepared"
			return config, nil
		},
		ValidateConfigFunc: func(config testConfig) error {
			validated = true
			if config.target != "prepared" {
				t.Fatalf("validate config target = %q, want prepared", config.target)
			}
			return nil
		},
		NewSnapshotterFunc: func(ctx CollectorContext[testConfig]) (framework.Snapshotter[testSnapshot], error) {
			if !validated {
				t.Fatal("NewSnapshotterFunc called before ValidateConfigFunc")
			}
			if ctx.Config.target != "prepared" {
				t.Fatalf("collector config target = %q, want prepared", ctx.Config.target)
			}
			return testSnapshotter{snapshot: testSnapshot{success: true, value: 3}}, nil
		},
		NewCollectorFunc: func(string, string, *slog.Logger, framework.Snapshotter[testSnapshot], time.Duration) framework.StartableCollector {
			return newTestStartableCollector(3, &atomic.Int32{})
		},
	})

	if err := feature.RegisterCollectors(framework.FeatureContext{Namespace: "demo_exporter"}, prometheus.NewRegistry()); err != nil {
		t.Fatalf("RegisterCollectors() error = %v", err)
	}
	if !validated {
		t.Fatal("ValidateConfigFunc was not called")
	}
}

func TestFeatureCachesPreparedConfigBetweenRuntimeConfigAndCollectors(t *testing.T) {
	t.Parallel()

	prepareCalls := atomic.Int32{}
	feature := NewFeature(FeatureSpec[testConfig, testSnapshot]{
		FeatureName: "demo",
		Config:      testConfig{target: "raw"},
		PrepareConfigFunc: func(featureName string, config testConfig) (testConfig, error) {
			prepareCalls.Add(1)
			config.target = "prepared"
			return config, nil
		},
		RuntimeConfigFunc: func(ctx RuntimeConfigContext[testConfig]) []any {
			return []any{"target", ctx.Config.target}
		},
		NewSnapshotterFunc: func(ctx CollectorContext[testConfig]) (framework.Snapshotter[testSnapshot], error) {
			if ctx.Config.target != "prepared" {
				t.Fatalf("collector config target = %q, want prepared", ctx.Config.target)
			}
			return testSnapshotter{snapshot: testSnapshot{success: true, value: 4}}, nil
		},
		NewCollectorFunc: func(string, string, *slog.Logger, framework.Snapshotter[testSnapshot], time.Duration) framework.StartableCollector {
			return newTestStartableCollector(4, &atomic.Int32{})
		},
	})

	if got := exportertest.RuntimeConfigValue(t, feature.RuntimeConfig(), "target"); got != "prepared" {
		t.Fatalf("runtime target = %v, want prepared", got)
	}
	if err := feature.RegisterCollectors(framework.FeatureContext{Namespace: "demo_exporter"}, prometheus.NewRegistry()); err != nil {
		t.Fatalf("RegisterCollectors() error = %v", err)
	}
	if got := prepareCalls.Load(); got != 1 {
		t.Fatalf("prepare calls = %d, want 1", got)
	}
}

func TestFeatureCachesPreparedConfigBetweenCollectorsAndRuntimeConfig(t *testing.T) {
	t.Parallel()

	prepareCalls := atomic.Int32{}
	feature := NewFeature(FeatureSpec[testConfig, testSnapshot]{
		FeatureName: "demo",
		Config:      testConfig{target: "raw"},
		PrepareConfigFunc: func(featureName string, config testConfig) (testConfig, error) {
			prepareCalls.Add(1)
			config.target = "prepared"
			return config, nil
		},
		RuntimeConfigFunc: func(ctx RuntimeConfigContext[testConfig]) []any {
			return []any{"target", ctx.Config.target}
		},
		NewSnapshotterFunc: func(ctx CollectorContext[testConfig]) (framework.Snapshotter[testSnapshot], error) {
			if ctx.Config.target != "prepared" {
				t.Fatalf("collector config target = %q, want prepared", ctx.Config.target)
			}
			return testSnapshotter{snapshot: testSnapshot{success: true, value: 4}}, nil
		},
		NewCollectorFunc: func(string, string, *slog.Logger, framework.Snapshotter[testSnapshot], time.Duration) framework.StartableCollector {
			return newTestStartableCollector(4, &atomic.Int32{})
		},
	})

	if err := feature.RegisterCollectors(framework.FeatureContext{Namespace: "demo_exporter"}, prometheus.NewRegistry()); err != nil {
		t.Fatalf("RegisterCollectors() error = %v", err)
	}
	if got := exportertest.RuntimeConfigValue(t, feature.RuntimeConfig(), "target"); got != "prepared" {
		t.Fatalf("runtime target = %v, want prepared", got)
	}
	if got := prepareCalls.Load(); got != 1 {
		t.Fatalf("prepare calls = %d, want 1", got)
	}
}

func TestFeatureRuntimeConfigReportsPrepareError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("bad config")
	feature := NewFeature(FeatureSpec[testConfig, testSnapshot]{
		FeatureName: "demo",
		Config:      testConfig{target: "raw"},
		PrepareConfigFunc: func(string, testConfig) (testConfig, error) {
			return testConfig{target: "ignored"}, wantErr
		},
		RuntimeConfigFunc: func(ctx RuntimeConfigContext[testConfig]) []any {
			return []any{"target", ctx.Config.target}
		},
	})

	config := feature.RuntimeConfig()
	if got := exportertest.RuntimeConfigValue(t, config, "config_error"); got != wantErr.Error() {
		t.Fatalf("config_error = %v, want %q", got, wantErr.Error())
	}
	if got := exportertest.RuntimeConfigValue(t, config, "target"); got != "raw" {
		t.Fatalf("target = %v, want raw fallback config", got)
	}
}

func TestFeatureRegisterCollectorsStartsCollectorWithFeatureContext(t *testing.T) {
	t.Parallel()

	type contextKey struct{}
	startContext := context.WithValue(context.Background(), contextKey{}, "feature-context")
	starts := atomic.Int32{}
	startable := newTestStartableCollector(5, &starts)
	feature := NewFeature(FeatureSpec[testConfig, testSnapshot]{
		FeatureName: "demo",
		NewCollectorFunc: func(string, string, *slog.Logger, framework.Snapshotter[testSnapshot], time.Duration) framework.StartableCollector {
			return startable
		},
	})

	if err := feature.RegisterCollectors(framework.FeatureContext{Context: startContext, Namespace: "demo_exporter"}, prometheus.NewRegistry()); err != nil {
		t.Fatalf("RegisterCollectors() error = %v", err)
	}
	if starts.Load() != 1 {
		t.Fatalf("starts = %d, want 1", starts.Load())
	}
	if startable.ctx != startContext {
		t.Fatal("collector Start() did not receive FeatureContext.Context")
	}
}

func TestFeatureDefaultsNoopCollectorAndStaticSmokeSpec(t *testing.T) {
	t.Parallel()

	feature := NewFeature(FeatureSpec[testConfig, testSnapshot]{
		Smoke: SmokeSpec{
			ServerArgs:    []string{"--demo.target=node-a"},
			WantMetrics:   []string{"demo_value 1"},
			RejectMetrics: []string{"demo_value 0"},
		},
	})
	if got := feature.FeatureName(); got != "exporter" {
		t.Fatalf("FeatureName() = %q, want exporter", got)
	}

	app := kingpin.New("test", "")
	app.Terminate(func(int) {})
	feature.RegisterFlags(app)
	if _, err := app.Parse(nil); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	config := feature.RuntimeConfig()
	if got := exportertest.RuntimeConfigValue(t, config, "refresh_interval"); got != framework.DefaultSnapshotRefreshInterval {
		t.Fatalf("refresh_interval = %v, want %v", got, framework.DefaultSnapshotRefreshInterval)
	}
	if err := feature.RegisterCollectors(framework.FeatureContext{}, prometheus.NewRegistry()); err != nil {
		t.Fatalf("RegisterCollectors() error = %v, want nil", err)
	}

	smoke := feature.SmokeSpec()
	if len(smoke.ServerArgs) != 1 || smoke.ServerArgs[0] != "--demo.target=node-a" {
		t.Fatalf("SmokeSpec().ServerArgs = %v, want static server args", smoke.ServerArgs)
	}
	if len(smoke.WantMetrics) != 1 || smoke.WantMetrics[0] != "demo_value 1" {
		t.Fatalf("SmokeSpec().WantMetrics = %v, want static wanted metric", smoke.WantMetrics)
	}
	if len(smoke.RejectMetrics) != 1 || smoke.RejectMetrics[0] != "demo_value 0" {
		t.Fatalf("SmokeSpec().RejectMetrics = %v, want static rejected metric", smoke.RejectMetrics)
	}
}

func TestFeatureReportsSnapshotterError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("snapshotter failed")
	feature := NewFeature(FeatureSpec[testConfig, testSnapshot]{
		FeatureName:             "demo",
		FallbackRefreshInterval: time.Minute,
		NewSnapshotterFunc: func(CollectorContext[testConfig]) (framework.Snapshotter[testSnapshot], error) {
			return nil, wantErr
		},
		NewCollectorFunc: func(string, string, *slog.Logger, framework.Snapshotter[testSnapshot], time.Duration) framework.StartableCollector {
			t.Fatal("NewCollectorFunc was called")
			return nil
		},
	})

	err := feature.RegisterCollectors(framework.FeatureContext{}, prometheus.NewRegistry())
	if !errors.Is(err, wantErr) {
		t.Fatalf("RegisterCollectors() error = %v, want %v", err, wantErr)
	}
}
