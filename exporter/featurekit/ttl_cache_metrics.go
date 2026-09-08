package featurekit

import "github.com/prometheus/client_golang/prometheus"

const (
	TTLCacheMetricEntries = "cache_entries"
	TTLCacheMetricHits    = "cache_hits_total"
	TTLCacheMetricMisses  = "cache_misses_total"
	TTLCacheMetricSets    = "cache_sets_total"
	TTLCacheMetricDeletes = "cache_deletes_total"
	TTLCacheMetricExpired = "cache_expired_total"
	TTLCacheMetricClears  = "cache_clears_total"
)

type TTLCacheMetricDescs struct {
	EntriesDesc *prometheus.Desc
	HitsDesc    *prometheus.Desc
	MissesDesc  *prometheus.Desc
	SetsDesc    *prometheus.Desc
	DeletesDesc *prometheus.Desc
	ExpiredDesc *prometheus.Desc
	ClearsDesc  *prometheus.Desc
}

func TTLCacheMetricSpecs(labels []string) []FeatureMetricSpec {
	cacheLabels := append([]string{"cache"}, labels...)
	return []FeatureMetricSpec{
		{
			ID:     TTLCacheMetricEntries,
			Scope:  MetricScopeFeature,
			Name:   "_cache_entries",
			Help:   "Current number of live entries in the feature cache.",
			Labels: cacheLabels,
		},
		{
			ID:     TTLCacheMetricHits,
			Scope:  MetricScopeFeature,
			Name:   "_cache_hits_total",
			Help:   "Cumulative total number of feature cache hits.",
			Labels: cacheLabels,
		},
		{
			ID:     TTLCacheMetricMisses,
			Scope:  MetricScopeFeature,
			Name:   "_cache_misses_total",
			Help:   "Cumulative total number of feature cache misses.",
			Labels: cacheLabels,
		},
		{
			ID:     TTLCacheMetricSets,
			Scope:  MetricScopeFeature,
			Name:   "_cache_sets_total",
			Help:   "Cumulative total number of feature cache set operations.",
			Labels: cacheLabels,
		},
		{
			ID:     TTLCacheMetricDeletes,
			Scope:  MetricScopeFeature,
			Name:   "_cache_deletes_total",
			Help:   "Cumulative total number of feature cache entries deleted explicitly.",
			Labels: cacheLabels,
		},
		{
			ID:     TTLCacheMetricExpired,
			Scope:  MetricScopeFeature,
			Name:   "_cache_expired_total",
			Help:   "Cumulative total number of feature cache entries removed after expiry.",
			Labels: cacheLabels,
		},
		{
			ID:     TTLCacheMetricClears,
			Scope:  MetricScopeFeature,
			Name:   "_cache_clears_total",
			Help:   "Cumulative total number of feature cache clear operations.",
			Labels: cacheLabels,
		},
	}
}

func TTLCacheMetricDescsFor(descs FeatureMetricDescriptors) TTLCacheMetricDescs {
	return TTLCacheMetricDescs{
		EntriesDesc: descs.Get(TTLCacheMetricEntries),
		HitsDesc:    descs.Get(TTLCacheMetricHits),
		MissesDesc:  descs.Get(TTLCacheMetricMisses),
		SetsDesc:    descs.Get(TTLCacheMetricSets),
		DeletesDesc: descs.Get(TTLCacheMetricDeletes),
		ExpiredDesc: descs.Get(TTLCacheMetricExpired),
		ClearsDesc:  descs.Get(TTLCacheMetricClears),
	}
}

func CollectTTLCacheMetrics[S any](ctx FeatureMetricsContext[S], ch chan<- prometheus.Metric, cache string, stats TTLCacheStats, labelValues ...string) {
	descs := TTLCacheMetricDescsFor(ctx.Descriptors)
	values := append([]string{cache}, labelValues...)
	ch <- prometheus.MustNewConstMetric(descs.EntriesDesc, prometheus.GaugeValue, float64(stats.Entries), values...)
	ch <- prometheus.MustNewConstMetric(descs.HitsDesc, prometheus.CounterValue, float64(stats.Hits), values...)
	ch <- prometheus.MustNewConstMetric(descs.MissesDesc, prometheus.CounterValue, float64(stats.Misses), values...)
	ch <- prometheus.MustNewConstMetric(descs.SetsDesc, prometheus.CounterValue, float64(stats.Sets), values...)
	ch <- prometheus.MustNewConstMetric(descs.DeletesDesc, prometheus.CounterValue, float64(stats.Deletes), values...)
	ch <- prometheus.MustNewConstMetric(descs.ExpiredDesc, prometheus.CounterValue, float64(stats.Expired), values...)
	ch <- prometheus.MustNewConstMetric(descs.ClearsDesc, prometheus.CounterValue, float64(stats.Clears), values...)
}
