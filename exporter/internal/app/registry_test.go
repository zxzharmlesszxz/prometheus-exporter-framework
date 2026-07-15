package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	featurepkg "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/internal/feature"
)

func TestNewRegistryRegistersBaseAndFeatureCollectors(t *testing.T) {
	t.Parallel()

	feature := featurepkg.CollectorFeature{
		Name: "demo",
		CollectorsFunc: func(ctx FeatureContext) ([]prometheus.Collector, error) {
			if ctx.Namespace != "demo_exporter" {
				t.Fatalf("FeatureContext.Namespace = %q, want %q", ctx.Namespace, "demo_exporter")
			}
			if ctx.Logger == nil {
				t.Fatal("FeatureContext.Logger = nil, want logger")
			}
			return []prometheus.Collector{
				newConstCollector("demo_feature_value", "Demo feature value", 1),
			}, nil
		},
	}

	registry, err := NewRegistry("demo_exporter", slog.New(slog.NewTextHandler(io.Discard, nil)), feature)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v, want nil", err)
	}

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v, want nil", err)
	}

	if !hasMetricFamily(families, "demo_exporter_build_info") {
		t.Fatal("Gather() missing demo_exporter_build_info")
	}
	if !hasMetricFamily(families, "demo_feature_value") {
		t.Fatal("Gather() missing demo_feature_value")
	}
}

func TestNewRegistryContextPassesContextToFeatures(t *testing.T) {
	t.Parallel()

	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "registry-context")
	feature := featurepkg.CollectorFeature{
		Name: "demo",
		RegisterCollectorsFunc: func(featureContext FeatureContext, registry *prometheus.Registry) error {
			if featureContext.Context != ctx {
				t.Fatal("FeatureContext.Context did not match NewRegistryContext context")
			}
			return featurepkg.RegisterCollectors(registry, newConstCollector("demo_context_value", "Demo context value", 1))
		},
	}

	registry, err := NewRegistryContext(ctx, "demo_exporter", slog.New(slog.NewTextHandler(io.Discard, nil)), feature)
	if err != nil {
		t.Fatalf("NewRegistryContext() error = %v, want nil", err)
	}
	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v, want nil", err)
	}
	if !hasMetricFamily(families, "demo_context_value") {
		t.Fatal("Gather() missing demo_context_value")
	}
}

func TestRegisterCollectorsSkipsNilCollectors(t *testing.T) {
	t.Parallel()

	registry := prometheus.NewRegistry()
	if err := featurepkg.RegisterCollectors(registry, nil, newConstCollector("template_nil_skip_value", "Nil skip value", 1)); err != nil {
		t.Fatalf("RegisterCollectors() error = %v, want nil", err)
	}

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v, want nil", err)
	}
	if !hasMetricFamily(families, "template_nil_skip_value") {
		t.Fatal("Gather() missing template_nil_skip_value")
	}
}

func TestNewRegistryUsesDefaultNamespaceAndSkipsNilFeatures(t *testing.T) {
	t.Parallel()

	registry, err := NewRegistry("", nil, nil)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v, want nil", err)
	}

	families, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v, want nil", err)
	}
	if !hasMetricFamily(families, "exporter_framework_build_info") {
		t.Fatal("Gather() missing exporter_framework_build_info")
	}
}

func TestNewRegistryWrapsFeatureRegistrationError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("registration failed")
	feature := featurepkg.CollectorFeature{
		Name: "broken",
		RegisterCollectorsFunc: func(ctx FeatureContext, registry *prometheus.Registry) error {
			return wantErr
		},
	}

	_, err := NewRegistry("demo_exporter", nil, feature)
	if !errors.Is(err, wantErr) {
		t.Fatalf("NewRegistry() error = %v, want wrapped %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), `register feature "broken"`) {
		t.Fatalf("NewRegistry() error = %q, want feature name context", err.Error())
	}
}

func TestNewRegistryRejectsInvalidNamespace(t *testing.T) {
	t.Parallel()

	_, err := NewRegistry("demo-exporter", nil)
	if err == nil {
		t.Fatal("NewRegistry() error = nil, want invalid namespace error")
	}
	if !strings.Contains(err.Error(), `invalid metric namespace "demo-exporter"`) {
		t.Fatalf("NewRegistry() error = %q, want invalid namespace context", err.Error())
	}
}

func TestNewRegistryUsesFeatureTypeWhenNameIsEmpty(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("registration failed")
	feature := unnamedFailingFeature{err: wantErr}

	_, err := NewRegistry("demo_exporter", nil, feature)
	if !errors.Is(err, wantErr) {
		t.Fatalf("NewRegistry() error = %v, want wrapped %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), `register feature "app.unnamedFailingFeature"`) {
		t.Fatalf("NewRegistry() error = %q, want feature type context", err.Error())
	}
}

func TestFeatureNameReturnsEmptyForUnnamedFeature(t *testing.T) {
	t.Parallel()

	feature := unnamedFeature{}
	if got := featureName(feature); got != "" {
		t.Fatalf("featureName() = %q, want empty string", got)
	}
}

type unnamedFeature struct{}

func (unnamedFeature) RegisterFlags(app *kingpin.Application) {}

func (unnamedFeature) RegisterCollectors(ctx FeatureContext, registry *prometheus.Registry) error {
	return nil
}

type unnamedFailingFeature struct {
	err error
}

func (unnamedFailingFeature) RegisterFlags(app *kingpin.Application) {}

func (f unnamedFailingFeature) RegisterCollectors(ctx FeatureContext, registry *prometheus.Registry) error {
	return f.err
}

func hasMetricFamily(families []*dto.MetricFamily, name string) bool {
	for _, family := range families {
		if family.GetName() == name {
			return true
		}
	}
	return false
}
