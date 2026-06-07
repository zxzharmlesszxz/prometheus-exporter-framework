package featurekit

import (
	"context"
	"log/slog"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
)

type SnapshotMetrics[S any] interface {
	Describe(ch chan<- *prometheus.Desc)
	Collect(ch chan<- prometheus.Metric, snapshot S, now time.Time)
}

type SnapshotErrorLogger[S any] interface {
	LogSnapshotError(logger *slog.Logger, snapshot S)
}

// SnapshotEngine is the feature-owned snapshot source used by snapshot features.
// It is intentionally the same shape as exporter.Snapshotter so feature packages
// can expose domain engines without local gatherer wrappers.
type SnapshotEngine[S any] interface {
	Snapshot(context.Context, time.Time) S
}

// SnapshotEngineFunc is a function that satisfies SnapshotEngine.
//
// It is useful for feature-level aggregate snapshots where the feature calls
// one or more check engines and combines their snapshots into the exported
// feature snapshot type.
type SnapshotEngineFunc[S any] func(context.Context, time.Time) S

func (f SnapshotEngineFunc[S]) Snapshot(ctx context.Context, now time.Time) S { return f(ctx, now) }

// SnapshotEngineFactory constructs a feature-owned snapshot engine from resolved
// collector context.
type SnapshotEngineFactory[C any, S any] func(ctx CollectorContext[C]) (SnapshotEngine[S], error)

type SnapshotMetricsContext[S any] struct {
	FeatureName string
	Namespace   string
	Snapshotter framework.Snapshotter[S]
}

type SnapshotMetricsFunc[S any] func(ctx SnapshotMetricsContext[S]) SnapshotMetrics[S]

type SnapshotFeatureSpec[C any, S any] struct {
	Options                SpecOptions
	DefaultRefreshInterval time.Duration
	Config                 C
	RegisterFlagsFunc      func(app *kingpin.Application, ctx FlagContext, config *C)
	ValidateConfigFunc     func(config C) error
	NewSnapshotterFunc     func(ctx CollectorContext[C]) (framework.Snapshotter[S], error)
	DefaultSnapshotter     framework.Snapshotter[S]
	MetricsFunc            SnapshotMetricsFunc[S]
	StatusFunc             func(S) framework.SnapshotStatus
	ErrorLogFunc           func(*slog.Logger, S)
	RuntimeConfigFunc      func(ctx RuntimeConfigContext[C]) []any
	Smoke                  SmokeSpec
	SmokeFunc              func(ctx SmokeContext[C]) SmokeSpec
}

type SnapshotCollectorOptions[S any] struct {
	FeatureName            string
	DefaultFeatureName     string
	Namespace              string
	DefaultMetricNamespace string
	Logger                 *slog.Logger
	Snapshotter            framework.Snapshotter[S]
	DefaultSnapshotter     framework.Snapshotter[S]
	RefreshInterval        time.Duration
	DefaultRefreshInterval time.Duration
	StatusFunc             func(S) framework.SnapshotStatus
	DescribeFunc           func(chan<- *prometheus.Desc)
	CollectFunc            func(chan<- prometheus.Metric, S, time.Time)
	ErrorLogFunc           func(*slog.Logger, S)
	Now                    func() time.Time
}

type SnapshotMetricsCollectorOptions[S any] struct {
	SnapshotCollectorOptions[S]
	MetricsFunc SnapshotMetricsFunc[S]
}

func NewSnapshotFeatureSpec[C any, S any](spec SnapshotFeatureSpec[C, S]) FeatureSpec[C, S] {
	defaultRefreshInterval := spec.Options.DefaultRefreshInterval
	if defaultRefreshInterval <= 0 {
		defaultRefreshInterval = spec.DefaultRefreshInterval
	}
	fallbackRefreshInterval := spec.Options.FallbackRefreshInterval
	if fallbackRefreshInterval <= 0 {
		fallbackRefreshInterval = spec.DefaultRefreshInterval
	}
	if fallbackRefreshInterval <= 0 {
		fallbackRefreshInterval = framework.DefaultSnapshotRefreshInterval
	}

	return FeatureSpec[C, S]{
		FeatureName:             spec.Options.FeatureName,
		DefaultRefreshInterval:  defaultRefreshInterval,
		FallbackRefreshInterval: fallbackRefreshInterval,
		Config:                  spec.Config,
		RegisterFlagsFunc:       spec.RegisterFlagsFunc,
		ValidateConfigFunc:      spec.ValidateConfigFunc,
		NewSnapshotterFunc:      spec.NewSnapshotterFunc,
		NewCollectorFunc: func(featureName string, namespace string, logger *slog.Logger, snapshotter framework.Snapshotter[S], refreshInterval time.Duration) framework.StartableCollector {
			return NewSnapshotMetricsCollector(SnapshotMetricsCollectorOptions[S]{
				SnapshotCollectorOptions: SnapshotCollectorOptions[S]{
					FeatureName:            featureName,
					Namespace:              namespace,
					Logger:                 logger,
					Snapshotter:            snapshotter,
					DefaultSnapshotter:     spec.DefaultSnapshotter,
					RefreshInterval:        refreshInterval,
					DefaultRefreshInterval: defaultRefreshInterval,
					StatusFunc:             spec.StatusFunc,
					ErrorLogFunc:           spec.ErrorLogFunc,
				},
				MetricsFunc: spec.MetricsFunc,
			})
		},
		RuntimeConfigFunc: spec.RuntimeConfigFunc,
		Smoke:             spec.Smoke,
		SmokeFunc:         spec.SmokeFunc,
	}
}

func ResolveSnapshotCollectorOptions[S any](options SnapshotCollectorOptions[S]) SnapshotCollectorOptions[S] {
	if options.DefaultFeatureName == "" {
		options.DefaultFeatureName = "exporter"
	}
	if options.FeatureName == "" {
		options.FeatureName = options.DefaultFeatureName
	}
	if options.DefaultMetricNamespace == "" {
		options.DefaultMetricNamespace = "exporter"
	}
	if options.Namespace == "" {
		options.Namespace = options.DefaultMetricNamespace
	}
	if options.Logger == nil {
		options.Logger = slog.Default()
	}
	if options.Snapshotter == nil {
		options.Snapshotter = options.DefaultSnapshotter
	}
	if options.DefaultRefreshInterval <= 0 {
		options.DefaultRefreshInterval = framework.DefaultSnapshotRefreshInterval
	}
	if options.RefreshInterval <= 0 {
		options.RefreshInterval = options.DefaultRefreshInterval
	}
	return options
}

func NewSnapshotMetricsCollector[S any](options SnapshotMetricsCollectorOptions[S]) *framework.SnapshotCollector[S] {
	collectorOptions := ResolveSnapshotCollectorOptions(options.SnapshotCollectorOptions)
	metrics := newSnapshotMetrics(SnapshotMetricsContext[S]{
		FeatureName: collectorOptions.FeatureName,
		Namespace:   collectorOptions.Namespace,
		Snapshotter: collectorOptions.Snapshotter,
	}, options.MetricsFunc)

	collectorOptions.DescribeFunc = metrics.Describe
	collectorOptions.CollectFunc = metrics.Collect
	if options.ErrorLogFunc != nil {
		collectorOptions.ErrorLogFunc = options.ErrorLogFunc
	} else if logger, ok := metrics.(SnapshotErrorLogger[S]); ok {
		collectorOptions.ErrorLogFunc = logger.LogSnapshotError
	}
	return NewSnapshotCollector(collectorOptions)
}

func NewSnapshotCollector[S any](options SnapshotCollectorOptions[S]) *framework.SnapshotCollector[S] {
	options = ResolveSnapshotCollectorOptions(options)
	return framework.NewSnapshotCollector(framework.SnapshotCollectorOptions[S]{
		Namespace:       options.Namespace,
		Logger:          options.Logger,
		Snapshotter:     options.Snapshotter,
		RefreshInterval: options.RefreshInterval,
		StatusFunc:      options.StatusFunc,
		DescribeFunc:    options.DescribeFunc,
		CollectFunc:     options.CollectFunc,
		ErrorLogFunc:    options.ErrorLogFunc,
		Now:             options.Now,

		LastCollectionSuccessHelp:    "Whether the last " + options.FeatureName + " data collection succeeded",
		LastCollectionTimestampHelp:  "Unix timestamp of the last " + options.FeatureName + " data collection attempt",
		LastSuccessfulCollectionHelp: "Unix timestamp of the last successful " + options.FeatureName + " data collection",
	})
}

func newSnapshotMetrics[S any](ctx SnapshotMetricsContext[S], metricsFunc SnapshotMetricsFunc[S]) SnapshotMetrics[S] {
	if metricsFunc == nil {
		return noopSnapshotMetrics[S]{}
	}
	metrics := metricsFunc(ctx)
	if metrics == nil {
		return noopSnapshotMetrics[S]{}
	}
	return metrics
}

type noopSnapshotMetrics[S any] struct{}

func (noopSnapshotMetrics[S]) Describe(ch chan<- *prometheus.Desc) {}

func (noopSnapshotMetrics[S]) Collect(ch chan<- prometheus.Metric, snapshot S, now time.Time) {}
