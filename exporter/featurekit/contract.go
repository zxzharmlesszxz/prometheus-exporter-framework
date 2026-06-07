package featurekit

import (
	"time"

	"github.com/alecthomas/kingpin/v2"
	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
)

type FeatureContract[C any, S any] interface {
	DefaultRefreshInterval() time.Duration
	DefaultConfig() C
	RegisterFlags(app *kingpin.Application, ctx FlagContext, config *C)
	ValidateConfig(config C) error
	NewSnapshotter(ctx CollectorContext[C]) (framework.Snapshotter[S], error)
	DefaultSnapshotter() framework.Snapshotter[S]
	NewMetrics(ctx SnapshotMetricsContext[S]) SnapshotMetrics[S]
	SnapshotStatus(snapshot S) framework.SnapshotStatus
	RuntimeConfig(ctx RuntimeConfigContext[C]) []any
	SmokeSpec(ctx SmokeContext[C]) SmokeSpec
}

type FeatureDefaults[C any, S any] struct{}

func (FeatureDefaults[C, S]) DefaultRefreshInterval() time.Duration {
	return 0
}

func (FeatureDefaults[C, S]) DefaultConfig() C {
	var config C
	return config
}

func (FeatureDefaults[C, S]) RegisterFlags(app *kingpin.Application, ctx FlagContext, config *C) {}

func (FeatureDefaults[C, S]) ValidateConfig(config C) error { return nil }

func (FeatureDefaults[C, S]) NewSnapshotter(ctx CollectorContext[C]) (framework.Snapshotter[S], error) { return nil, nil }

func (FeatureDefaults[C, S]) DefaultSnapshotter() framework.Snapshotter[S] {
	return nil
}

func (FeatureDefaults[C, S]) NewMetrics(ctx SnapshotMetricsContext[S]) SnapshotMetrics[S] { return nil }

func (FeatureDefaults[C, S]) SnapshotStatus(snapshot S) framework.SnapshotStatus { return framework.SnapshotStatus{} }

func (FeatureDefaults[C, S]) RuntimeConfig(ctx RuntimeConfigContext[C]) []any { return nil }

func (FeatureDefaults[C, S]) SmokeSpec(ctx SmokeContext[C]) SmokeSpec { return SmokeSpec{} }

func NewContractSnapshotFeatureSpec[C any, S any](options SpecOptions, contract FeatureContract[C, S]) FeatureSpec[C, S] {
	if contract == nil {
		contract = FeatureDefaults[C, S]{}
	}
	return NewSnapshotFeatureSpec(SnapshotFeatureSpec[C, S]{
		Options:                options,
		DefaultRefreshInterval: contract.DefaultRefreshInterval(),
		Config:                 contract.DefaultConfig(),
		RegisterFlagsFunc:      contract.RegisterFlags,
		ValidateConfigFunc:     contract.ValidateConfig,
		NewSnapshotterFunc:     contract.NewSnapshotter,
		DefaultSnapshotter:     contract.DefaultSnapshotter(),
		MetricsFunc:            contract.NewMetrics,
		StatusFunc:             contract.SnapshotStatus,
		RuntimeConfigFunc:      contract.RuntimeConfig,
		SmokeFunc:              contract.SmokeSpec,
	})
}

func NewContractFeature[C any, S any](options SpecOptions, contract FeatureContract[C, S]) *Feature[C, S] {
	return NewFeature(NewContractSnapshotFeatureSpec[C, S](options, contract))
}
