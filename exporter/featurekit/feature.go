package featurekit

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
)

type SpecOptions struct {
	FeatureName             string
	DefaultFeatureName      string
	DefaultRefreshInterval  time.Duration
	FallbackRefreshInterval time.Duration
}

type FeatureSpec[C any, S any] struct {
	FeatureName             string
	DefaultFeatureName      string
	DefaultRefreshInterval  time.Duration
	FallbackRefreshInterval time.Duration
	Config                  C
	RegisterFlagsFunc       func(app *kingpin.Application, ctx FlagContext, config *C)
	ValidateConfigFunc      func(config C) error
	NewSnapshotterFunc      func(ctx CollectorContext[C]) (framework.Snapshotter[S], error)
	NewCollectorFunc        NewCollectorFunc[S]
	RuntimeConfigFunc       func(ctx RuntimeConfigContext[C]) []any
}

type FlagContext struct {
	FeatureName            string
	DefaultRefreshInterval time.Duration
}

type CollectorContext[C any] struct {
	FeatureName     string
	Framework       framework.FeatureContext
	Config          C
	RefreshInterval time.Duration
}

type RuntimeConfigContext[C any] struct {
	FeatureName     string
	Config          C
	RefreshInterval time.Duration
}

type NewCollectorFunc[S any] func(featureName string, namespace string, logger *slog.Logger, snapshotter framework.Snapshotter[S], refreshInterval time.Duration) framework.StartableCollector

type SmokeSpec struct {
	ServerArgs    []string
	WantMetrics   []string
	RejectMetrics []string
}

type Feature[C any, S any] struct {
	featureName            string
	defaultRefreshInterval time.Duration
	refreshInterval        time.Duration
	config                 C
	registerFlagsFunc      func(app *kingpin.Application, ctx FlagContext, config *C)
	validateConfigFunc     func(config C) error
	newSnapshotterFunc     func(ctx CollectorContext[C]) (framework.Snapshotter[S], error)
	newCollectorFunc       NewCollectorFunc[S]
	runtimeConfigFunc      func(ctx RuntimeConfigContext[C]) []any
}

func NewFeature[C any, S any](spec FeatureSpec[C, S]) *Feature[C, S] {
	defaultFeatureName := spec.DefaultFeatureName
	if defaultFeatureName == "" {
		defaultFeatureName = "exporter"
	}
	featureName := spec.FeatureName
	if featureName == "" {
		featureName = defaultFeatureName
	}
	fallbackRefreshInterval := spec.FallbackRefreshInterval
	if fallbackRefreshInterval <= 0 {
		fallbackRefreshInterval = framework.DefaultSnapshotRefreshInterval
	}
	defaultRefreshInterval := spec.DefaultRefreshInterval
	if defaultRefreshInterval <= 0 {
		defaultRefreshInterval = fallbackRefreshInterval
	}

	return &Feature[C, S]{
		featureName:            featureName,
		defaultRefreshInterval: defaultRefreshInterval,
		refreshInterval:        defaultRefreshInterval,
		config:                 spec.Config,
		registerFlagsFunc:      spec.RegisterFlagsFunc,
		validateConfigFunc:     spec.ValidateConfigFunc,
		newSnapshotterFunc:     spec.NewSnapshotterFunc,
		newCollectorFunc:       spec.NewCollectorFunc,
		runtimeConfigFunc:      spec.RuntimeConfigFunc,
	}
}

func (f *Feature[C, S]) FeatureName() string {
	return f.featureName
}

func (f *Feature[C, S]) RegisterFlags(app *kingpin.Application) {
	app.Flag(
		f.featureName+".refresh-interval", "How often exporter refreshes "+f.featureName+" data",
	).Default(f.defaultRefreshInterval.String()).DurationVar(&f.refreshInterval)
	if f.registerFlagsFunc != nil {
		f.registerFlagsFunc(app, FlagContext{
			FeatureName:            f.featureName,
			DefaultRefreshInterval: f.defaultRefreshInterval,
		}, &f.config)
	}
}

func (f *Feature[C, S]) RegisterCollectors(ctx framework.FeatureContext, registry *prometheus.Registry) error {
	if f.validateConfigFunc != nil {
		if err := f.validateConfigFunc(f.config); err != nil {
			return err
		}
	}
	if f.newCollectorFunc == nil {
		return nil
	}

	collectorContext := CollectorContext[C]{
		FeatureName:     f.featureName,
		Framework:       ctx,
		Config:          f.config,
		RefreshInterval: framework.NormalizeDuration(f.refreshInterval, f.defaultRefreshInterval),
	}
	var snapshotter framework.Snapshotter[S]
	var err error
	if f.newSnapshotterFunc != nil {
		snapshotter, err = f.newSnapshotterFunc(collectorContext)
		if err != nil {
			return err
		}
	}

	collector := f.newCollectorFunc(
		f.featureName,
		ctx.Namespace,
		ctx.Logger,
		snapshotter,
		collectorContext.RefreshInterval,
	)
	if err := framework.RegisterAndStartCollectors(context.Background(), registry, collector); err != nil {
		return fmt.Errorf("register %s collector: %w", f.featureName, err)
	}
	return nil
}

func (f *Feature[C, S]) RuntimeConfig() []any {
	refreshInterval := framework.NormalizeDuration(f.refreshInterval, f.defaultRefreshInterval)
	config := []any{
		"refresh_interval", refreshInterval,
	}
	if f.runtimeConfigFunc != nil {
		config = append(config, f.runtimeConfigFunc(RuntimeConfigContext[C]{
			FeatureName:     f.featureName,
			Config:          f.config,
			RefreshInterval: refreshInterval,
		})...)
	}
	return config
}
