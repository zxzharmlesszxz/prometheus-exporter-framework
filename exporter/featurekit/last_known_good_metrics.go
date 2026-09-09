package featurekit

import (
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type LastKnownGoodMetricIDs struct {
	LastSuccessTimestampSeconds string
	ConsecutiveFailures         string
	DataAvailable               string
	DataStale                   string
}

type LastKnownGoodMetricDescs struct {
	LastSuccessTimestampDesc *prometheus.Desc
	ConsecutiveFailuresDesc  *prometheus.Desc
	DataAvailableDesc        *prometheus.Desc
	DataStaleDesc            *prometheus.Desc
}

func LastKnownGoodMetricIDsFor(source string) LastKnownGoodMetricIDs {
	if strings.TrimSpace(source) == "" {
		panic("last known good metric source is required")
	}
	return LastKnownGoodMetricIDs{
		LastSuccessTimestampSeconds: source + "_last_success_timestamp_seconds",
		ConsecutiveFailures:         source + "_consecutive_failures",
		DataAvailable:               source + "_data_available",
		DataStale:                   source + "_data_stale",
	}
}

func LastKnownGoodMetricSpecs(source string, labels []string) []FeatureMetricSpec {
	ids := LastKnownGoodMetricIDsFor(source)
	return []FeatureMetricSpec{
		{
			ID:     ids.LastSuccessTimestampSeconds,
			Scope:  MetricScopeFeature,
			Name:   "_" + source + "_last_success_timestamp_seconds",
			Help:   "Unix timestamp of the last successful " + source + " refresh.",
			Labels: labels,
		},
		{
			ID:     ids.ConsecutiveFailures,
			Scope:  MetricScopeFeature,
			Name:   "_" + source + "_consecutive_failures",
			Help:   "Current number of consecutive failed " + source + " refresh attempts.",
			Labels: labels,
		},
		{
			ID:     ids.DataAvailable,
			Scope:  MetricScopeFeature,
			Name:   "_" + source + "_data_available",
			Help:   "Whether last-known-good " + source + " data is available.",
			Labels: labels,
		},
		{
			ID:     ids.DataStale,
			Scope:  MetricScopeFeature,
			Name:   "_" + source + "_data_stale",
			Help:   "Whether last-known-good " + source + " data is older than its stale threshold.",
			Labels: labels,
		},
	}
}

func (ids LastKnownGoodMetricIDs) Descs(descs FeatureMetricDescriptors) LastKnownGoodMetricDescs {
	return LastKnownGoodMetricDescs{
		LastSuccessTimestampDesc: descs.Get(ids.LastSuccessTimestampSeconds),
		ConsecutiveFailuresDesc:  descs.Get(ids.ConsecutiveFailures),
		DataAvailableDesc:        descs.Get(ids.DataAvailable),
		DataStaleDesc:            descs.Get(ids.DataStale),
	}
}

func CollectLastKnownGoodMetrics[S any, V any](
	ctx FeatureMetricsContext[S],
	ch chan<- prometheus.Metric,
	ids LastKnownGoodMetricIDs,
	result LastKnownGoodResult[V],
	labelValues ...string,
) {
	descs := ids.Descs(ctx.Descriptors)
	ch <- prometheus.MustNewConstMetric(descs.LastSuccessTimestampDesc, prometheus.GaugeValue, lastKnownGoodUnixTimestamp(result.LastSuccess), labelValues...)
	ch <- prometheus.MustNewConstMetric(descs.ConsecutiveFailuresDesc, prometheus.GaugeValue, float64(result.ConsecutiveFailures), labelValues...)
	ch <- prometheus.MustNewConstMetric(descs.DataAvailableDesc, prometheus.GaugeValue, lastKnownGoodBoolFloat(result.Available), labelValues...)
	ch <- prometheus.MustNewConstMetric(descs.DataStaleDesc, prometheus.GaugeValue, lastKnownGoodBoolFloat(result.Stale), labelValues...)
}

func lastKnownGoodBoolFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func lastKnownGoodUnixTimestamp(value time.Time) float64 {
	if value.IsZero() {
		return 0
	}
	return float64(value.UnixNano()) / float64(time.Second)
}
