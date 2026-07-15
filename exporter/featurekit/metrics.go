package featurekit

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type MetricScope int

const (
	MetricScopeUnset MetricScope = iota
	// MetricScopeFeature prefixes Name with the framework feature name.
	MetricScopeFeature
	// MetricScopeNamespace prefixes Name with the exporter/framework metric namespace.
	MetricScopeNamespace
	// MetricScopeAbsolute uses Name as the complete Prometheus metric name.
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
	validateFeatureMetricSpecs(specs)
	seenNames := make(map[string]string, len(specs))
	metrics := FeatureMetricDescriptors{
		order: make([]string, 0, len(specs)),
		descs: make(map[string]*prometheus.Desc, len(specs)),
	}
	for _, spec := range specs {
		metricName := spec.MetricName(featureName, namespace)
		if metricName == "" {
			panic(fmt.Sprintf("metric spec ID %q renders empty metric name", spec.ID))
		}
		if previousID, ok := seenNames[metricName]; ok {
			panic(fmt.Sprintf("duplicate metric name %q for IDs %q and %q", metricName, previousID, spec.ID))
		}
		seenNames[metricName] = spec.ID
		metrics.order = append(metrics.order, spec.ID)
		metrics.descs[spec.ID] = prometheus.NewDesc(
			metricName,
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
	desc := d.descs[id]
	if desc == nil {
		panic("unknown metric descriptor ID: " + id)
	}
	return desc
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

func validateFeatureMetricSpecs(specs []FeatureMetricSpec) {
	seen := make(map[string]struct{}, len(specs))
	for index, spec := range specs {
		if spec.ID == "" {
			panic(fmt.Sprintf("metric spec at index %d has empty ID", index))
		}
		if _, ok := seen[spec.ID]; ok {
			panic("duplicate metric spec ID: " + spec.ID)
		}
		seen[spec.ID] = struct{}{}
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
