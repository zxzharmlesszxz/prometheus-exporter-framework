package snapshot

import (
	"context"
	"log/slog"
	"math"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/internal/metric"
)

const DefaultSnapshotRefreshInterval = 5 * time.Minute

type Snapshotter[T any] interface {
	Snapshot(context.Context, time.Time) T
}

type SnapshotStatus struct {
	// AttemptTime is the time of the collection attempt represented by the
	// snapshot. If StatusFunc leaves it zero, SnapshotCollector falls back to
	// the collector's refresh time so cache freshness and timestamp metrics
	// continue to behave predictably.
	AttemptTime time.Time
	Success     bool
}

type SnapshotCollectorOptions[T any] struct {
	Namespace       string
	Logger          *slog.Logger
	Snapshotter     Snapshotter[T]
	RefreshInterval time.Duration
	StatusFunc      func(T) SnapshotStatus
	DescribeFunc    func(chan<- *prometheus.Desc)
	CollectFunc     func(chan<- prometheus.Metric, T, time.Time)
	// ErrorLogFunc is called after every refresh with the returned snapshot.
	// The hook should inspect the snapshot/status payload and log only failed
	// domain collection attempts.
	ErrorLogFunc func(*slog.Logger, T)
	Now          func() time.Time

	LastCollectionSuccessHelp    string
	LastCollectionTimestampHelp  string
	LastSuccessfulCollectionHelp string
	CollectionDurationHelp       string

	// SyncRefreshTimeout bounds a scrape-triggered synchronous refresh when the
	// cache is empty or stale and the background refresh loop is not running. A
	// zero value keeps the historical unbounded behavior. Background refreshes
	// use the context passed to Start.
	SyncRefreshTimeout time.Duration
}

type SnapshotCollector[T any] struct {
	namespace          string
	logger             *slog.Logger
	snapshotter        Snapshotter[T]
	refreshInterval    time.Duration
	syncRefreshTimeout time.Duration
	statusFunc         func(T) SnapshotStatus
	describeFunc       func(chan<- *prometheus.Desc)
	collectFunc        func(chan<- prometheus.Metric, T, time.Time)
	errorLogFunc       func(*slog.Logger, T)
	now                func() time.Time

	mu                       sync.Mutex
	cond                     *sync.Cond
	initialized              bool
	backgroundStarted        bool
	refreshing               bool
	snapshot                 T
	snapshotStatus           SnapshotStatus
	lastSuccessfulCollection time.Time
	collectionDuration       collectionDurationHistogram

	lastCollectionSuccessDesc    *prometheus.Desc
	lastCollectionTimestampDesc  *prometheus.Desc
	lastSuccessfulCollectionDesc *prometheus.Desc
	collectionDurationDesc       *prometheus.Desc
}

func NewSnapshotCollector[T any](options SnapshotCollectorOptions[T]) *SnapshotCollector[T] {
	namespace := options.Namespace
	if namespace == "" {
		namespace = "exporter"
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	snapshotter := options.Snapshotter
	if snapshotter == nil {
		snapshotter = zeroSnapshotter[T]{}
	}
	refreshInterval := options.RefreshInterval
	if refreshInterval <= 0 {
		refreshInterval = DefaultSnapshotRefreshInterval
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	statusFunc := options.StatusFunc
	if statusFunc == nil {
		statusFunc = zeroSnapshotStatus[T]
	}

	collector := &SnapshotCollector[T]{
		namespace:          namespace,
		logger:             logger,
		snapshotter:        snapshotter,
		refreshInterval:    refreshInterval,
		syncRefreshTimeout: options.SyncRefreshTimeout,
		statusFunc:         statusFunc,
		describeFunc:       options.DescribeFunc,
		collectFunc:        options.CollectFunc,
		errorLogFunc:       options.ErrorLogFunc,
		now:                now,

		lastCollectionSuccessDesc: prometheus.NewDesc(
			namespace+"_last_collection_success",
			defaultString(options.LastCollectionSuccessHelp, "Whether the last collection succeeded"),
			nil,
			nil,
		),
		lastCollectionTimestampDesc: prometheus.NewDesc(
			namespace+"_last_collection_timestamp_seconds",
			defaultString(options.LastCollectionTimestampHelp, "Unix timestamp of the last collection attempt"),
			nil,
			nil,
		),
		lastSuccessfulCollectionDesc: prometheus.NewDesc(
			namespace+"_last_successful_collection_timestamp_seconds",
			defaultString(options.LastSuccessfulCollectionHelp, "Unix timestamp of the last successful collection"),
			nil,
			nil,
		),
		collectionDurationDesc: prometheus.NewDesc(
			namespace+"_collection_duration_seconds",
			defaultString(options.CollectionDurationHelp, "Time spent refreshing collection data"),
			nil,
			nil,
		),
		collectionDuration: newCollectionDurationHistogram(),
	}
	collector.cond = sync.NewCond(&collector.mu)
	return collector
}

func (c *SnapshotCollector[T]) Describe(ch chan<- *prometheus.Desc) {
	if c.describeFunc != nil {
		c.describeFunc(ch)
	}
	ch <- c.lastCollectionSuccessDesc
	ch <- c.lastCollectionTimestampDesc
	ch <- c.lastSuccessfulCollectionDesc
	ch <- c.collectionDurationDesc
}

func (c *SnapshotCollector[T]) Start(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	c.mu.Lock()
	if c.backgroundStarted {
		c.mu.Unlock()
		return
	}
	c.backgroundStarted = true
	c.mu.Unlock()

	go c.refreshLoop(ctx)
}

func (c *SnapshotCollector[T]) Collect(ch chan<- prometheus.Metric) {
	now := c.now()
	state := c.currentSnapshot(now)

	if c.collectFunc != nil {
		c.collectFunc(ch, state.snapshot, now)
	}

	ch <- prometheus.MustNewConstMetric(
		c.lastCollectionSuccessDesc,
		prometheus.GaugeValue,
		metric.BoolFloat(state.status.Success),
	)
	ch <- prometheus.MustNewConstMetric(
		c.lastCollectionTimestampDesc,
		prometheus.GaugeValue,
		metric.UnixTimestamp(state.status.AttemptTime),
	)
	ch <- prometheus.MustNewConstMetric(
		c.lastSuccessfulCollectionDesc,
		prometheus.GaugeValue,
		metric.UnixTimestamp(state.lastSuccessfulCollection),
	)
	ch <- prometheus.MustNewConstHistogram(
		c.collectionDurationDesc,
		state.collectionDuration.count,
		state.collectionDuration.sum,
		state.collectionDuration.buckets,
	)
}

func (c *SnapshotCollector[T]) refreshLoop(ctx context.Context) {
	if ctx.Err() == nil {
		c.refresh(ctx, c.now())
	}

	ticker := time.NewTicker(c.refreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			c.mu.Lock()
			c.backgroundStarted = false
			c.mu.Unlock()
			return
		case <-ticker.C:
			c.refresh(ctx, c.now())
		}
	}
}

func (c *SnapshotCollector[T]) refresh(ctx context.Context, now time.Time) {
	if !c.beginRefresh() {
		return
	}

	snapshot, duration := c.collectSnapshot(ctx, now)
	c.logSnapshotErrors(snapshot)
	c.finishRefresh(snapshot, duration, now)
}

func (c *SnapshotCollector[T]) currentSnapshot(now time.Time) snapshotState[T] {
	for {
		c.mu.Lock()
		if c.initialized && (c.backgroundStarted || now.Sub(c.snapshotStatus.AttemptTime) < c.refreshInterval) {
			state := c.snapshotStateLocked()
			c.mu.Unlock()
			return state
		}
		if c.refreshing {
			for c.refreshing {
				c.cond.Wait()
			}
			now = c.now()
			c.mu.Unlock()
			continue
		}
		c.refreshing = true
		c.mu.Unlock()

		ctx := context.Background()
		cancel := func() {}
		if c.syncRefreshTimeout > 0 {
			ctx, cancel = context.WithTimeout(ctx, c.syncRefreshTimeout)
		}
		snapshot, duration := c.collectSnapshot(ctx, now)
		cancel()
		c.logSnapshotErrors(snapshot)

		c.mu.Lock()
		c.storeSnapshotLocked(snapshot, duration, now)
		c.refreshing = false
		c.cond.Broadcast()
		state := c.snapshotStateLocked()
		c.mu.Unlock()
		return state
	}
}

func (c *SnapshotCollector[T]) beginRefresh() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.refreshing {
		return false
	}
	c.refreshing = true
	return true
}

func (c *SnapshotCollector[T]) finishRefresh(snapshot T, duration time.Duration, refreshTime time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.storeSnapshotLocked(snapshot, duration, refreshTime)
	c.refreshing = false
	c.cond.Broadcast()
}

func (c *SnapshotCollector[T]) storeSnapshotLocked(snapshot T, duration time.Duration, refreshTime time.Time) {
	status := c.statusFunc(snapshot)
	if status.AttemptTime.IsZero() {
		status.AttemptTime = refreshTime
	}
	if status.Success {
		c.lastSuccessfulCollection = status.AttemptTime
	}

	c.snapshot = snapshot
	c.snapshotStatus = status
	c.collectionDuration.observe(duration)
	c.initialized = true
}

func (c *SnapshotCollector[T]) snapshotStateLocked() snapshotState[T] {
	return snapshotState[T]{
		snapshot:                 c.snapshot,
		status:                   c.snapshotStatus,
		lastSuccessfulCollection: c.lastSuccessfulCollection,
		collectionDuration:       c.collectionDuration.clone(),
	}
}

func (c *SnapshotCollector[T]) collectSnapshot(ctx context.Context, now time.Time) (T, time.Duration) {
	start := c.now()
	snapshot := c.snapshotter.Snapshot(ctx, now)
	duration := c.now().Sub(start)
	if duration < 0 {
		duration = 0
	}
	return snapshot, duration
}

func (c *SnapshotCollector[T]) logSnapshotErrors(snapshot T) {
	if c.errorLogFunc != nil {
		c.errorLogFunc(c.logger, snapshot)
	}
}

type snapshotState[T any] struct {
	snapshot                 T
	status                   SnapshotStatus
	lastSuccessfulCollection time.Time
	collectionDuration       collectionDurationHistogram
}

type collectionDurationHistogram struct {
	count   uint64
	sum     float64
	buckets map[float64]uint64
}

func newCollectionDurationHistogram() collectionDurationHistogram {
	buckets := make(map[float64]uint64, len(prometheus.DefBuckets)+1)
	for _, bound := range prometheus.DefBuckets {
		buckets[bound] = 0
	}
	buckets[math.Inf(1)] = 0
	return collectionDurationHistogram{
		buckets: buckets,
	}
}

func (h *collectionDurationHistogram) observe(duration time.Duration) {
	seconds := duration.Seconds()
	h.count++
	h.sum += seconds
	for _, upperBound := range prometheus.DefBuckets {
		if seconds <= upperBound {
			h.buckets[upperBound]++
		}
	}
	h.buckets[math.Inf(1)]++
}

func (h *collectionDurationHistogram) clone() collectionDurationHistogram {
	clone := collectionDurationHistogram{
		count:   h.count,
		sum:     h.sum,
		buckets: make(map[float64]uint64, len(h.buckets)),
	}
	for upperBound, count := range h.buckets {
		clone.buckets[upperBound] = count
	}
	return clone
}

type zeroSnapshotter[T any] struct{}

func (zeroSnapshotter[T]) Snapshot(context.Context, time.Time) T {
	var snapshot T
	return snapshot
}

func zeroSnapshotStatus[T any](T) SnapshotStatus {
	return SnapshotStatus{}
}

func defaultString(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
