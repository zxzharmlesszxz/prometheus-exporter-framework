package exportertest

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

const defaultWaitForMetricTimeout = 5 * time.Second

type TB interface {
	Helper()
	Fatalf(format string, args ...any)
}

func Register(tb TB, registry *prometheus.Registry, collector prometheus.Collector) {
	tb.Helper()

	if err := registry.Register(collector); err != nil {
		tb.Fatalf("register collector: %v", err)
	}
}

func Gather(tb TB, gatherer prometheus.Gatherer) []*dto.MetricFamily {
	tb.Helper()

	families, err := gatherer.Gather()
	if err != nil {
		tb.Fatalf("gather metrics: %v", err)
	}
	return families
}

func RegisterAndGather(tb TB, collector prometheus.Collector) []*dto.MetricFamily {
	tb.Helper()

	registry := prometheus.NewRegistry()
	Register(tb, registry, collector)
	return Gather(tb, registry)
}

func MetricFamily(tb TB, families []*dto.MetricFamily, name string) *dto.MetricFamily {
	tb.Helper()

	for _, family := range families {
		if family.GetName() == name {
			return family
		}
	}
	tb.Fatalf("metric family %q not found", name)
	return nil
}

// MetricValue returns gauge, counter, and untyped metric values. Histograms and
// summaries intentionally return false; use Histogram for histogram metrics.
func MetricValue(families []*dto.MetricFamily, name string, labels map[string]string) (float64, bool) {
	metric := Metric(families, name, labels)
	if metric == nil {
		return 0, false
	}
	switch {
	case metric.Gauge != nil:
		return metric.GetGauge().GetValue(), true
	case metric.Counter != nil:
		return metric.GetCounter().GetValue(), true
	case metric.Untyped != nil:
		return metric.GetUntyped().GetValue(), true
	}
	return 0, false
}

func Metric(families []*dto.MetricFamily, name string, labels map[string]string) *dto.Metric {
	family := metricFamily(families, name)
	if family == nil {
		return nil
	}
	for _, metric := range family.GetMetric() {
		if LabelsMatch(metric, labels) {
			return metric
		}
	}
	return nil
}

func AssertMetricValue(tb TB, families []*dto.MetricFamily, name string, labels map[string]string, want float64) {
	tb.Helper()

	got, ok := MetricValue(families, name, labels)
	if !ok {
		tb.Fatalf("metric %s%v not found", name, labels)
	}
	if got != want {
		tb.Fatalf("%s%v = %v, want %v", name, labels, got, want)
	}
}

func AssertMetricExists(tb TB, families []*dto.MetricFamily, name string, labels map[string]string) {
	tb.Helper()

	if Metric(families, name, labels) == nil {
		tb.Fatalf("metric %s%v not found", name, labels)
	}
}

func AssertMetricLabelPresent(tb TB, families []*dto.MetricFamily, name string, labels map[string]string, labelName string) {
	tb.Helper()

	metric := Metric(families, name, labels)
	if metric == nil {
		tb.Fatalf("metric %s%v not found", name, labels)
	}
	for _, label := range metric.GetLabel() {
		if label.GetName() == labelName {
			return
		}
	}
	tb.Fatalf("metric %s%v missing label %q", name, labels, labelName)
}

func RuntimeConfigValue(tb TB, config []any, key string) any {
	tb.Helper()

	for i := 0; i+1 < len(config); i += 2 {
		if config[i] == key {
			return config[i+1]
		}
	}
	tb.Fatalf("missing runtime config key %q in %#v", key, config)
	return nil
}

func WaitForMetricValue(tb TB, gatherer prometheus.Gatherer, name string, labels map[string]string, want float64) {
	tb.Helper()
	WaitForMetricValueWithin(tb, gatherer, name, labels, want, defaultWaitForMetricTimeout)
}

// WaitForMetricValueWithin polls gatherer until a metric reaches want or the
// timeout expires. Non-positive timeouts use the package default.
func WaitForMetricValueWithin(tb TB, gatherer prometheus.Gatherer, name string, labels map[string]string, want float64, timeout time.Duration) {
	tb.Helper()

	if timeout <= 0 {
		timeout = defaultWaitForMetricTimeout
	}
	deadline := time.Now().Add(timeout)
	var last float64
	seen := false
	for {
		families := Gather(tb, gatherer)
		got, ok := MetricValue(families, name, labels)
		if ok && got == want {
			return
		}
		if ok {
			last = got
			seen = true
		}
		if time.Now().After(deadline) {
			if seen {
				tb.Fatalf("%s%v did not become %v; last value was %v", name, labels, want, last)
			}
			tb.Fatalf("%s%v did not become %v; metric was not found", name, labels, want)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func Histogram(tb TB, families []*dto.MetricFamily, name string, labels map[string]string) *dto.Histogram {
	tb.Helper()

	family := metricFamily(families, name)
	if family == nil {
		tb.Fatalf("histogram family %q not found", name)
	}
	for _, metric := range family.GetMetric() {
		if LabelsMatch(metric, labels) && metric.Histogram != nil {
			return metric.GetHistogram()
		}
	}
	tb.Fatalf("histogram %s%v not found", name, labels)
	return nil
}

func LabelsMatch(metric *dto.Metric, want map[string]string) bool {
	if metric == nil {
		return false
	}
	for name, value := range want {
		found := false
		for _, label := range metric.GetLabel() {
			if label.GetName() == name && label.GetValue() == value {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func metricFamily(families []*dto.MetricFamily, name string) *dto.MetricFamily {
	for _, family := range families {
		if family.GetName() == name {
			return family
		}
	}
	return nil
}
