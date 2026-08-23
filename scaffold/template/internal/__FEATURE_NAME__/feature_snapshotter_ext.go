package __FEATURE_NAME__

import (
	"context"
	"time"

	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"

	"__GO_MODULE__/internal/__FEATURE_NAME__check"
)

func NewDefaultSnapshotEngine() featurekit.SnapshotEngine[Snapshot] {
	return newSnapshotEngine(NewDefaultConfig())
}

func NewSnapshotEngine(ctx featurekit.CollectorContext[Config]) (featurekit.SnapshotEngine[Snapshot], error) {
	return newSnapshotEngine(ctx.Config), nil
}

func FeatureSnapshotStatus(snapshot Snapshot) framework.SnapshotStatus {
	return framework.SnapshotStatus{
		AttemptTime: snapshot.__FEATURE_NAME__.AttemptTime,
		Success:     snapshot.__FEATURE_NAME__.Success,
	}
}

func newSnapshotEngine(_ Config) featurekit.SnapshotEngine[Snapshot] {
	checker := __FEATURE_NAME__check.NewChecker()
	return featurekit.SnapshotEngineFunc[Snapshot](func(ctx context.Context, now time.Time) Snapshot {
		return Snapshot{
			__FEATURE_NAME__: checker.Snapshot(ctx, now),
		}
	})
}
