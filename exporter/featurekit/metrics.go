package featurekit

import "github.com/prometheus/client_golang/prometheus"

type MetricScope int

const (
	MetricScopeFeature MetricScope = iota
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
	return id
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
