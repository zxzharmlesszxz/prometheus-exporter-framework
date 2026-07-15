package feature

import (
	"context"
	"errors"
	"testing"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
)

func TestCollectorFeatureRegisterFlags(t *testing.T) {
	t.Parallel()

	called := false
	feature := CollectorFeature{
		RegisterFlagsFunc: func(app *kingpin.Application) {
			called = true
			app.Flag("demo.value", "Demo value").Default("default").String()
		},
	}

	app := kingpin.New("test", "test")
	feature.RegisterFlags(app)

	if !called {
		t.Fatal("RegisterFlagsFunc was not called")
	}
	if _, err := app.Parse([]string{"--demo.value=custom"}); err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}
}

func TestCollectorFeatureRegisterCollectorsUsesCustomFunc(t *testing.T) {
	t.Parallel()

	customCalled := false
	collectorsCalled := false
	feature := CollectorFeature{
		CollectorsFunc: func(ctx FeatureContext) ([]prometheus.Collector, error) {
			collectorsCalled = true
			return nil, nil
		},
		RegisterCollectorsFunc: func(ctx FeatureContext, registry *prometheus.Registry) error {
			customCalled = true
			if ctx.Namespace != "custom_exporter" {
				t.Fatalf("FeatureContext.Namespace = %q, want %q", ctx.Namespace, "custom_exporter")
			}
			return registry.Register(newConstCollector("custom_feature_value", "Custom feature value", 3))
		},
	}

	registry := prometheus.NewRegistry()
	if err := feature.RegisterCollectors(FeatureContext{Namespace: "custom_exporter"}, registry); err != nil {
		t.Fatalf("RegisterCollectors() error = %v, want nil", err)
	}
	if !customCalled {
		t.Fatal("RegisterCollectorsFunc was not called")
	}
	if collectorsCalled {
		t.Fatal("CollectorsFunc was called despite RegisterCollectorsFunc being set")
	}

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v, want nil", err)
	}
	if !hasMetricFamily(families, "custom_feature_value") {
		t.Fatal("Gather() missing custom_feature_value")
	}
}

func TestCollectorFeatureRegisterCollectorsReturnsCollectorsFuncError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("collectors failed")
	feature := CollectorFeature{
		CollectorsFunc: func(ctx FeatureContext) ([]prometheus.Collector, error) {
			return nil, wantErr
		},
	}

	err := feature.RegisterCollectors(FeatureContext{}, prometheus.NewRegistry())
	if !errors.Is(err, wantErr) {
		t.Fatalf("RegisterCollectors() error = %v, want %v", err, wantErr)
	}
}

func TestCollectorFeatureRegisterCollectorsNoopWithoutFuncs(t *testing.T) {
	t.Parallel()

	feature := CollectorFeature{}
	if err := feature.RegisterCollectors(FeatureContext{}, prometheus.NewRegistry()); err != nil {
		t.Fatalf("RegisterCollectors() error = %v, want nil", err)
	}
}

func TestCollectorFeatureRuntimeConfig(t *testing.T) {
	t.Parallel()

	if got := (CollectorFeature{}).RuntimeConfig(); got != nil {
		t.Fatalf("RuntimeConfig() = %v, want nil", got)
	}

	feature := CollectorFeature{
		RuntimeConfigFunc: func() []any {
			return []any{"demo", "enabled"}
		},
	}
	got := feature.RuntimeConfig()
	if len(got) != 2 || got[0] != "demo" || got[1] != "enabled" {
		t.Fatalf("RuntimeConfig() = %v, want [demo enabled]", got)
	}
}

func TestRegisterCollectorsReturnsDuplicateCollectorError(t *testing.T) {
	t.Parallel()

	registry := prometheus.NewRegistry()
	collector := newConstCollector("duplicate_feature_value", "Duplicate feature value", 1)
	if err := RegisterCollectors(registry, collector); err != nil {
		t.Fatalf("RegisterCollectors() initial error = %v, want nil", err)
	}

	err := RegisterCollectors(registry, collector)
	if err == nil {
		t.Fatal("RegisterCollectors() error = nil, want duplicate collector error")
	}
}

func TestRegisterCollectorsRollsBackBatchOnError(t *testing.T) {
	t.Parallel()

	registry := prometheus.NewRegistry()
	existing := newConstCollector("existing_feature_value", "Existing feature value", 1)
	if err := RegisterCollectors(registry, existing); err != nil {
		t.Fatalf("RegisterCollectors() existing error = %v, want nil", err)
	}

	first := newConstCollector("batch_feature_value", "Batch feature value", 1)
	duplicate := newConstCollector("existing_feature_value", "Existing feature value", 2)
	if err := RegisterCollectors(registry, first, duplicate); err == nil {
		t.Fatal("RegisterCollectors() error = nil, want duplicate collector error")
	}

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v, want nil", err)
	}
	if hasMetricFamily(families, "batch_feature_value") {
		t.Fatal("Gather() contains batch_feature_value after rollback")
	}
	if !hasMetricFamily(families, "existing_feature_value") {
		t.Fatal("Gather() missing pre-existing existing_feature_value")
	}
}

func TestRegisterAndStartCollectorsRegistersAndStarts(t *testing.T) {
	t.Parallel()

	registry := prometheus.NewRegistry()
	collector := &startableTestCollector{
		collector: newConstCollector("startable_feature_value", "Startable feature value", 1),
	}
	ctx := context.WithValue(context.Background(), testContextKey{}, "value")

	if err := RegisterAndStartCollectors(ctx, registry, collector); err != nil {
		t.Fatalf("RegisterAndStartCollectors() error = %v, want nil", err)
	}
	if collector.startCount != 1 {
		t.Fatalf("startCount = %d, want 1", collector.startCount)
	}
	if collector.ctx != ctx {
		t.Fatal("Start() received different context")
	}

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v, want nil", err)
	}
	if !hasMetricFamily(families, "startable_feature_value") {
		t.Fatal("Gather() missing startable_feature_value")
	}
}

func TestRegisterAndStartCollectorsUsesBackgroundForNilContext(t *testing.T) {
	t.Parallel()

	collector := &startableTestCollector{
		collector: newConstCollector("startable_nil_context_value", "Startable nil context value", 1),
	}
	var nilContext context.Context
	if err := RegisterAndStartCollectors(nilContext, prometheus.NewRegistry(), collector); err != nil {
		t.Fatalf("RegisterAndStartCollectors() error = %v, want nil", err)
	}
	if collector.ctx == nil {
		t.Fatal("Start() context = nil, want background context")
	}
}

func TestRegisterAndStartCollectorsDoesNotStartOnRegistrationError(t *testing.T) {
	t.Parallel()

	registry := prometheus.NewRegistry()
	existing := newConstCollector("existing_startable_value", "Existing startable value", 1)
	if err := RegisterCollectors(registry, existing); err != nil {
		t.Fatalf("RegisterCollectors() existing error = %v, want nil", err)
	}
	first := &startableTestCollector{
		collector: newConstCollector("duplicate_startable_value", "Duplicate startable value", 1),
	}
	second := &startableTestCollector{
		collector: newConstCollector("duplicate_startable_value", "Duplicate startable value", 2),
	}

	err := RegisterAndStartCollectors(context.Background(), registry, first, second)
	if err == nil {
		t.Fatal("RegisterAndStartCollectors() error = nil, want duplicate collector error")
	}
	if first.startCount != 0 {
		t.Fatalf("first startCount = %d, want 0", first.startCount)
	}
	if second.startCount != 0 {
		t.Fatalf("second startCount = %d, want 0", second.startCount)
	}
	families, gatherErr := registry.Gather()
	if gatherErr != nil {
		t.Fatalf("Gather() error = %v, want nil", gatherErr)
	}
	if hasMetricFamily(families, "duplicate_startable_value") {
		t.Fatal("Gather() contains duplicate_startable_value after rollback")
	}
	if !hasMetricFamily(families, "existing_startable_value") {
		t.Fatal("Gather() missing pre-existing existing_startable_value")
	}
}

type testContextKey struct{}

type startableTestCollector struct {
	collector  prometheus.Collector
	startCount int
	ctx        context.Context
}

func (c *startableTestCollector) Describe(ch chan<- *prometheus.Desc) {
	c.collector.Describe(ch)
}

func (c *startableTestCollector) Collect(ch chan<- prometheus.Metric) {
	c.collector.Collect(ch)
}

func (c *startableTestCollector) Start(ctx context.Context) {
	c.startCount++
	c.ctx = ctx
}
