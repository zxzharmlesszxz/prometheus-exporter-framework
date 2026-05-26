package exportertest

import (
	"reflect"
	"testing"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
)

type FeatureContractFeature interface {
	RegisterFlags(app *kingpin.Application)
	RuntimeConfig() []any
}

type FeatureContractConfig struct {
	NewFeature                  func() FeatureContractFeature
	FeatureContext              any
	FlagArgs                    []string
	WantRuntimeConfig           map[string]any
	RegisterCollectors          bool
	DuplicateRegistration       bool
	LastCollectionSuccessMetric string
}

func RunFeatureContract(t *testing.T, config FeatureContractConfig) {
	t.Helper()
	if config.NewFeature == nil {
		t.Fatal("FeatureContractConfig.NewFeature is required")
	}

	if len(config.FlagArgs) > 0 || len(config.WantRuntimeConfig) > 0 {
		t.Run("registers flags and reports runtime config", func(t *testing.T) {
			feature := config.NewFeature()
			ParseFeatureFlags(t, feature, config.FlagArgs)
			AssertRuntimeConfigValues(t, feature.RuntimeConfig(), config.WantRuntimeConfig)
		})
	}

	if config.RegisterCollectors || config.LastCollectionSuccessMetric != "" {
		t.Run("registers collectors", func(t *testing.T) {
			feature := config.NewFeature()
			registry := prometheus.NewRegistry()
			if err := registerCollectors(t, feature, config.FeatureContext, registry); err != nil {
				t.Fatalf("RegisterCollectors() error = %v", err)
			}
			if config.LastCollectionSuccessMetric != "" {
				WaitForMetricValue(t, registry, config.LastCollectionSuccessMetric, nil, 1)
			}
		})
	}

	if config.DuplicateRegistration {
		t.Run("reports duplicate collector registration", func(t *testing.T) {
			feature := config.NewFeature()
			registry := prometheus.NewRegistry()
			if err := registerCollectors(t, feature, config.FeatureContext, registry); err != nil {
				t.Fatalf("RegisterCollectors() error = %v", err)
			}
			if err := registerCollectors(t, feature, config.FeatureContext, registry); err == nil {
				t.Fatal("RegisterCollectors() error = nil, want duplicate registration error")
			}
		})
	}
}

func ParseFeatureFlags(t *testing.T, feature interface {
	RegisterFlags(app *kingpin.Application)
}, args []string) {
	t.Helper()

	app := kingpin.New("test", "")
	app.Terminate(func(int) {})
	feature.RegisterFlags(app)
	if _, err := app.Parse(args); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
}

func registerCollectors(t *testing.T, feature any, ctx any, registry *prometheus.Registry) error {
	t.Helper()

	method := reflect.ValueOf(feature).MethodByName("RegisterCollectors")
	if !method.IsValid() {
		t.Fatalf("%T does not define RegisterCollectors", feature)
	}
	values := method.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(registry),
	})
	if len(values) != 1 {
		t.Fatalf("%T.RegisterCollectors returned %d values, want 1", feature, len(values))
	}
	if values[0].IsNil() {
		return nil
	}
	err, ok := values[0].Interface().(error)
	if !ok {
		t.Fatalf("%T.RegisterCollectors returned %T, want error", feature, values[0].Interface())
	}
	return err
}

func AssertRuntimeConfigValues(t *testing.T, config []any, want map[string]any) {
	t.Helper()

	for key, wantValue := range want {
		got := RuntimeConfigValue(t, config, key)
		if !reflect.DeepEqual(got, wantValue) {
			t.Fatalf("%s = %#v, want %#v", key, got, wantValue)
		}
	}
}
