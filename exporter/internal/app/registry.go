package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	versioncollector "github.com/prometheus/client_golang/prometheus/collectors/version"
)

func NewRegistry(namespace string, logger *slog.Logger, features ...Feature) (*prometheus.Registry, error) {
	return NewRegistryContext(context.Background(), namespace, logger, features...)
}

func NewRegistryContext(ctx context.Context, namespace string, logger *slog.Logger, features ...Feature) (*prometheus.Registry, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if namespace == "" {
		namespace = defaultExporterName
	}
	if err := validateMetricNamespace(namespace); err != nil {
		return nil, fmt.Errorf("invalid metric namespace %q: %w", namespace, err)
	}
	if logger == nil {
		logger = slog.Default()
	}

	registry := prometheus.NewPedanticRegistry()
	registry.MustRegister(versioncollector.NewCollector(namespace))
	registry.MustRegister(
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		collectors.NewGoCollector(),
	)

	for _, feature := range features {
		if feature == nil {
			continue
		}

		featureLogger := logger
		if name := featureName(feature); name != "" {
			featureLogger = featureLogger.With("feature", name)
		}

		featureContext := FeatureContext{
			Context:      ctx,
			Logger:       featureLogger,
			ExporterName: namespace,
			Namespace:    namespace,
		}
		if err := feature.RegisterCollectors(featureContext, registry); err != nil {
			return nil, fmt.Errorf("register feature %q: %w", featureLogName(feature), err)
		}
	}

	return registry, nil
}

func featureName(feature Feature) string {
	named, ok := feature.(NamedFeature)
	if !ok {
		return ""
	}
	return named.FeatureName()
}

func featureLogName(feature Feature) string {
	if name := featureName(feature); name != "" {
		return name
	}
	return fmt.Sprintf("%T", feature)
}
