package featurekit_test

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/exportertest"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"
)

func TestLastKnownGoodMetricSpecs(t *testing.T) {
	t.Parallel()

	specs := featurekit.LastKnownGoodMetricSpecs("registration", []string{"domain"})
	if len(specs) != 4 {
		t.Fatalf("LastKnownGoodMetricSpecs() returned %d specs, want 4", len(specs))
	}
	if specs[0].ID != "registration_last_success_timestamp_seconds" {
		t.Fatalf("first spec ID = %q", specs[0].ID)
	}
	if got := specs[0].MetricName("domain", "domain_exporter"); got != "domain_registration_last_success_timestamp_seconds" {
		t.Fatalf("metric name = %q, want domain_registration_last_success_timestamp_seconds", got)
	}
	if len(specs[0].Labels) != 1 || specs[0].Labels[0] != "domain" {
		t.Fatalf("labels = %#v, want domain", specs[0].Labels)
	}
}

func TestLastKnownGoodMetricIDsForRequiresSource(t *testing.T) {
	t.Parallel()

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for empty source")
		}
	}()
	_ = featurekit.LastKnownGoodMetricIDsFor(" ")
}

func TestCollectLastKnownGoodMetrics(t *testing.T) {
	t.Parallel()

	ids := featurekit.LastKnownGoodMetricIDsFor("registration")
	descriptors := featurekit.LoadFeatureMetricDescriptors("domain", "domain_exporter", featurekit.LastKnownGoodMetricSpecs("registration", []string{"domain"}))
	ctx := featurekit.FeatureMetricsContext[struct{}]{
		Descriptors: descriptors,
	}
	collector := callbackCollector{
		describe: func(ch chan<- *prometheus.Desc) {
			descriptors.Describe(ch)
		},
		collect: func(ch chan<- prometheus.Metric) {
			featurekit.CollectLastKnownGoodMetrics(ctx, ch, ids, featurekit.LastKnownGoodResult[int]{
				Available:           true,
				Stale:               true,
				LastSuccess:         time.Unix(123, 456_000_000),
				ConsecutiveFailures: 3,
			}, "example.com")
		},
	}

	families := exportertest.RegisterAndGather(t, collector)
	labels := map[string]string{"domain": "example.com"}
	exportertest.AssertMetricValue(t, families, "domain_registration_last_success_timestamp_seconds", labels, 123.456)
	exportertest.AssertMetricValue(t, families, "domain_registration_consecutive_failures", labels, 3)
	exportertest.AssertMetricValue(t, families, "domain_registration_data_available", labels, 1)
	exportertest.AssertMetricValue(t, families, "domain_registration_data_stale", labels, 1)
}
