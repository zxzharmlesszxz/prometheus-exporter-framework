package featurekit

import "github.com/prometheus/client_golang/prometheus"

type FileScrapeMetricIDs struct {
	MTimeSeconds          string
	Up                    string
	Valid                 string
	ReadErrorsTotal       string
	ParseErrorsTotal      string
	ScrapeDurationSeconds string
}

type FileScrapeMetricDescs struct {
	MTimeDesc            *prometheus.Desc
	UpDesc               *prometheus.Desc
	ValidDesc            *prometheus.Desc
	ReadErrorsTotalDesc  *prometheus.Desc
	ParseErrorsTotalDesc *prometheus.Desc
	ScrapeDurationDesc   *prometheus.Desc
}

func FileScrapeMetricIDsFor(source string) FileScrapeMetricIDs {
	return FileScrapeMetricIDs{
		MTimeSeconds:          source + "_mtime_seconds",
		Up:                    source + "_up",
		Valid:                 source + "_valid",
		ReadErrorsTotal:       source + "_read_errors_total",
		ParseErrorsTotal:      source + "_parse_errors_total",
		ScrapeDurationSeconds: source + "_scrape_duration_seconds",
	}
}

func FileScrapeMetricSpecs(source string, labels []string) []FeatureMetricSpec {
	ids := FileScrapeMetricIDsFor(source)
	return []FeatureMetricSpec{
		{
			ID:     ids.MTimeSeconds,
			Scope:  MetricScopeFeature,
			Name:   "_" + source + "_mtime_seconds",
			Help:   "Unix timestamp of the " + source + " source file modification time.",
			Labels: labels,
		},
		{
			ID:     ids.Up,
			Scope:  MetricScopeFeature,
			Name:   "_" + source + "_up",
			Help:   "Whether the " + source + " source was readable during the last collection.",
			Labels: labels,
		},
		{
			ID:     ids.Valid,
			Scope:  MetricScopeFeature,
			Name:   "_" + source + "_valid",
			Help:   "Whether the " + source + " source produced valid data during the last collection.",
			Labels: labels,
		},
		{
			ID:     ids.ReadErrorsTotal,
			Scope:  MetricScopeFeature,
			Name:   "_" + source + "_read_errors_total",
			Help:   "Total number of " + source + " source read errors.",
			Labels: labels,
		},
		{
			ID:     ids.ParseErrorsTotal,
			Scope:  MetricScopeFeature,
			Name:   "_" + source + "_parse_errors_total",
			Help:   "Total number of " + source + " source parse errors.",
			Labels: labels,
		},
		{
			ID:     ids.ScrapeDurationSeconds,
			Scope:  MetricScopeFeature,
			Name:   "_" + source + "_scrape_duration_seconds",
			Help:   "Duration in seconds of the last " + source + " source scrape.",
			Labels: labels,
		},
	}
}

func (ids FileScrapeMetricIDs) Descs(descs FeatureMetricDescriptors) FileScrapeMetricDescs {
	return FileScrapeMetricDescs{
		MTimeDesc:            descs.Get(ids.MTimeSeconds),
		UpDesc:               descs.Get(ids.Up),
		ValidDesc:            descs.Get(ids.Valid),
		ReadErrorsTotalDesc:  descs.Get(ids.ReadErrorsTotal),
		ParseErrorsTotalDesc: descs.Get(ids.ParseErrorsTotal),
		ScrapeDurationDesc:   descs.Get(ids.ScrapeDurationSeconds),
	}
}
