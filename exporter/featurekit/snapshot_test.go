package featurekit

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/exportertest"
)

func TestSnapshotEngineFactory(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	factory := SnapshotEngineFactory[testConfig, testSnapshot](func(ctx CollectorContext[testConfig]) (SnapshotEngine[testSnapshot], error) {
		if ctx.FeatureName != "demo" {
			t.Fatalf("FeatureName = %q, want demo", ctx.FeatureName)
		}
		return SnapshotEngineFunc[testSnapshot](func(_ context.Context, _ time.Time) testSnapshot {
			return testSnapshot{attemptTime: now, success: true, value: float64(len(ctx.Config.target))}
		}), nil
	})

	engine, err := factory(CollectorContext[testConfig]{
		FeatureName: "demo",
		Config:      testConfig{target: "hello"},
	})
	if err != nil {
		t.Fatalf("factory() error = %v", err)
	}

	snapshot := engine.Snapshot(context.Background(), now)
	if snapshot.value != 5 {
		t.Fatalf("snapshot value = %v, want 5 (len of 'hello')", snapshot.value)
	}
}

func TestNewSnapshotFeatureSpecDefaultsFallbackRefreshInterval(t *testing.T) {
	t.Parallel()

	spec := NewSnapshotFeatureSpec(SnapshotFeatureSpec[testConfig, testSnapshot]{
		Options: SpecOptions{
			FeatureName: "demo",
		},
	})

	if spec.FallbackRefreshInterval != framework.DefaultSnapshotRefreshInterval {
		t.Fatalf("FallbackRefreshInterval = %v, want %v", spec.FallbackRefreshInterval, framework.DefaultSnapshotRefreshInterval)
	}
}

func TestNewSnapshotFeatureSpecPrefersSpecDefaultRefreshInterval(t *testing.T) {
	t.Parallel()

	spec := NewSnapshotFeatureSpec(SnapshotFeatureSpec[testConfig, testSnapshot]{
		Options: SpecOptions{
			FeatureName: "demo",
		},
		DefaultRefreshInterval: 90 * time.Second,
	})

	if spec.DefaultRefreshInterval != 90*time.Second {
		t.Fatalf("DefaultRefreshInterval = %v, want 90s", spec.DefaultRefreshInterval)
	}
}

func TestResolveSnapshotCollectorOptionsNilLoggerDefaults(t *testing.T) {
	t.Parallel()

	options := ResolveSnapshotCollectorOptions(SnapshotCollectorOptions[testSnapshot]{
		FeatureName: "demo",
		Namespace:   "demo_exporter",
		Snapshotter: testSnapshotter{},
	})
	if options.Logger == nil {
		t.Fatal("Logger = nil, want slog.Default()")
	}
}

func TestNewSnapshotCollectorDefaultsToZeroSnapshotter(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	collector := NewSnapshotCollector(SnapshotCollectorOptions[testSnapshot]{
		FeatureName:     "demo",
		Namespace:       "demo_exporter",
		RefreshInterval: time.Minute,
		StatusFunc: func(snapshot testSnapshot) framework.SnapshotStatus {
			return framework.SnapshotStatus{AttemptTime: snapshot.attemptTime, Success: snapshot.success}
		},
		Now: func() time.Time {
			return now
		},
	})

	// With zero snapshotter, the collector still produces collection health metrics.
	families := exportertest.RegisterAndGather(t, collector)
	exportertest.AssertMetricValue(t, families, "demo_exporter_last_collection_success", nil, 0)
	exportertest.AssertMetricValue(t, families, "demo_exporter_last_collection_timestamp_seconds", nil, 0)
}

func TestNewSnapshotCollectorWithNilStatusFunc(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	collector := NewSnapshotCollector(SnapshotCollectorOptions[testSnapshot]{
		FeatureName:     "demo",
		Namespace:       "demo_exporter",
		Snapshotter:     testSnapshotter{snapshot: testSnapshot{attemptTime: now}},
		RefreshInterval: time.Minute,
		Now: func() time.Time {
			return now
		},
	})

	families := exportertest.RegisterAndGather(t, collector)
	exportertest.AssertMetricValue(t, families, "demo_exporter_last_collection_success", nil, 0)
}

func TestNewSnapshotCollectorWithNilCollectFunc(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	collector := NewSnapshotCollector(SnapshotCollectorOptions[testSnapshot]{
		FeatureName:     "demo",
		Namespace:       "demo_exporter",
		Snapshotter:     testSnapshotter{snapshot: testSnapshot{attemptTime: now, success: true}},
		RefreshInterval: time.Minute,
		StatusFunc: func(snapshot testSnapshot) framework.SnapshotStatus {
			return framework.SnapshotStatus{AttemptTime: snapshot.attemptTime, Success: snapshot.success}
		},
		Now: func() time.Time {
			return now
		},
	})

	families := exportertest.RegisterAndGather(t, collector)
	exportertest.AssertMetricValue(t, families, "demo_exporter_last_collection_success", nil, 1)
}

func TestSnapshotEngineFunc(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)

	engine := SnapshotEngineFunc[testSnapshot](func(ctx context.Context, snapshotTime time.Time) testSnapshot {
		if ctx == nil {
			t.Fatal("ctx = nil, want context")
		}
		return testSnapshot{attemptTime: snapshotTime, success: true, value: 42}
	})

	snapshot := engine.Snapshot(context.Background(), now)
	if snapshot.attemptTime != now || !snapshot.success || snapshot.value != 42 {
		t.Fatalf("Snapshot() = %+v, want attemptTime %v success true value 42", snapshot, now)
	}
}

func TestResolveSnapshotCollectorOptionsDefaults(t *testing.T) {
	t.Parallel()

	snapshotter := testSnapshotter{}
	options := ResolveSnapshotCollectorOptions(SnapshotCollectorOptions[testSnapshot]{
		DefaultFeatureName:     "demo",
		DefaultMetricNamespace: "demo_exporter",
		DefaultSnapshotter:     snapshotter,
		DefaultRefreshInterval: time.Minute,
	})

	if options.FeatureName != "demo" {
		t.Fatalf("FeatureName = %q, want demo", options.FeatureName)
	}
	if options.Namespace != "demo_exporter" {
		t.Fatalf("Namespace = %q, want demo_exporter", options.Namespace)
	}
	if options.Snapshotter != snapshotter {
		t.Fatal("Snapshotter was not defaulted")
	}
	if options.RefreshInterval != time.Minute {
		t.Fatalf("RefreshInterval = %v, want 1m", options.RefreshInterval)
	}
}

func TestNewSnapshotCollectorBuildsFrameworkCollector(t *testing.T) {
	t.Parallel()

	now := time.Unix(1700000000, 0)
	valueDesc := prometheus.NewDesc("demo_value", "Demo value.", nil, nil)
	collector := NewSnapshotCollector(SnapshotCollectorOptions[testSnapshot]{
		FeatureName:     "demo",
		Namespace:       "demo_exporter",
		Snapshotter:     testSnapshotter{snapshot: testSnapshot{attemptTime: now, success: true, value: 7}},
		RefreshInterval: time.Minute,
		StatusFunc: func(snapshot testSnapshot) framework.SnapshotStatus {
			return framework.SnapshotStatus{
				AttemptTime: snapshot.attemptTime,
				Success:     snapshot.success,
			}
		},
		DescribeFunc: func(ch chan<- *prometheus.Desc) {
			ch <- valueDesc
		},
		CollectFunc: func(ch chan<- prometheus.Metric, snapshot testSnapshot, _ time.Time) {
			ch <- prometheus.MustNewConstMetric(valueDesc, prometheus.GaugeValue, snapshot.value)
		},
		Now: func() time.Time {
			return now
		},
	})

	expected := `
# HELP demo_exporter_last_collection_success Whether the last demo data collection succeeded
# TYPE demo_exporter_last_collection_success gauge
demo_exporter_last_collection_success 1
# HELP demo_exporter_last_collection_timestamp_seconds Unix timestamp of the last demo data collection attempt
# TYPE demo_exporter_last_collection_timestamp_seconds gauge
demo_exporter_last_collection_timestamp_seconds 1.7e+09
# HELP demo_exporter_last_successful_collection_timestamp_seconds Unix timestamp of the last successful demo data collection
# TYPE demo_exporter_last_successful_collection_timestamp_seconds gauge
demo_exporter_last_successful_collection_timestamp_seconds 1.7e+09
# HELP demo_value Demo value.
# TYPE demo_value gauge
demo_value 7
`

	if err := testutil.CollectAndCompare(collector, strings.NewReader(expected),
		"demo_value",
		"demo_exporter_last_collection_success",
		"demo_exporter_last_collection_timestamp_seconds",
		"demo_exporter_last_successful_collection_timestamp_seconds",
	); err != nil {
		t.Fatalf("CollectAndCompare() error = %v", err)
	}
}

func TestNewSnapshotFeatureSpecWiresSnapshotCollector(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	logCalls := atomic.Int32{}
	feature := NewFeature(NewSnapshotFeatureSpec(SnapshotFeatureSpec[testConfig, testSnapshot]{
		Options: SpecOptions{
			FeatureName:            "demo",
			DefaultRefreshInterval: 45 * time.Second,
		},
		DefaultRefreshInterval: time.Minute,
		Config:                 testConfig{target: "default"},
		RegisterFlagsFunc: func(app *kingpin.Application, ctx FlagContext, config *testConfig) {
			if ctx.FeatureName != "demo" {
				t.Fatalf("flag FeatureName = %q, want demo", ctx.FeatureName)
			}
			if ctx.DefaultRefreshInterval != 45*time.Second {
				t.Fatalf("flag DefaultRefreshInterval = %v, want 45s", ctx.DefaultRefreshInterval)
			}
			app.Flag(ctx.FeatureName+".target", "Demo target.").Default(config.target).StringVar(&config.target)
		},
		ValidateConfigFunc: func(config testConfig) error {
			if config.target != "node-a" {
				t.Fatalf("target = %q, want node-a", config.target)
			}
			return nil
		},
		NewSnapshotterFunc: func(ctx CollectorContext[testConfig]) (framework.Snapshotter[testSnapshot], error) {
			if ctx.FeatureName != "demo" {
				t.Fatalf("collector FeatureName = %q, want demo", ctx.FeatureName)
			}
			if ctx.Framework.Namespace != "demo_exporter" {
				t.Fatalf("Framework.Namespace = %q, want demo_exporter", ctx.Framework.Namespace)
			}
			if ctx.Config.target != "node-a" {
				t.Fatalf("collector target = %q, want node-a", ctx.Config.target)
			}
			if ctx.RefreshInterval != 30*time.Second {
				t.Fatalf("RefreshInterval = %v, want 30s", ctx.RefreshInterval)
			}
			return testSnapshotter{snapshot: testSnapshot{attemptTime: now, success: true, value: 9}}, nil
		},
		MetricsFunc: func(ctx SnapshotMetricsContext[testSnapshot]) SnapshotMetrics[testSnapshot] {
			if ctx.FeatureName != "demo" {
				t.Fatalf("metrics FeatureName = %q, want demo", ctx.FeatureName)
			}
			if ctx.Namespace != "demo_exporter" {
				t.Fatalf("metrics Namespace = %q, want demo_exporter", ctx.Namespace)
			}
			if ctx.Snapshotter == nil {
				t.Fatal("metrics Snapshotter = nil, want snapshotter")
			}
			return testSnapshotMetrics{
				desc:     prometheus.NewDesc("demo_observed_value", "Demo observed value.", nil, nil),
				logCalls: &logCalls,
			}
		},
		StatusFunc: func(snapshot testSnapshot) framework.SnapshotStatus {
			return framework.SnapshotStatus{
				AttemptTime: snapshot.attemptTime,
				Success:     snapshot.success,
			}
		},
		RuntimeConfigFunc: func(ctx RuntimeConfigContext[testConfig]) []any {
			return []any{"target", ctx.Config.target}
		},
		Smoke: SmokeSpec{
			WantMetrics: []string{"demo_observed_value 9"},
		},
	}))

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
	exportertest.WaitForMetricValue(t, registry, "demo_observed_value", nil, 9)
	exportertest.WaitForMetricValue(t, registry, "demo_exporter_last_collection_success", nil, 1)
	if logCalls.Load() == 0 {
		t.Fatal("LogSnapshotError was not called")
	}

	smoke := feature.SmokeSpec()
	if len(smoke.WantMetrics) != 1 || smoke.WantMetrics[0] != "demo_observed_value 9" {
		t.Fatalf("SmokeSpec().WantMetrics = %v, want demo_observed_value 9", smoke.WantMetrics)
	}
}

func TestNewSnapshotMetricsCollectorUsesExplicitErrorLogFunc(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	explicitLogCalls := atomic.Int32{}
	metricsLogCalls := atomic.Int32{}
	collector := NewSnapshotMetricsCollector(SnapshotMetricsCollectorOptions[testSnapshot]{
		SnapshotCollectorOptions: SnapshotCollectorOptions[testSnapshot]{
			FeatureName:     "demo",
			Namespace:       "demo_exporter",
			Snapshotter:     testSnapshotter{snapshot: testSnapshot{attemptTime: now, success: true, value: 11}},
			RefreshInterval: time.Minute,
			StatusFunc: func(snapshot testSnapshot) framework.SnapshotStatus {
				return framework.SnapshotStatus{
					AttemptTime: snapshot.attemptTime,
					Success:     snapshot.success,
				}
			},
			ErrorLogFunc: func(logger *slog.Logger, snapshot testSnapshot) {
				if logger == nil {
					t.Fatal("logger = nil, want default logger")
				}
				if snapshot.value != 11 {
					t.Fatalf("snapshot value = %v, want 11", snapshot.value)
				}
				explicitLogCalls.Add(1)
			},
			Now: func() time.Time {
				return now
			},
		},
		MetricsFunc: func(SnapshotMetricsContext[testSnapshot]) SnapshotMetrics[testSnapshot] {
			return testSnapshotMetrics{
				desc:     prometheus.NewDesc("demo_explicit_value", "Demo explicit value.", nil, nil),
				logCalls: &metricsLogCalls,
			}
		},
	})

	families := exportertest.RegisterAndGather(t, collector)
	exportertest.AssertMetricValue(t, families, "demo_explicit_value", nil, 11)
	if explicitLogCalls.Load() != 1 {
		t.Fatalf("explicit log calls = %d, want 1", explicitLogCalls.Load())
	}
	if metricsLogCalls.Load() != 0 {
		t.Fatalf("metrics log calls = %d, want 0", metricsLogCalls.Load())
	}
}

func TestNewSnapshotMetricsDefaultsToNoopMetrics(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	collector := NewSnapshotMetricsCollector(SnapshotMetricsCollectorOptions[testSnapshot]{
		SnapshotCollectorOptions: SnapshotCollectorOptions[testSnapshot]{
			FeatureName:     "demo",
			Namespace:       "demo_exporter",
			Snapshotter:     testSnapshotter{snapshot: testSnapshot{attemptTime: now, success: true, value: 13}},
			RefreshInterval: time.Minute,
			StatusFunc: func(snapshot testSnapshot) framework.SnapshotStatus {
				return framework.SnapshotStatus{
					AttemptTime: snapshot.attemptTime,
					Success:     snapshot.success,
				}
			},
			Now: func() time.Time {
				return now
			},
		},
	})

	families := exportertest.RegisterAndGather(t, collector)
	exportertest.AssertMetricValue(t, families, "demo_exporter_last_collection_success", nil, 1)
	if _, ok := exportertest.MetricValue(families, "demo_observed_value", nil); ok {
		t.Fatal("demo_observed_value was exported, want only collection metrics")
	}
}

func TestNewSnapshotMetricsFallsBackWhenFactoryReturnsNil(t *testing.T) {
	t.Parallel()

	metrics := newSnapshotMetrics(SnapshotMetricsContext[testSnapshot]{
		FeatureName: "demo",
		Namespace:   "demo_exporter",
	}, func(SnapshotMetricsContext[testSnapshot]) SnapshotMetrics[testSnapshot] {
		return nil
	})

	descCh := make(chan *prometheus.Desc, 1)
	metrics.Describe(descCh)
	if len(descCh) != 0 {
		t.Fatalf("Describe emitted %d descriptors, want 0", len(descCh))
	}

	metricCh := make(chan prometheus.Metric, 1)
	metrics.Collect(metricCh, testSnapshot{}, time.Now())
	if len(metricCh) != 0 {
		t.Fatalf("Collect emitted %d metrics, want 0", len(metricCh))
	}
}

type testSnapshotMetrics struct {
	desc     *prometheus.Desc
	logCalls *atomic.Int32
}

func (m testSnapshotMetrics) Describe(ch chan<- *prometheus.Desc) {
	ch <- m.desc
}

func (m testSnapshotMetrics) Collect(ch chan<- prometheus.Metric, snapshot testSnapshot, _ time.Time) {
	ch <- prometheus.MustNewConstMetric(m.desc, prometheus.GaugeValue, snapshot.value)
}

func (m testSnapshotMetrics) LogSnapshotError(_ *slog.Logger, _ testSnapshot) {
	if m.logCalls != nil {
		m.logCalls.Add(1)
	}
}
