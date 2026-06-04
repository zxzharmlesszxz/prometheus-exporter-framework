package exporter

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/internal/feature"
)

type Feature = feature.Feature
type NamedFeature = feature.NamedFeature
type RuntimeConfigReporter = feature.RuntimeConfigReporter
type SmokeSpecProvider = feature.SmokeSpecProvider
type SmokeSpec = feature.SmokeSpec
type DefaultListenAddressProvider = feature.DefaultListenAddressProvider
type StartableCollector = feature.StartableCollector
type FeatureContext = feature.FeatureContext
type CollectorFeature = feature.CollectorFeature

func RegisterCollectors(registry *prometheus.Registry, collectors ...prometheus.Collector) error {
	return feature.RegisterCollectors(registry, collectors...)
}

func RegisterAndStartCollectors(ctx context.Context, registry *prometheus.Registry, collectors ...StartableCollector) error {
	return feature.RegisterAndStartCollectors(ctx, registry, collectors...)
}
