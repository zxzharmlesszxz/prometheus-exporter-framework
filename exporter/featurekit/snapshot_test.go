package featurekit

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
)

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
