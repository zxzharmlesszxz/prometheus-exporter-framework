package adaptertest

import (
	"testing"

	kingpin "github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
)

func TestRunInjectedAdapterContract(t *testing.T) {
	t.Parallel()

	metadata := framework.ProjectMetadata{
		ExporterName:         "prometheus-demo-exporter",
		ExporterDescription:  "Prometheus Demo Exporter",
		FeatureName:          "demo",
		MetricNamespace:      "demo_exporter",
		DefaultListenAddress: ":9888",
	}
	newFeature := func() framework.Feature {
		return adapterContractFeature{
			name: metadata.FeatureName,
			smoke: framework.SmokeSpec{
				ServerArgs:    []string{"--demo.target=example.net"},
				WantMetrics:   []string{"demo_exporter_target_up 1"},
				RejectMetrics: []string{"demo_exporter_target_up 0"},
			},
		}
	}
	main := MainFromInjectedProjectFunc(func(features ...framework.Feature) {
		t.Fatalf("unexpected main call with %d features", len(features))
	})

	RunInjectedAdapterContract(t, InjectedAdapterContractConfig{
		NewFeature: newFeature,
		Main: func() {
			main(newFeature())
		},
		ExporterInfo: func() framework.ExporterInfo {
			return framework.ExporterInfoFromProjectMetadata(metadata, newFeature())
		},
		ReplaceMainFromInjectedProject: func(fn MainFromInjectedProjectFunc) func() {
			oldMain := main
			main = fn
			return func() {
				main = oldMain
			}
		},
		Metadata: metadata,
	})
}

type adapterContractFeature struct {
	name  string
	smoke framework.SmokeSpec
}

func (f adapterContractFeature) FeatureName() string {
	return f.name
}

func (f adapterContractFeature) RegisterFlags(*kingpin.Application) {}

func (f adapterContractFeature) RegisterCollectors(framework.FeatureContext, *prometheus.Registry) error {
	return nil
}

func (f adapterContractFeature) SmokeSpec() framework.SmokeSpec {
	return f.smoke
}
