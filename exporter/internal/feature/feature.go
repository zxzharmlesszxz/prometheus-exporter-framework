package feature

import (
	"context"
	"log/slog"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
)

// Feature is the stable extension point for concrete exporters.
//
// A feature owns its domain flags and collector registration. Concrete exporter
// repositories add features in their own code and pass them to Main or RunCLI.
type Feature interface {
	RegisterFlags(app *kingpin.Application)
	RegisterCollectors(ctx FeatureContext, registry *prometheus.Registry) error
}

type NamedFeature interface {
	FeatureName() string
}

type RuntimeConfigReporter interface {
	RuntimeConfig() []any
}

type SmokeSpecProvider interface {
	SmokeSpec() SmokeSpec
}

type SmokeSpec struct {
	// ServerArgs are extra binary arguments for smoke tests. Features that need
	// faster or safer smoke behavior should pass test-specific flag values here,
	// for example a short refresh interval or a local config file path.
	ServerArgs []string
	// WantMetrics are substrings that must be present in the scraped metrics
	// response during binary smoke tests.
	WantMetrics []string
	// RejectMetrics are substrings that must be absent from the scraped metrics
	// response during binary smoke tests.
	RejectMetrics []string
}

type DefaultListenAddressProvider interface {
	DefaultListenAddress() string
}

type StartableCollector interface {
	prometheus.Collector
	Start(context.Context)
}

type FeatureContext struct {
	Context      context.Context
	Logger       *slog.Logger
	ExporterName string
	Namespace    string
}

type CollectorFeature struct {
	Name                      string
	DefaultListenAddressValue string
	RegisterFlagsFunc         func(app *kingpin.Application)
	CollectorsFunc            func(ctx FeatureContext) ([]prometheus.Collector, error)
	RuntimeConfigFunc         func() []any
	RegisterCollectorsFunc    func(ctx FeatureContext, registry *prometheus.Registry) error
}

func (f CollectorFeature) FeatureName() string {
	return f.Name
}

func (f CollectorFeature) DefaultListenAddress() string {
	return f.DefaultListenAddressValue
}

func (f CollectorFeature) RegisterFlags(app *kingpin.Application) {
	if f.RegisterFlagsFunc != nil {
		f.RegisterFlagsFunc(app)
	}
}

func (f CollectorFeature) RegisterCollectors(ctx FeatureContext, registry *prometheus.Registry) error {
	if f.RegisterCollectorsFunc != nil {
		return f.RegisterCollectorsFunc(ctx, registry)
	}
	if f.CollectorsFunc == nil {
		return nil
	}
	collectors, err := f.CollectorsFunc(ctx)
	if err != nil {
		return err
	}
	return RegisterCollectors(registry, collectors...)
}

func (f CollectorFeature) RuntimeConfig() []any {
	if f.RuntimeConfigFunc == nil {
		return nil
	}
	return f.RuntimeConfigFunc()
}

func RegisterCollectors(registry *prometheus.Registry, collectors ...prometheus.Collector) error {
	registered := make([]prometheus.Collector, 0, len(collectors))
	for _, collector := range collectors {
		if collector == nil {
			continue
		}
		if err := registry.Register(collector); err != nil {
			for _, registeredCollector := range registered {
				registry.Unregister(registeredCollector)
			}
			return err
		}
		registered = append(registered, collector)
	}
	return nil
}

func RegisterAndStartCollectors(ctx context.Context, registry *prometheus.Registry, collectors ...StartableCollector) error {
	registered := make([]StartableCollector, 0, len(collectors))
	for _, collector := range collectors {
		if collector == nil {
			continue
		}
		if err := registry.Register(collector); err != nil {
			for _, registeredCollector := range registered {
				registry.Unregister(registeredCollector)
			}
			return err
		}
		registered = append(registered, collector)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	for _, collector := range registered {
		collector.Start(ctx)
	}
	return nil
}
