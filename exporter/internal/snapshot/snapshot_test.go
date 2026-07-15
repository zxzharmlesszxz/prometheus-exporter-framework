package snapshot

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

type sequenceClock struct {
	values []time.Time
	index  atomic.Int64
}

func newSequenceClock(values ...time.Time) *sequenceClock {
	return &sequenceClock{values: values}
}

func (c *sequenceClock) Now() time.Time {
	index := int(c.index.Add(1)) - 1
	if index >= len(c.values) {
		return c.values[len(c.values)-1]
	}
	return c.values[index]
}

func TestSnapshotCollectorExportsSnapshotAndCollectionMetrics(t *testing.T) {
	t.Parallel()

	now := time.Unix(1700000000, 0)
	clock := newSequenceClock(now, now, now.Add(250*time.Millisecond))
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
		Now: clock.Now,
	})

	families := exportertest.RegisterAndGather(t, collector)
	exportertest.AssertMetricValue(t, families, "snapshot_example_value", nil, 7)
	exportertest.AssertMetricValue(t, families, "demo_exporter_last_collection_success", nil, 1)
	exportertest.AssertMetricValue(t, families, "demo_exporter_last_collection_timestamp_seconds", nil, float64(now.Unix()))
	exportertest.AssertMetricValue(t, families, "demo_exporter_last_successful_collection_timestamp_seconds", nil, float64(now.Unix()))
	histogram := exportertest.Histogram(t, families, "demo_exporter_collection_duration_seconds", nil)
	if got := histogram.GetSampleCount(); got != 1 {
		t.Fatalf("collection duration count = %d, want 1", got)
	}
	if got := histogram.GetSampleSum(); got != 0.25 {
		t.Fatalf("collection duration sum = %v, want 0.25", got)
	}
	if got := len(histogram.GetBucket()); got != len(prometheus.DefBuckets) {
		t.Fatalf("collection duration buckets = %d, want %d", got, len(prometheus.DefBuckets))
	}
	for i, bucket := range histogram.GetBucket() {
		if got := bucket.GetUpperBound(); got != prometheus.DefBuckets[i] {
			t.Fatalf("collection duration bucket[%d] upper bound = %v, want %v", i, got, prometheus.DefBuckets[i])
		}
	}
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
	now := time.Unix(1_700_000_000, 0)
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
		Now: func() time.Time { return now },
	})

	families := exportertest.RegisterAndGather(t, collector)
	exportertest.AssertMetricValue(t, families, "exporter_last_collection_success", nil, 0)
	exportertest.AssertMetricValue(t, families, "exporter_last_collection_timestamp_seconds", nil, float64(now.Unix()))
	exportertest.AssertMetricValue(t, families, "exporter_last_successful_collection_timestamp_seconds", nil, 0)
	histogram := exportertest.Histogram(t, families, "exporter_collection_duration_seconds", nil)
	if got := histogram.GetSampleCount(); got != 1 {
		t.Fatalf("collection duration count = %d, want 1", got)
	}
	if logged.Load() != 1 {
		t.Fatalf("error logs = %d, want 1", logged.Load())
	}
}

func TestSnapshotCollectorFallsBackToRefreshTimeWhenStatusAttemptTimeIsZero(t *testing.T) {
	t.Parallel()

	start := time.Unix(1_700_000_000, 0)
	now := start
	snapshotter := newFakeSnapshotter(testSnapshot{Success: true, Value: 1})
	collector := NewSnapshotCollector(SnapshotCollectorOptions[testSnapshot]{
		Namespace:       "demo_exporter",
		Snapshotter:     snapshotter,
		RefreshInterval: time.Hour,
		StatusFunc: func(snapshot testSnapshot) SnapshotStatus {
			return SnapshotStatus{Success: snapshot.Success}
		},
		Now: func() time.Time { return now },
	})

	families := exportertest.RegisterAndGather(t, collector)
	exportertest.AssertMetricValue(t, families, "demo_exporter_last_collection_success", nil, 1)
	exportertest.AssertMetricValue(t, families, "demo_exporter_last_collection_timestamp_seconds", nil, float64(start.Unix()))
	exportertest.AssertMetricValue(t, families, "demo_exporter_last_successful_collection_timestamp_seconds", nil, float64(start.Unix()))

	now = start.Add(30 * time.Minute)
	_ = exportertest.RegisterAndGather(t, collector)
	if calls := snapshotter.calls.Load(); calls != 1 {
		t.Fatalf("snapshot calls = %d, want cached snapshot without second refresh", calls)
	}
}

func TestSnapshotCollectorRechecksFreshnessWithCurrentTimeAfterWaiting(t *testing.T) {
	start := time.Unix(1_700_000_000, 0)
	clock := newSequenceClock(
		start.Add(3*time.Hour),
		start.Add(3*time.Hour),
	)
	snapshotter := newFakeSnapshotter(testSnapshot{
		AttemptTime: start.Add(3 * time.Hour),
		Success:     true,
		Value:       3,
	})
	collector := NewSnapshotCollector(SnapshotCollectorOptions[testSnapshot]{
		Namespace:       "demo_exporter",
		Snapshotter:     snapshotter,
		RefreshInterval: time.Hour,
		StatusFunc:      testSnapshotStatus,
		Now:             clock.Now,
	})
	collector.initialized = true
	collector.refreshing = true
	collector.snapshot = testSnapshot{AttemptTime: start, Success: true, Value: 1}
	collector.snapshotStatus = SnapshotStatus{AttemptTime: start, Success: true}
	collector.lastSuccessfulCollection = start

	done := make(chan snapshotState[testSnapshot], 1)
	go func() {
		done <- collector.currentSnapshot(start.Add(2 * time.Hour))
	}()

	time.Sleep(10 * time.Millisecond)
	collector.mu.Lock()
	collector.snapshot = testSnapshot{AttemptTime: start.Add(2 * time.Hour), Success: true, Value: 2}
	collector.snapshotStatus = SnapshotStatus{AttemptTime: start.Add(2 * time.Hour), Success: true}
	collector.lastSuccessfulCollection = start.Add(2 * time.Hour)
	collector.refreshing = false
	collector.cond.Broadcast()
	collector.mu.Unlock()

	state := <-done
	if state.snapshot.Value != 3 {
		t.Fatalf("snapshot value = %v, want refresh after waited snapshot was already stale", state.snapshot.Value)
	}
	if calls := snapshotter.calls.Load(); calls != 1 {
		t.Fatalf("snapshot calls = %d, want one follow-up refresh", calls)
	}
}

func TestSnapshotCollectorUsesCustomHelpText(t *testing.T) {
	t.Parallel()

	collector := NewSnapshotCollector(SnapshotCollectorOptions[int]{
		Namespace:                    "demo_exporter",
		LastCollectionSuccessHelp:    "Custom last collection success help.",
		LastCollectionTimestampHelp:  "Custom last collection timestamp help.",
		LastSuccessfulCollectionHelp: "Custom last successful collection help.",
		CollectionDurationHelp:       "Custom collection duration help.",
	})

	families := exportertest.RegisterAndGather(t, collector)
	assertMetricHelp(t, families, "demo_exporter_last_collection_success", "Custom last collection success help.")
	assertMetricHelp(t, families, "demo_exporter_last_collection_timestamp_seconds", "Custom last collection timestamp help.")
	assertMetricHelp(t, families, "demo_exporter_last_successful_collection_timestamp_seconds", "Custom last successful collection help.")
	assertMetricHelp(t, families, "demo_exporter_collection_duration_seconds", "Custom collection duration help.")
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
	defer cancel()

	collector.Start(ctx)
	collector.Start(ctx)
	collector.mu.Lock()
	started := collector.backgroundStarted
	collector.mu.Unlock()
	if !started {
		t.Fatal("backgroundStarted = false, want true")
	}
}

func TestSnapshotCollectorClearsBackgroundStartedWhenContextIsCanceled(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	collector := NewSnapshotCollector(SnapshotCollectorOptions[testSnapshot]{
		Snapshotter:     newFakeSnapshotter(testSnapshot{AttemptTime: now, Success: true, Value: 1}),
		RefreshInterval: time.Hour,
		StatusFunc:      testSnapshotStatus,
		Now:             func() time.Time { return now },
	})
	ctx, cancel := context.WithCancel(context.Background())

	collector.Start(ctx)
	cancel()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		collector.mu.Lock()
		started := collector.backgroundStarted
		collector.mu.Unlock()
		if !started {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("backgroundStarted stayed true after context cancellation")
}

func TestSnapshotCollectorCanRestartAfterContextCancellation(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	snapshotter := newFakeSnapshotter(testSnapshot{AttemptTime: now, Success: true, Value: 1})
	collector := NewSnapshotCollector(SnapshotCollectorOptions[testSnapshot]{
		Snapshotter:     snapshotter,
		RefreshInterval: time.Hour,
		StatusFunc:      testSnapshotStatus,
		Now:             func() time.Time { return now },
	})

	firstCtx, firstCancel := context.WithCancel(context.Background())
	collector.Start(firstCtx)
	waitForSnapshotCalls(t, snapshotter, 1)
	firstCancel()
	waitForBackgroundStopped(t, collector)

	secondCtx, secondCancel := context.WithCancel(context.Background())
	defer secondCancel()
	collector.Start(secondCtx)
	waitForSnapshotCalls(t, snapshotter, 2)
}

func TestSnapshotCollectorDoesNotRefreshWhenStartedWithCanceledContext(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	snapshotter := newFakeSnapshotter(testSnapshot{AttemptTime: now, Success: true, Value: 1})
	collector := NewSnapshotCollector(SnapshotCollectorOptions[testSnapshot]{
		Snapshotter:     snapshotter,
		RefreshInterval: time.Hour,
		StatusFunc:      testSnapshotStatus,
		Now:             func() time.Time { return now },
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	collector.Start(ctx)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		collector.mu.Lock()
		started := collector.backgroundStarted
		collector.mu.Unlock()
		if !started {
			if calls := snapshotter.calls.Load(); calls != 0 {
				t.Fatalf("snapshot calls = %d, want 0", calls)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("backgroundStarted stayed true after starting with a canceled context")
}

func TestSnapshotCollectorStartAcceptsNilContext(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	snapshotter := newFakeSnapshotter(testSnapshot{AttemptTime: now, Success: true, Value: 1})
	collector := NewSnapshotCollector(SnapshotCollectorOptions[testSnapshot]{
		Snapshotter:     snapshotter,
		RefreshInterval: time.Hour,
		StatusFunc:      testSnapshotStatus,
		Now:             func() time.Time { return now },
	})

	var ctx context.Context
	collector.Start(ctx)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if snapshotter.calls.Load() > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("snapshotter was not called after Start(nil)")
}

func TestSnapshotCollectorSyncRefreshTimeoutAddsDeadline(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	snapshotter := &contextInspectingSnapshotter{}
	collector := NewSnapshotCollector(SnapshotCollectorOptions[testSnapshot]{
		Namespace:          "demo_exporter",
		Snapshotter:        snapshotter,
		RefreshInterval:    time.Hour,
		StatusFunc:         testSnapshotStatus,
		SyncRefreshTimeout: time.Second,
		Now:                func() time.Time { return now },
	})

	_ = exportertest.RegisterAndGather(t, collector)

	deadline, ok := snapshotter.deadline.Load().(time.Time)
	if !ok {
		t.Fatal("snapshot context had no deadline")
	}
	if deadline.Before(time.Now()) {
		t.Fatalf("snapshot context deadline = %v, want future deadline", deadline)
	}
}

func TestSnapshotCollectorBackgroundRefreshUsesStartContextWithoutSyncTimeout(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	snapshotter := &contextInspectingSnapshotter{}
	collector := NewSnapshotCollector(SnapshotCollectorOptions[testSnapshot]{
		Namespace:          "demo_exporter",
		Snapshotter:        snapshotter,
		RefreshInterval:    time.Hour,
		StatusFunc:         testSnapshotStatus,
		SyncRefreshTimeout: time.Second,
		Now:                func() time.Time { return now },
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	collector.Start(ctx)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if snapshotter.calls.Load() > 0 {
			if snapshotter.hasDeadline.Load() {
				t.Fatal("background refresh context had sync refresh deadline")
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("snapshotter was not called by background refresh")
}

func waitForBackgroundStopped[T any](t *testing.T, collector *SnapshotCollector[T]) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		collector.mu.Lock()
		started := collector.backgroundStarted
		collector.mu.Unlock()
		if !started {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("backgroundStarted stayed true after context cancellation")
}

func waitForSnapshotCalls(t *testing.T, snapshotter *fakeSnapshotter, want int64) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if snapshotter.calls.Load() >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("snapshot calls = %d, want at least %d", snapshotter.calls.Load(), want)
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

type contextInspectingSnapshotter struct {
	calls       atomic.Int64
	hasDeadline atomic.Bool
	deadline    atomic.Value
}

func (s *contextInspectingSnapshotter) Snapshot(ctx context.Context, now time.Time) testSnapshot {
	s.calls.Add(1)
	if deadline, ok := ctx.Deadline(); ok {
		s.hasDeadline.Store(true)
		s.deadline.Store(deadline)
	} else {
		s.hasDeadline.Store(false)
	}
	return testSnapshot{AttemptTime: now, Success: true, Value: 1}
}
