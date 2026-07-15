package featurekit

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
)

type SpecOptions struct {
	FeatureName             string
	DefaultRefreshInterval  time.Duration
	FallbackRefreshInterval time.Duration
}

type FeatureSpec[C any, S any] struct {
	FeatureName             string
	DefaultRefreshInterval  time.Duration
	FallbackRefreshInterval time.Duration
	Config                  C
	RegisterFlagsFunc       func(app *kingpin.Application, ctx FlagContext, config *C)
	PrepareConfigFunc       func(featureName string, config C) (C, error)
	ValidateConfigFunc      func(config C) error
	NewSnapshotterFunc      func(ctx CollectorContext[C]) (framework.Snapshotter[S], error)
	NewCollectorFunc        NewCollectorFunc[S]
	RuntimeConfigFunc       func(ctx RuntimeConfigContext[C]) []any
	Smoke                   SmokeSpec
	SmokeFunc               func(ctx SmokeContext[C]) SmokeSpec
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

type SmokeContext[C any] struct {
	FeatureName string
	Config      C
}

type NewCollectorFunc[S any] func(featureName string, namespace string, logger *slog.Logger, snapshotter framework.Snapshotter[S], refreshInterval time.Duration) framework.StartableCollector

type SmokeSpec = framework.SmokeSpec

type Feature[C any, S any] struct {
	featureName            string
	defaultRefreshInterval time.Duration
	refreshInterval        time.Duration
	config                 C
	configMu               sync.Mutex
	preparedConfig         C
	configPrepared         bool
	registerFlagsFunc      func(app *kingpin.Application, ctx FlagContext, config *C)
	prepareConfigFunc      func(featureName string, config C) (C, error)
	validateConfigFunc     func(config C) error
	newSnapshotterFunc     func(ctx CollectorContext[C]) (framework.Snapshotter[S], error)
	newCollectorFunc       NewCollectorFunc[S]
	runtimeConfigFunc      func(ctx RuntimeConfigContext[C]) []any
	smoke                  SmokeSpec
	smokeFunc              func(ctx SmokeContext[C]) SmokeSpec
}

func NewFeature[C any, S any](spec FeatureSpec[C, S]) *Feature[C, S] {
	featureName := spec.FeatureName
	if featureName == "" {
		featureName = "exporter"
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
		prepareConfigFunc:      spec.PrepareConfigFunc,
		validateConfigFunc:     spec.ValidateConfigFunc,
		newSnapshotterFunc:     spec.NewSnapshotterFunc,
		newCollectorFunc:       spec.NewCollectorFunc,
		runtimeConfigFunc:      spec.RuntimeConfigFunc,
		smoke:                  spec.Smoke,
		smokeFunc:              spec.SmokeFunc,
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
	config, err := f.prepareConfig()
	if err != nil {
		return err
	}
	if f.validateConfigFunc != nil {
		if err := f.validateConfigFunc(config); err != nil {
			return err
		}
	}
	if f.newCollectorFunc == nil {
		return nil
	}

	collectorContext := CollectorContext[C]{
		FeatureName:     f.featureName,
		Framework:       ctx,
		Config:          config,
		RefreshInterval: framework.NormalizeDuration(f.refreshInterval, f.defaultRefreshInterval),
	}
	var snapshotter framework.Snapshotter[S]
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
	if collector == nil {
		return fmt.Errorf("create %s collector: collector factory returned nil", f.featureName)
	}
	collectorStartContext := ctx.Context
	if collectorStartContext == nil {
		collectorStartContext = context.Background()
	}
	if err := framework.RegisterAndStartCollectors(collectorStartContext, registry, collector); err != nil {
		return fmt.Errorf("register %s collector: %w", f.featureName, err)
	}
	return nil
}

func (f *Feature[C, S]) RuntimeConfig() []any {
	refreshInterval := framework.NormalizeDuration(f.refreshInterval, f.defaultRefreshInterval)
	configValue := f.config
	var configErr error
	if config, err := f.prepareConfig(); err == nil {
		configValue = config
	} else {
		configErr = err
	}
	config := []any{
		"refresh_interval", refreshInterval,
	}
	if f.runtimeConfigFunc != nil {
		config = append(config, f.runtimeConfigFunc(RuntimeConfigContext[C]{
			FeatureName:     f.featureName,
			Config:          configValue,
			RefreshInterval: refreshInterval,
		})...)
	}
	if configErr != nil {
		config = append(config, "config_error", configErr.Error())
	}
	return config
}

func (f *Feature[C, S]) prepareConfig() (C, error) {
	f.configMu.Lock()
	defer f.configMu.Unlock()

	if f.configPrepared {
		return f.preparedConfig, nil
	}
	config := f.config
	if f.prepareConfigFunc != nil {
		var err error
		config, err = f.prepareConfigFunc(f.featureName, config)
		if err != nil {
			return config, err
		}
	}
	f.preparedConfig = config
	f.configPrepared = true
	return config, nil
}

func (f *Feature[C, S]) SmokeSpec() SmokeSpec {
	if f.smokeFunc != nil {
		return f.smokeFunc(SmokeContext[C]{
			FeatureName: f.featureName,
			Config:      f.config,
		})
	}
	return f.smoke
}
