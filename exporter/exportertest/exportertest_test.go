package exportertest

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"google.golang.org/protobuf/proto"
)

func TestRegisterGatherAndMetricAssertions(t *testing.T) {
	t.Parallel()

	registry := prometheus.NewRegistry()
	Register(t, registry, constCollector{
		desc:  prometheus.NewDesc("demo_value", "Demo value.", []string{"state"}, nil),
		value: 7,
		label: "ready",
	})

	families := Gather(t, registry)
	family := MetricFamily(t, families, "demo_value")
	if got := family.GetName(); got != "demo_value" {
		t.Fatalf("MetricFamily() = %q, want demo_value", got)
	}

	labels := map[string]string{"state": "ready"}
	AssertMetricExists(t, families, "demo_value", labels)
	AssertMetricLabelPresent(t, families, "demo_value", labels, "state")
	AssertMetricValue(t, families, "demo_value", labels, 7)
	WaitForMetricValue(t, registry, "demo_value", labels, 7)
	WaitForMetricValueWithin(t, registry, "demo_value", labels, 7, 50*time.Millisecond)
	WaitForMetricValueWithin(t, registry, "demo_value", labels, 7, 0)

	got, ok := MetricValue(families, "demo_value", labels)
	if !ok || got != 7 {
		t.Fatalf("MetricValue() = %v, %v; want 7, true", got, ok)
	}
	if metric := Metric(families, "demo_value", labels); metric == nil {
		t.Fatal("Metric() = nil, want matching metric")
	}
	if metric := Metric(families, "demo_value", map[string]string{"state": "missing"}); metric != nil {
		t.Fatalf("Metric() = %v, want nil for missing labels", metric)
	}
	if LabelsMatch(family.GetMetric()[0], map[string]string{"state": "missing"}) {
		t.Fatal("LabelsMatch() = true, want false")
	}
	if LabelsMatch(nil, labels) {
		t.Fatal("LabelsMatch(nil) = true, want false")
	}
}

func TestRegisterAndGather(t *testing.T) {
	t.Parallel()

	families := RegisterAndGather(t, constCollector{
		desc:  prometheus.NewDesc("single_value", "Single value.", nil, nil),
		value: 1,
	})
	AssertMetricValue(t, families, "single_value", nil, 1)
}

func TestMetricValueSupportsCounterAndUntyped(t *testing.T) {
	t.Parallel()

	families := []*dto.MetricFamily{
		{
			Name: proto.String("counter_value"),
			Metric: []*dto.Metric{{
				Counter: &dto.Counter{Value: proto.Float64(3)},
			}},
		},
		{
			Name: proto.String("untyped_value"),
			Metric: []*dto.Metric{{
				Untyped: &dto.Untyped{Value: proto.Float64(4)},
			}},
		},
		{
			Name: proto.String("empty_value"),
			Metric: []*dto.Metric{{
				Label: []*dto.LabelPair{{
					Name:  proto.String("state"),
					Value: proto.String("ready"),
				}},
			}},
		},
	}

	if got, ok := MetricValue(families, "counter_value", nil); !ok || got != 3 {
		t.Fatalf("counter MetricValue() = %v, %v; want 3, true", got, ok)
	}
	if got, ok := MetricValue(families, "untyped_value", nil); !ok || got != 4 {
		t.Fatalf("untyped MetricValue() = %v, %v; want 4, true", got, ok)
	}
	if _, ok := MetricValue(families, "empty_value", map[string]string{"state": "ready"}); ok {
		t.Fatal("empty MetricValue() ok = true, want false")
	}
	if _, ok := MetricValue(families, "missing_value", nil); ok {
		t.Fatal("missing MetricValue() ok = true, want false")
	}
	if metric := Metric(families, "missing_value", nil); metric != nil {
		t.Fatalf("missing Metric() = %v, want nil", metric)
	}
}

func TestHistogram(t *testing.T) {
	t.Parallel()

	families := []*dto.MetricFamily{{
		Name: proto.String("request_duration_seconds"),
		Metric: []*dto.Metric{{
			Label: []*dto.LabelPair{{
				Name:  proto.String("route"),
				Value: proto.String("/metrics"),
			}},
			Histogram: &dto.Histogram{
				SampleCount: proto.Uint64(2),
				SampleSum:   proto.Float64(1.5),
			},
		}},
	}}

	histogram := Histogram(t, families, "request_duration_seconds", map[string]string{"route": "/metrics"})
	if got := histogram.GetSampleCount(); got != 2 {
		t.Fatalf("Histogram().SampleCount = %d, want 2", got)
	}
}

func TestRuntimeConfigValue(t *testing.T) {
	t.Parallel()

	config := []any{"first", 1, "second", "value"}

	if got := RuntimeConfigValue(t, config, "second"); got != "value" {
		t.Fatalf("RuntimeConfigValue() = %#v, want %#v", got, "value")
	}
}

func TestFailures(t *testing.T) {
	t.Parallel()

	families := []*dto.MetricFamily{{
		Name: proto.String("demo_value"),
		Metric: []*dto.Metric{{
			Gauge: &dto.Gauge{Value: proto.Float64(1)},
		}},
	}}

	expectFatal(t, func(tb TB) {
		registry := prometheus.NewRegistry()
		collector := constCollector{
			desc:  prometheus.NewDesc("duplicate_value", "Duplicate value.", nil, nil),
			value: 1,
		}
		registry.MustRegister(collector)
		Register(tb, registry, collector)
	}, "register collector")
	expectFatal(t, func(tb TB) {
		Gather(tb, failingGatherer{})
	}, "gather metrics")
	expectFatal(t, func(tb TB) {
		MetricFamily(tb, families, "missing_value")
	}, "metric family")
	expectFatal(t, func(tb TB) {
		AssertMetricValue(tb, families, "missing_value", nil, 1)
	}, "not found")
	expectFatal(t, func(tb TB) {
		AssertMetricExists(tb, families, "missing_value", nil)
	}, "not found")
	expectFatal(t, func(tb TB) {
		AssertMetricLabelPresent(tb, families, "missing_value", nil, "state")
	}, "not found")
	expectFatal(t, func(tb TB) {
		AssertMetricLabelPresent(tb, families, "demo_value", nil, "state")
	}, `missing label "state"`)
	expectFatal(t, func(tb TB) {
		AssertMetricValue(tb, families, "demo_value", nil, 2)
	}, "want 2")
	expectFatal(t, func(tb TB) {
		RuntimeConfigValue(tb, []any{"first", 1}, "missing")
	}, "missing runtime config")
	expectFatal(t, func(tb TB) {
		Histogram(tb, families, "missing_histogram", nil)
	}, "histogram family")
	expectFatal(t, func(tb TB) {
		Histogram(tb, families, "demo_value", nil)
	}, "histogram demo_value")
	expectFatal(t, func(tb TB) {
		registry := prometheus.NewRegistry()
		Register(t, registry, constCollector{
			desc:  prometheus.NewDesc("waiting_value", "Waiting value.", nil, nil),
			value: 1,
		})
		WaitForMetricValueWithin(tb, registry, "waiting_value", nil, 2, time.Nanosecond)
	}, "last value was 1")
	expectFatal(t, func(tb TB) {
		WaitForMetricValueWithin(tb, prometheus.NewRegistry(), "missing_waiting_value", nil, 1, time.Nanosecond)
	}, "metric was not found")
}

type constCollector struct {
	desc  *prometheus.Desc
	value float64
	label string
}

func (c constCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.desc
}

func (c constCollector) Collect(ch chan<- prometheus.Metric) {
	if c.label == "" {
		ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, c.value)
		return
	}
	ch <- prometheus.MustNewConstMetric(c.desc, prometheus.GaugeValue, c.value, c.label)
}

type failingGatherer struct{}

func (failingGatherer) Gather() ([]*dto.MetricFamily, error) {
	return nil, errors.New("gather failed")
}

type panicTB struct{}

func (panicTB) Helper() {}

func (panicTB) Fatalf(format string, args ...any) {
	panic(fmt.Sprintf(format, args...))
}

func expectFatal(t *testing.T, fn func(TB), contains string) {
	t.Helper()

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatalf("Fatalf was not called; want message containing %q", contains)
		}
		message := fmt.Sprint(recovered)
		if !strings.Contains(message, contains) {
			t.Fatalf("Fatalf message = %q, want to contain %q", message, contains)
		}
	}()

	fn(panicTB{})
}
