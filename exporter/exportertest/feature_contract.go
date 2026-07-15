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
			feature := newFeatureContractFeature(t, config.NewFeature)
			ParseFeatureFlags(t, feature, config.FlagArgs)
			AssertRuntimeConfigValues(t, feature.RuntimeConfig(), config.WantRuntimeConfig)
		})
	}

	if config.RegisterCollectors || config.LastCollectionSuccessMetric != "" {
		t.Run("registers collectors", func(t *testing.T) {
			feature := newFeatureContractFeature(t, config.NewFeature)
			ParseFeatureFlags(t, feature, config.FlagArgs)
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
			feature := newFeatureContractFeature(t, config.NewFeature)
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

func newFeatureContractFeature(tb TB, newFeature func() FeatureContractFeature) FeatureContractFeature {
	tb.Helper()

	feature := newFeature()
	if isNil(feature) {
		tb.Fatalf("FeatureContractConfig.NewFeature returned nil")
	}
	return feature
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

var errorType = reflect.TypeOf((*error)(nil)).Elem()

func registerCollectors(tb TB, feature any, ctx any, registry *prometheus.Registry) error {
	tb.Helper()

	method := reflect.ValueOf(feature).MethodByName("RegisterCollectors")
	if !method.IsValid() {
		tb.Fatalf("%T does not define RegisterCollectors", feature)
	}
	methodType := method.Type()
	if methodType.NumIn() != 2 {
		tb.Fatalf("%T.RegisterCollectors accepts %d arguments, want 2", feature, methodType.NumIn())
	}
	if methodType.NumOut() != 1 || !methodType.Out(0).Implements(errorType) {
		tb.Fatalf("%T.RegisterCollectors must return error", feature)
	}
	ctxValue := reflect.ValueOf(ctx)
	if !ctxValue.IsValid() || !ctxValue.Type().AssignableTo(methodType.In(0)) {
		tb.Fatalf("%T.RegisterCollectors context argument is %v, want %v", feature, typeName(ctxValue), methodType.In(0))
	}
	registryValue := reflect.ValueOf(registry)
	if !registryValue.IsValid() || !registryValue.Type().AssignableTo(methodType.In(1)) {
		tb.Fatalf("%T.RegisterCollectors registry argument is %v, want %v", feature, typeName(registryValue), methodType.In(1))
	}

	values := method.Call([]reflect.Value{
		ctxValue,
		registryValue,
	})
	if values[0].IsNil() {
		return nil
	}
	err, ok := values[0].Interface().(error)
	if !ok {
		tb.Fatalf("%T.RegisterCollectors returned %T, want error", feature, values[0].Interface())
	}
	return err
}

func typeName(value reflect.Value) any {
	if !value.IsValid() {
		return "<nil>"
	}
	return value.Type()
}

func isNil(value any) bool {
	if value == nil {
		return true
	}
	reflectValue := reflect.ValueOf(value)
	switch reflectValue.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return reflectValue.IsNil()
	default:
		return false
	}
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
