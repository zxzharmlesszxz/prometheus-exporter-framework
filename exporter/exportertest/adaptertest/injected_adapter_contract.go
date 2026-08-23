package adaptertest

import (
	"testing"

	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
)

type MainFromInjectedProjectFunc = func(features ...framework.Feature)

type InjectedAdapterContractConfig struct {
	NewFeature                     func() framework.Feature
	Main                           func()
	ExporterInfo                   func() framework.ExporterInfo
	ExporterInfoErr                func() (framework.ExporterInfo, error)
	ReplaceMainFromInjectedProject func(MainFromInjectedProjectFunc) func()
	Metadata                       framework.ProjectMetadata
}

func RunInjectedAdapterContract(t *testing.T, config InjectedAdapterContractConfig) {
	t.Helper()
	requireInjectedAdapterContractConfig(t, config)

	metadata := config.Metadata
	if metadata == (framework.ProjectMetadata{}) {
		var err error
		metadata, err = framework.InjectedProjectMetadataErr()
		if err != nil {
			t.Skipf("skipping injected adapter contract without injected metadata: %v", err)
		}
	}

	t.Run("creates feature with injected feature name", func(t *testing.T) {
		feature := config.NewFeature()
		named, ok := feature.(framework.NamedFeature)
		if !ok {
			t.Fatalf("NewFeature() = %T, want framework.NamedFeature", feature)
		}
		if got := named.FeatureName(); got != metadata.FeatureName {
			t.Fatalf("FeatureName() = %q, want %q", got, metadata.FeatureName)
		}
	})

	t.Run("main delegates to framework", func(t *testing.T) {
		called := false
		restore := config.ReplaceMainFromInjectedProject(func(features ...framework.Feature) {
			called = true
			if len(features) != 1 {
				t.Fatalf("features length = %d, want 1", len(features))
			}
			named, ok := features[0].(framework.NamedFeature)
			if !ok {
				t.Fatalf("feature = %T, want framework.NamedFeature", features[0])
			}
			if got := named.FeatureName(); got != metadata.FeatureName {
				t.Fatalf("FeatureName() = %q, want %q", got, metadata.FeatureName)
			}
		})
		if restore == nil {
			t.Fatal("ReplaceMainFromInjectedProject returned nil restore function")
		}
		t.Cleanup(restore)

		config.Main()
		if !called {
			t.Fatal("framework main was not called")
		}
	})

	t.Run("reports injected exporter info", func(t *testing.T) {
		info := exporterInfo(t, config)
		if info.Name != metadata.ExporterName {
			t.Fatalf("Name = %q, want %q", info.Name, metadata.ExporterName)
		}
		if info.Description != metadata.ExporterDescription {
			t.Fatalf("Description = %q, want %q", info.Description, metadata.ExporterDescription)
		}
		if info.FeatureName != metadata.FeatureName {
			t.Fatalf("FeatureName = %q, want %q", info.FeatureName, metadata.FeatureName)
		}
		if info.MetricNamespace != metadata.MetricNamespace {
			t.Fatalf("MetricNamespace = %q, want %q", info.MetricNamespace, metadata.MetricNamespace)
		}
		if info.DefaultListenAddress != metadata.DefaultListenAddress {
			t.Fatalf("DefaultListenAddress = %q, want %q", info.DefaultListenAddress, metadata.DefaultListenAddress)
		}

		metrics := framework.StandardMetricInfo(metadata.MetricNamespace)
		if info.Metrics != metrics {
			t.Fatalf("Metrics = %#v, want %#v", info.Metrics, metrics)
		}
		assertHasString(t, info.Smoke.ForbiddenUsageNames, metadata.MetricNamespace, "Smoke.ForbiddenUsageNames")
		assertHasString(t, info.Smoke.WantMetrics, metrics.LastCollectionSuccess+" 1", "Smoke.WantMetrics")
		assertHasString(t, info.Smoke.RejectMetrics, metrics.LastCollectionSuccess+" 0", "Smoke.RejectMetrics")

		if provider, ok := config.NewFeature().(framework.SmokeSpecProvider); ok {
			spec := provider.SmokeSpec()
			for _, want := range spec.ServerArgs {
				assertHasString(t, info.Smoke.ServerArgs, want, "Smoke.ServerArgs")
			}
			for _, want := range spec.WantMetrics {
				assertHasString(t, info.Smoke.WantMetrics, want, "Smoke.WantMetrics")
			}
			for _, want := range spec.RejectMetrics {
				assertHasString(t, info.Smoke.RejectMetrics, want, "Smoke.RejectMetrics")
			}
		}
	})
}

func requireInjectedAdapterContractConfig(t *testing.T, config InjectedAdapterContractConfig) {
	t.Helper()

	if config.NewFeature == nil {
		t.Fatal("InjectedAdapterContractConfig.NewFeature is required")
	}
	if config.Main == nil {
		t.Fatal("InjectedAdapterContractConfig.Main is required")
	}
	if config.ExporterInfo == nil && config.ExporterInfoErr == nil {
		t.Fatal("InjectedAdapterContractConfig.ExporterInfo or ExporterInfoErr is required")
	}
	if config.ReplaceMainFromInjectedProject == nil {
		t.Fatal("InjectedAdapterContractConfig.ReplaceMainFromInjectedProject is required")
	}
}

func exporterInfo(t *testing.T, config InjectedAdapterContractConfig) framework.ExporterInfo {
	t.Helper()

	if config.ExporterInfoErr != nil {
		info, err := config.ExporterInfoErr()
		if err != nil {
			t.Skipf("skipping injected exporter info contract without injected metadata: %v", err)
		}
		return info
	}
	return config.ExporterInfo()
}

func assertHasString(t *testing.T, values []string, want string, field string) {
	t.Helper()

	for _, value := range values {
		if value == want {
			return
		}
	}
	t.Fatalf("%s = %v, want %q", field, values, want)
}
