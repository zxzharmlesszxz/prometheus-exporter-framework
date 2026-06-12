package __FEATURE_NAME__

import (
	"context"
	"time"

	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"

	"__GO_MODULE__/internal/__FEATURE_NAME__check"
)

func NewDefaultSnapshotEngine() featurekit.SnapshotEngine[Snapshot] {
	engine, err := newSnapshotEngine(NewDefaultConfig())
	if err != nil {
		panic(err)
	}
	return engine
}

func NewSnapshotEngine(ctx featurekit.CollectorContext[Config]) (featurekit.SnapshotEngine[Snapshot], error) {
	config, _, _, err := ResolveFeatureConfig(ctx.FeatureName, ctx.Config)
	if err != nil {
		return nil, err
	}
	return newSnapshotEngine(config)
}

func FeatureSnapshotStatus(snapshot Snapshot) framework.SnapshotStatus {
	return framework.SnapshotStatus{
		AttemptTime: snapshot.__FEATURE_NAME__.AttemptTime,
		Success:     snapshot.__FEATURE_NAME__.Success,
	}
}

func newSnapshotEngine(_ Config) (featurekit.SnapshotEngine[Snapshot], error) {
	checker := __FEATURE_NAME__check.NewChecker()
	return featurekit.SnapshotEngineFunc[Snapshot](func(ctx context.Context, now time.Time) Snapshot {
		return Snapshot{
			__FEATURE_NAME__: checker.Snapshot(ctx, now),
		}
	}), nil
}
