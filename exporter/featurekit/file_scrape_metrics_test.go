package featurekit_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/exportertest"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"
)

func TestFileScrapeMetricSpecs(t *testing.T) {
	t.Parallel()

	ids := featurekit.FileScrapeMetricIDsFor("config")
	if ids.Up != "config_up" || ids.Valid != "config_valid" {
		t.Fatalf("FileScrapeMetricIDsFor() = %#v", ids)
	}

	specs := featurekit.FileScrapeMetricSpecs("config", []string{"source"})
	descs := ids.Descs(featurekit.LoadFeatureMetricDescriptors("demo", "", specs))
	metrics := framework.FileScrapeMetrics{
		LabelValues:          []string{"/tmp/config.yml"},
		MTimeDesc:            descs.MTimeDesc,
		UpDesc:               descs.UpDesc,
		ValidDesc:            descs.ValidDesc,
		ReadErrorsTotalDesc:  descs.ReadErrorsTotalDesc,
		ParseErrorsTotalDesc: descs.ParseErrorsTotalDesc,
		ScrapeDurationDesc:   descs.ScrapeDurationDesc,
	}
	collector := callbackCollector{
		describe: func(ch chan<- *prometheus.Desc) {
			metrics.Describe(ch)
		},
		collect: func(ch chan<- prometheus.Metric) {
			metrics.CollectResult(ch, framework.FileScrapeResult{
				Up:                    true,
				MTimeSeconds:          123,
				ReadErrorsTotal:       2,
				ParseErrorsTotal:      3,
				ScrapeDurationSeconds: 0.25,
			})
			metrics.CollectValid(ch, false)
		},
	}

	registry := prometheus.NewRegistry()
	if err := registry.Register(collector); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	labels := map[string]string{"source": "/tmp/config.yml"}
	exportertest.AssertMetricValue(t, families, "demo_config_up", labels, 1)
	exportertest.AssertMetricValue(t, families, "demo_config_valid", labels, 0)
	exportertest.AssertMetricValue(t, families, "demo_config_mtime_seconds", labels, 123)
	exportertest.AssertMetricValue(t, families, "demo_config_read_errors_total", labels, 2)
	exportertest.AssertMetricValue(t, families, "demo_config_parse_errors_total", labels, 3)
	exportertest.AssertMetricValue(t, families, "demo_config_scrape_duration_seconds", labels, 0.25)
}

type callbackCollector struct {
	describe func(chan<- *prometheus.Desc)
	collect  func(chan<- prometheus.Metric)
}

func (c callbackCollector) Describe(ch chan<- *prometheus.Desc) {
	c.describe(ch)
}

func (c callbackCollector) Collect(ch chan<- prometheus.Metric) {
	c.collect(ch)
}
