package featurekit

import (
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
)

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
