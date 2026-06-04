package featurekit

import (
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/exportertest"
)

func TestFeatureMetricName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		spec FeatureMetricSpec
		want string
	}{
		{
			name: "feature scope",
			spec: FeatureMetricSpec{Scope: MetricScopeFeature, Name: "_value"},
			want: "feature_value",
		},
		{
			name: "namespace scope",
			spec: FeatureMetricSpec{Scope: MetricScopeNamespace, Name: "_value"},
			want: "namespace_value",
		},
		{
			name: "absolute scope",
			spec: FeatureMetricSpec{Scope: MetricScopeAbsolute, Name: "absolute_value"},
			want: "absolute_value",
		},
		{
			name: "unknown scope fallback",
			spec: FeatureMetricSpec{Scope: MetricScope(99), Name: "fallback_value"},
			want: "fallback_value",
		},
	}
	for _, tt := range tests {
		if got := tt.spec.MetricName("feature", "namespace"); got != tt.want {
			t.Fatalf("%s: MetricName() = %q, want %q", tt.name, got, tt.want)
		}
	}

	specs := []FeatureMetricSpec{
		{ID: "known", Scope: MetricScopeFeature, Name: "_known"},
	}
	if got := FeatureMetricName("feature", "namespace", "known", specs); got != "feature_known" {
		t.Fatalf("FeatureMetricName(known) = %q, want feature_known", got)
	}
	if got := FeatureMetricName("feature", "namespace", "missing", specs); got != "missing" {
		t.Fatalf("FeatureMetricName(missing) = %q, want missing", got)
	}
}

func TestLoadFeatureMetricDescriptorsPreservesSpecOrder(t *testing.T) {
	t.Parallel()

	specs := []FeatureMetricSpec{
		{ID: "first", Scope: MetricScopeFeature, Name: "_first", Help: "First metric"},
		{ID: "second", Scope: MetricScopeNamespace, Name: "_second", Help: "Second metric"},
	}
	descriptors := LoadFeatureMetricDescriptors("feature", "namespace", specs)
	ch := make(chan *prometheus.Desc, len(specs))
	descriptors.Describe(ch)
	close(ch)

	got := make([]*prometheus.Desc, 0, len(specs))
	for desc := range ch {
		got = append(got, desc)
	}
	want := []*prometheus.Desc{descriptors.Get("first"), descriptors.Get("second")}
	if len(got) != len(want) {
		t.Fatalf("Describe() emitted %d descriptors, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Describe()[%d] = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestNewFeatureMetricsLoadsDescriptorsAndDelegatesHandlers(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	snapshotter := testSnapshotter{snapshot: testSnapshot{attemptTime: now, success: true, value: 7}}
	logCalls := atomic.Int32{}
	specs := []FeatureMetricSpec{
		{
			ID:    "value",
			Scope: MetricScopeFeature,
			Name:  "_value",
			Help:  "Demo value.",
		},
	}

	collector := NewSnapshotMetricsCollector(SnapshotMetricsCollectorOptions[testSnapshot]{
		SnapshotCollectorOptions: SnapshotCollectorOptions[testSnapshot]{
			FeatureName:     "demo",
			Namespace:       "demo_exporter",
			Snapshotter:     snapshotter,
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
		MetricsFunc: func(ctx SnapshotMetricsContext[testSnapshot]) SnapshotMetrics[testSnapshot] {
			return NewFeatureMetrics(ctx, specs, FeatureMetricHandlers[testSnapshot]{
				Collect: func(ctx FeatureMetricsContext[testSnapshot], ch chan<- prometheus.Metric, snapshot testSnapshot, collectTime time.Time) {
					if ctx.FeatureName != "demo" {
						t.Fatalf("FeatureName = %q, want demo", ctx.FeatureName)
					}
					if ctx.Namespace != "demo_exporter" {
						t.Fatalf("Namespace = %q, want demo_exporter", ctx.Namespace)
					}
					if ctx.Snapshotter != snapshotter {
						t.Fatalf("Snapshotter = %T, want test snapshotter", ctx.Snapshotter)
					}
					if !collectTime.Equal(now) {
						t.Fatalf("collect time = %v, want %v", collectTime, now)
					}
					ch <- prometheus.MustNewConstMetric(ctx.Descriptors.Get("value"), prometheus.GaugeValue, snapshot.value)
				},
				LogError: func(ctx FeatureMetricsContext[testSnapshot], logger *slog.Logger, snapshot testSnapshot) {
					if ctx.FeatureName != "demo" {
						t.Fatalf("log FeatureName = %q, want demo", ctx.FeatureName)
					}
					if logger == nil {
						t.Fatal("logger = nil, want logger")
					}
					if snapshot.value != 7 {
						t.Fatalf("log snapshot value = %v, want 7", snapshot.value)
					}
					logCalls.Add(1)
				},
			})
		},
	})

	families := exportertest.RegisterAndGather(t, collector)
	exportertest.AssertMetricValue(t, families, "demo_value", nil, 7)
	if logCalls.Load() != 1 {
		t.Fatalf("log calls = %d, want 1", logCalls.Load())
	}
}
