package featurekit

import (
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type MetricScope int

const (
	MetricScopeUnset MetricScope = iota
	MetricScopeFeature
	MetricScopeNamespace
	MetricScopeAbsolute
)

type FeatureMetricSpec struct {
	ID     string
	Scope  MetricScope
	Name   string
	Help   string
	Labels []string
}

type FeatureMetricDescriptors struct {
	order []string
	descs map[string]*prometheus.Desc
}

type FeatureMetricsContext[S any] struct {
	SnapshotMetricsContext[S]
	Descriptors FeatureMetricDescriptors
}

type FeatureMetricsCollectFunc[S any] func(ctx FeatureMetricsContext[S], ch chan<- prometheus.Metric, snapshot S, now time.Time)

type FeatureMetricsLogFunc[S any] func(ctx FeatureMetricsContext[S], logger *slog.Logger, snapshot S)

type FeatureMetricHandlers[S any] struct {
	Collect  FeatureMetricsCollectFunc[S]
	LogError FeatureMetricsLogFunc[S]
}

func NewFeatureMetrics[S any](ctx SnapshotMetricsContext[S], specs []FeatureMetricSpec, handlers FeatureMetricHandlers[S]) SnapshotMetrics[S] {
	return featureMetrics[S]{
		ctx: FeatureMetricsContext[S]{
			SnapshotMetricsContext: ctx,
			Descriptors:            LoadFeatureMetricDescriptors(ctx.FeatureName, ctx.Namespace, specs),
		},
		handlers: handlers,
	}
}

func LoadFeatureMetricDescriptors(featureName string, namespace string, specs []FeatureMetricSpec) FeatureMetricDescriptors {
	metrics := FeatureMetricDescriptors{
		order: make([]string, 0, len(specs)),
		descs: make(map[string]*prometheus.Desc, len(specs)),
	}
	for _, spec := range specs {
		metrics.order = append(metrics.order, spec.ID)
		metrics.descs[spec.ID] = prometheus.NewDesc(
			spec.MetricName(featureName, namespace),
			spec.Help,
			spec.Labels,
			nil,
		)
	}
	return metrics
}

func (d FeatureMetricDescriptors) Describe(ch chan<- *prometheus.Desc) {
	for _, id := range d.order {
		ch <- d.Get(id)
	}
}

func (d FeatureMetricDescriptors) Get(id string) *prometheus.Desc {
	return d.descs[id]
}

func FeatureMetricName(featureName string, namespace string, id string, specs []FeatureMetricSpec) string {
	for _, spec := range specs {
		if spec.ID == id {
			return spec.MetricName(featureName, namespace)
		}
	}
	panic("unknown metric ID: " + id)
}

func (s FeatureMetricSpec) MetricName(featureName string, namespace string) string {
	switch s.Scope {
	case MetricScopeFeature:
		return featureName + s.Name
	case MetricScopeNamespace:
		return namespace + s.Name
	case MetricScopeAbsolute:
		return s.Name
	default:
		return s.Name
	}
}

type featureMetrics[S any] struct {
	ctx      FeatureMetricsContext[S]
	handlers FeatureMetricHandlers[S]
}

func (m featureMetrics[S]) Describe(ch chan<- *prometheus.Desc) {
	m.ctx.Descriptors.Describe(ch)
}

func (m featureMetrics[S]) Collect(ch chan<- prometheus.Metric, snapshot S, now time.Time) {
	if m.handlers.Collect != nil {
		m.handlers.Collect(m.ctx, ch, snapshot, now)
	}
}

func (m featureMetrics[S]) LogSnapshotError(logger *slog.Logger, snapshot S) {
	if m.handlers.LogError != nil {
		m.handlers.LogError(m.ctx, logger, snapshot)
	}
}
