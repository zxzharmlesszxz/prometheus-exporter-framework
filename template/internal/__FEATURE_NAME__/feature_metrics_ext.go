package __FEATURE_NAME__

import (
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"
)

func NewFeatureMetricHandlers() featurekit.FeatureMetricHandlers[Snapshot] {
	return featurekit.FeatureMetricHandlers[Snapshot]{
		Collect:  CollectFeatureMetrics,
		LogError: LogFeatureSnapshotError,
	}
}

func CollectFeatureMetrics(ctx featurekit.FeatureMetricsContext[Snapshot], ch chan<- prometheus.Metric, snapshot Snapshot, _ time.Time) {
	if !snapshot.__FEATURE_NAME__.Success {
		return
	}
	ch <- prometheus.MustNewConstMetric(
		ctx.Descriptors.Get(metricExampleValue),
		prometheus.GaugeValue,
		snapshot.__FEATURE_NAME__.Value,
	)
}

func LogFeatureSnapshotError(ctx featurekit.FeatureMetricsContext[Snapshot], logger *slog.Logger, snapshot Snapshot) {
	if snapshot.__FEATURE_NAME__.Err != nil {
		logger.Error("data collection failed",
			"feature", ctx.FeatureName,
			"err", snapshot.__FEATURE_NAME__.Err,
		)
	}
}
