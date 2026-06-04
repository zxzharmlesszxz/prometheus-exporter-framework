package exporter

import (
	"time"

	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/internal/metric"
)

func BoolFloat(value bool) float64 {
	return metric.BoolFloat(value)
}

func UnixTimestamp(value time.Time) float64 {
	return metric.UnixTimestamp(value)
}

func NormalizeDuration(value time.Duration, fallback time.Duration) time.Duration {
	return metric.NormalizeDuration(value, fallback)
}
