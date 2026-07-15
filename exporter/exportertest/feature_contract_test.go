package exportertest

import (
	"testing"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
)

type contractContext struct {
	namespace string
}

type contractFeature struct {
	target *string
}

func (f *contractFeature) RegisterFlags(app *kingpin.Application) {
	f.target = app.Flag("demo.target", "Demo target.").Default("default").String()
}

func (f *contractFeature) RuntimeConfig() []any {
	target := "default"
	if f.target != nil {
		target = *f.target
	}
	return []any{"target", target}
}

func (f *contractFeature) RegisterCollectors(ctx contractContext, registry *prometheus.Registry) error {
	metricName := ctx.namespace + "_last_collection_success"
	return registry.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: metricName,
			Help: "Whether the last contract collection succeeded.",
		},
		func() float64 {
			return 1
		},
	))
}

func TestRunFeatureContract(t *testing.T) {
	RunFeatureContract(t, FeatureContractConfig{
		NewFeature: func() FeatureContractFeature {
			return &contractFeature{}
		},
		FeatureContext:              contractContext{namespace: "contract"},
		FlagArgs:                    []string{"--demo.target=node-a"},
		WantRuntimeConfig:           map[string]any{"target": "node-a"},
		RegisterCollectors:          true,
		DuplicateRegistration:       true,
		LastCollectionSuccessMetric: "contract_last_collection_success",
	})
}

func TestRunFeatureContractSkipsOptionalChecks(t *testing.T) {
	RunFeatureContract(t, FeatureContractConfig{
		NewFeature: func() FeatureContractFeature {
			return &contractFeature{}
		},
	})
}

func TestRunFeatureContractRejectsNilFeatureFactoryResult(t *testing.T) {
	t.Parallel()

	expectFatal(t, func(tb TB) {
		_ = newFeatureContractFeature(tb, func() FeatureContractFeature {
			return nil
		})
	}, "FeatureContractConfig.NewFeature returned nil")
	expectFatal(t, func(tb TB) {
		_ = newFeatureContractFeature(tb, func() FeatureContractFeature {
			var feature *contractFeature
			return feature
		})
	}, "FeatureContractConfig.NewFeature returned nil")
}

func TestRegisterCollectorsReportsContractShapeErrors(t *testing.T) {
	t.Parallel()

	registry := prometheus.NewRegistry()
	expectFatal(t, func(tb TB) {
		_ = registerCollectors(tb, missingRegisterCollectorsFeature{}, contractContext{}, registry)
	}, "does not define RegisterCollectors")
	expectFatal(t, func(tb TB) {
		_ = registerCollectors(tb, badContextRegisterCollectorsFeature{}, contractContext{}, registry)
	}, "context argument")
	expectFatal(t, func(tb TB) {
		_ = registerCollectors(tb, badReturnRegisterCollectorsFeature{}, contractContext{}, registry)
	}, "must return error")
}

type missingRegisterCollectorsFeature struct{}

type badContextRegisterCollectorsFeature struct{}

func (badContextRegisterCollectorsFeature) RegisterCollectors(string, *prometheus.Registry) error {
	return nil
}

type badReturnRegisterCollectorsFeature struct{}

func (badReturnRegisterCollectorsFeature) RegisterCollectors(contractContext, *prometheus.Registry) string {
	return ""
}
