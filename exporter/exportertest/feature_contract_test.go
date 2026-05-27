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
