package featurekit_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/exportertest"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"
)

func TestTTLCacheMetricSpecs(t *testing.T) {
	t.Parallel()

	specs := featurekit.TTLCacheMetricSpecs([]string{"source"})
	if len(specs) != 7 {
		t.Fatalf("TTLCacheMetricSpecs() returned %d specs, want 7", len(specs))
	}
	if specs[0].ID != featurekit.TTLCacheMetricEntries {
		t.Fatalf("first spec ID = %q, want %q", specs[0].ID, featurekit.TTLCacheMetricEntries)
	}
	if got := specs[0].MetricName("demo", "demo_exporter"); got != "demo_cache_entries" {
		t.Fatalf("entries metric name = %q, want demo_cache_entries", got)
	}
	if len(specs[0].Labels) != 2 || specs[0].Labels[0] != "cache" || specs[0].Labels[1] != "source" {
		t.Fatalf("labels = %#v, want cache and source", specs[0].Labels)
	}
}

func TestCollectTTLCacheMetrics(t *testing.T) {
	t.Parallel()

	descriptors := featurekit.LoadFeatureMetricDescriptors("demo", "demo_exporter", featurekit.TTLCacheMetricSpecs([]string{"source"}))
	ctx := featurekit.FeatureMetricsContext[struct{}]{
		Descriptors: descriptors,
	}
	collector := callbackCollector{
		describe: func(ch chan<- *prometheus.Desc) {
			descriptors.Describe(ch)
		},
		collect: func(ch chan<- prometheus.Metric) {
			featurekit.CollectTTLCacheMetrics(ctx, ch, "whois", featurekit.TTLCacheStats{
				Entries: 3,
				Hits:    11,
				Misses:  2,
				Sets:    5,
				Deletes: 1,
				Expired: 4,
				Clears:  1,
			}, "/etc/demo.yml")
		},
	}

	families := exportertest.RegisterAndGather(t, collector)
	labels := map[string]string{"cache": "whois", "source": "/etc/demo.yml"}
	exportertest.AssertMetricValue(t, families, "demo_cache_entries", labels, 3)
	exportertest.AssertMetricValue(t, families, "demo_cache_hits_total", labels, 11)
	exportertest.AssertMetricValue(t, families, "demo_cache_misses_total", labels, 2)
	exportertest.AssertMetricValue(t, families, "demo_cache_sets_total", labels, 5)
	exportertest.AssertMetricValue(t, families, "demo_cache_deletes_total", labels, 1)
	exportertest.AssertMetricValue(t, families, "demo_cache_expired_total", labels, 4)
	exportertest.AssertMetricValue(t, families, "demo_cache_clears_total", labels, 1)
}
