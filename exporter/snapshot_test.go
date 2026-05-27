package exporter

import (
	"context"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/exportertest"
)

type testSnapshot struct {
	AttemptTime time.Time
	Success     bool
	Value       float64
}

type fakeSnapshotter struct {
	snapshot atomic.Value
	calls    atomic.Int64
}

func newFakeSnapshotter(snapshot testSnapshot) *fakeSnapshotter {
	s := &fakeSnapshotter{}
	s.snapshot.Store(snapshot)
	return s
}

func (s *fakeSnapshotter) Snapshot(context.Context, time.Time) testSnapshot {
	s.calls.Add(1)
	return s.snapshot.Load().(testSnapshot)
}

func (s *fakeSnapshotter) set(snapshot testSnapshot) {
	s.snapshot.Store(snapshot)
}

func TestSnapshotCollectorExportsSnapshotAndCollectionMetrics(t *testing.T) {
	t.Parallel()

	now := time.Unix(1700000000, 0)
	valueDesc := prometheus.NewDesc("snapshot_example_value", "Snapshot example value", nil, nil)
	collector := NewSnapshotCollector(SnapshotCollectorOptions[testSnapshot]{
		Namespace:       "demo_exporter",
		Snapshotter:     newFakeSnapshotter(testSnapshot{AttemptTime: now, Success: true, Value: 7}),
		RefreshInterval: time.Hour,
		StatusFunc:      testSnapshotStatus,
		DescribeFunc: func(ch chan<- *prometheus.Desc) {
			ch <- valueDesc
		},
		CollectFunc: func(ch chan<- prometheus.Metric, snapshot testSnapshot, _ time.Time) {
			ch <- prometheus.MustNewConstMetric(valueDesc, prometheus.GaugeValue, snapshot.Value)
		},
		Now: func() time.Time { return now },
	})

	families := exportertest.RegisterAndGather(t, collector)
	exportertest.AssertMetricValue(t, families, "snapshot_example_value", nil, 7)
	exportertest.AssertMetricValue(t, families, "demo_exporter_last_collection_success", nil, 1)
	exportertest.AssertMetricValue(t, families, "demo_exporter_last_collection_timestamp_seconds", nil, float64(now.Unix()))
	exportertest.AssertMetricValue(t, families, "demo_exporter_last_successful_collection_timestamp_seconds", nil, float64(now.Unix()))
}

func TestSnapshotCollectorCachesSnapshotUntilRefreshInterval(t *testing.T) {
	t.Parallel()

	start := time.Unix(1700000000, 0)
	now := start
	valueDesc := prometheus.NewDesc("snapshot_cached_value", "Snapshot cached value", nil, nil)
	snapshotter := newFakeSnapshotter(testSnapshot{AttemptTime: start, Success: true, Value: 1})
	collector := NewSnapshotCollector(SnapshotCollectorOptions[testSnapshot]{
		Namespace:       "demo_exporter",
		Snapshotter:     snapshotter,
		RefreshInterval: time.Hour,
		StatusFunc:      testSnapshotStatus,
		DescribeFunc: func(ch chan<- *prometheus.Desc) {
			ch <- valueDesc
		},
		CollectFunc: func(ch chan<- prometheus.Metric, snapshot testSnapshot, _ time.Time) {
			ch <- prometheus.MustNewConstMetric(valueDesc, prometheus.GaugeValue, snapshot.Value)
		},
		Now: func() time.Time { return now },
	})

	families := exportertest.RegisterAndGather(t, collector)
	exportertest.AssertMetricValue(t, families, "snapshot_cached_value", nil, 1)
	exportertest.AssertMetricValue(t, families, "demo_exporter_last_successful_collection_timestamp_seconds", nil, float64(start.Unix()))

	snapshotter.set(testSnapshot{AttemptTime: start.Add(30 * time.Minute), Success: true, Value: 2})
	now = start.Add(30 * time.Minute)
	families = exportertest.RegisterAndGather(t, collector)
	exportertest.AssertMetricValue(t, families, "snapshot_cached_value", nil, 1)
	exportertest.AssertMetricValue(t, families, "demo_exporter_last_collection_timestamp_seconds", nil, float64(start.Unix()))

	snapshotter.set(testSnapshot{AttemptTime: start.Add(2 * time.Hour), Success: false, Value: 3})
	now = start.Add(2 * time.Hour)
	families = exportertest.RegisterAndGather(t, collector)
	exportertest.AssertMetricValue(t, families, "snapshot_cached_value", nil, 3)
	exportertest.AssertMetricValue(t, families, "demo_exporter_last_collection_success", nil, 0)
	exportertest.AssertMetricValue(t, families, "demo_exporter_last_collection_timestamp_seconds", nil, float64(now.Unix()))
	exportertest.AssertMetricValue(t, families, "demo_exporter_last_successful_collection_timestamp_seconds", nil, float64(start.Unix()))
}

func TestSnapshotCollectorBackgroundRefreshUpdatesSnapshotOutsideScrape(t *testing.T) {
	t.Parallel()

	now := time.Unix(1700000000, 0)
	nowUnix := atomic.Int64{}
	nowUnix.Store(now.Unix())
	valueDesc := prometheus.NewDesc("snapshot_background_value", "Snapshot background value", nil, nil)
	snapshotter := newFakeSnapshotter(testSnapshot{AttemptTime: now, Success: true, Value: 1})
	collector := NewSnapshotCollector(SnapshotCollectorOptions[testSnapshot]{
		Namespace:       "demo_exporter",
		Snapshotter:     snapshotter,
		RefreshInterval: 20 * time.Millisecond,
		StatusFunc:      testSnapshotStatus,
		DescribeFunc: func(ch chan<- *prometheus.Desc) {
			ch <- valueDesc
		},
		CollectFunc: func(ch chan<- prometheus.Metric, snapshot testSnapshot, _ time.Time) {
			ch <- prometheus.MustNewConstMetric(valueDesc, prometheus.GaugeValue, snapshot.Value)
		},
		Now: func() time.Time { return time.Unix(nowUnix.Load(), 0) },
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	collector.Start(ctx)

	registry := prometheus.NewRegistry()
	exportertest.Register(t, registry, collector)
	exportertest.WaitForMetricValue(t, registry, "snapshot_background_value", nil, 1)

	next := now.Add(time.Minute)
	nowUnix.Store(next.Unix())
	snapshotter.set(testSnapshot{AttemptTime: next, Success: true, Value: 2})
	exportertest.WaitForMetricValue(t, registry, "snapshot_background_value", nil, 2)
}

func TestSnapshotCollectorInitializesAfterBackgroundStartBeforeFirstRefresh(t *testing.T) {
	t.Parallel()

	now := time.Unix(1700000000, 0)
	snapshotter := newFakeSnapshotter(testSnapshot{AttemptTime: now, Success: true, Value: 1})
	collector := NewSnapshotCollector(SnapshotCollectorOptions[testSnapshot]{
		Namespace:       "demo_exporter",
		Snapshotter:     snapshotter,
		RefreshInterval: time.Hour,
		StatusFunc:      testSnapshotStatus,
		Now:             func() time.Time { return now },
	})
	collector.backgroundStarted = true

	families := exportertest.RegisterAndGather(t, collector)
	exportertest.AssertMetricValue(t, families, "demo_exporter_last_collection_success", nil, 1)
	exportertest.AssertMetricValue(t, families, "demo_exporter_last_collection_timestamp_seconds", nil, float64(now.Unix()))
	if calls := snapshotter.calls.Load(); calls != 1 {
		t.Fatalf("snapshot calls = %d, want 1", calls)
	}
}

func TestSnapshotCollectorDefaultsAndErrorLogging(t *testing.T) {
	t.Parallel()

	logged := atomic.Int32{}
	collector := NewSnapshotCollector(SnapshotCollectorOptions[int]{
		ErrorLogFunc: func(logger *slog.Logger, snapshot int) {
			if logger == nil {
				t.Fatal("logger = nil, want default logger")
			}
			if snapshot != 0 {
				t.Fatalf("snapshot = %d, want zero value", snapshot)
			}
			logged.Add(1)
		},
	})

	families := exportertest.RegisterAndGather(t, collector)
	exportertest.AssertMetricValue(t, families, "exporter_last_collection_success", nil, 0)
	exportertest.AssertMetricValue(t, families, "exporter_last_collection_timestamp_seconds", nil, 0)
	exportertest.AssertMetricValue(t, families, "exporter_last_successful_collection_timestamp_seconds", nil, 0)
	if logged.Load() != 1 {
		t.Fatalf("error logs = %d, want 1", logged.Load())
	}
}

func TestSnapshotCollectorUsesCustomHelpText(t *testing.T) {
	t.Parallel()

	collector := NewSnapshotCollector(SnapshotCollectorOptions[int]{
		Namespace:                    "demo_exporter",
		LastCollectionSuccessHelp:    "Custom last collection success help.",
		LastCollectionTimestampHelp:  "Custom last collection timestamp help.",
		LastSuccessfulCollectionHelp: "Custom last successful collection help.",
	})

	families := exportertest.RegisterAndGather(t, collector)
	assertMetricHelp(t, families, "demo_exporter_last_collection_success", "Custom last collection success help.")
	assertMetricHelp(t, families, "demo_exporter_last_collection_timestamp_seconds", "Custom last collection timestamp help.")
	assertMetricHelp(t, families, "demo_exporter_last_successful_collection_timestamp_seconds", "Custom last successful collection help.")
}

func TestSnapshotCollectorStartIsIdempotent(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	collector := NewSnapshotCollector(SnapshotCollectorOptions[testSnapshot]{
		Snapshotter:     newFakeSnapshotter(testSnapshot{AttemptTime: now, Success: true, Value: 1}),
		RefreshInterval: time.Hour,
		StatusFunc:      testSnapshotStatus,
		Now:             func() time.Time { return now },
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	collector.Start(ctx)
	collector.Start(ctx)
	if !collector.backgroundStarted {
		t.Fatal("backgroundStarted = false, want true")
	}
}

func assertMetricHelp(t *testing.T, families []*dto.MetricFamily, name string, want string) {
	t.Helper()

	got := exportertest.MetricFamily(t, families, name).GetHelp()
	if got != want {
		t.Fatalf("%s help = %q, want %q", name, got, want)
	}
}

func testSnapshotStatus(snapshot testSnapshot) SnapshotStatus {
	return SnapshotStatus{
		AttemptTime: snapshot.AttemptTime,
		Success:     snapshot.Success,
	}
}
