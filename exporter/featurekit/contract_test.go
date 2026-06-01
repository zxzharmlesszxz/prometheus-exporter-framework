package featurekit

import (
	"reflect"
	"testing"
	"time"

	"github.com/alecthomas/kingpin/v2"
	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
)

type contractTestConfig struct {
	Name string
}

type contractTestSnapshot struct {
	AttemptTime time.Time
	Success     bool
}

func TestFeatureDefaultsAreNoops(t *testing.T) {
	t.Parallel()

	defaults := FeatureDefaults[contractTestConfig, contractTestSnapshot]{}
	if got := defaults.DefaultRefreshInterval(); got != 0 {
		t.Fatalf("DefaultRefreshInterval() = %v, want 0", got)
	}
	if got := defaults.DefaultConfig(); !reflect.DeepEqual(got, contractTestConfig{}) {
		t.Fatalf("DefaultConfig() = %+v, want zero config", got)
	}

	app := kingpin.New("test", "")
	defaults.RegisterFlags(app, FlagContext{}, &contractTestConfig{})
	if _, err := app.Parse(nil); err != nil {
		t.Fatalf("Parse() after RegisterFlags() error = %v", err)
	}
	if err := defaults.ValidateConfig(contractTestConfig{}); err != nil {
		t.Fatalf("ValidateConfig() error = %v", err)
	}

	snapshotter, err := defaults.NewSnapshotter(CollectorContext[contractTestConfig]{
		Framework: framework.FeatureContext{},
	})
	if err != nil {
		t.Fatalf("NewSnapshotter() error = %v", err)
	}
	if snapshotter != nil {
		t.Fatalf("NewSnapshotter() = %T, want nil", snapshotter)
	}
	if got := defaults.DefaultSnapshotter(); got != nil {
		t.Fatalf("DefaultSnapshotter() = %T, want nil", got)
	}
	if got := defaults.NewMetrics(SnapshotMetricsContext[contractTestSnapshot]{}); got != nil {
		t.Fatalf("NewMetrics() = %T, want nil", got)
	}
	if got := defaults.SnapshotStatus(contractTestSnapshot{}); got != (framework.SnapshotStatus{}) {
		t.Fatalf("SnapshotStatus() = %+v, want zero status", got)
	}
	if got := defaults.RuntimeConfig(RuntimeConfigContext[contractTestConfig]{}); got != nil {
		t.Fatalf("RuntimeConfig() = %+v, want nil", got)
	}
	if got := defaults.SmokeSpec(SmokeContext[contractTestConfig]{}); !emptySmokeSpec(got) {
		t.Fatalf("SmokeSpec() = %+v, want empty smoke spec", got)
	}
}

func TestNewContractSnapshotFeatureSpecDelegatesToContract(t *testing.T) {
	t.Parallel()

	contract := contractTestFeature{}
	spec := NewContractSnapshotFeatureSpec[contractTestConfig, contractTestSnapshot](SpecOptions{
		FeatureName:            "contract",
		DefaultRefreshInterval: 10 * time.Second,
	}, contract)

	feature := NewFeature(spec)
	app := kingpin.New("test", "")
	app.Terminate(func(int) {})
	feature.RegisterFlags(app)
	if _, err := app.Parse([]string{"--contract.name=example", "--contract.refresh-interval=15s"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	config := feature.RuntimeConfig()
	if got := configValue(t, config, "refresh_interval"); got != 15*time.Second {
		t.Fatalf("refresh_interval = %v, want 15s", got)
	}
	if got := configValue(t, config, "name"); got != "example" {
		t.Fatalf("name = %v, want example", got)
	}
	if got := feature.SmokeSpec().WantMetrics; !reflect.DeepEqual(got, []string{"contract_metric 1"}) {
		t.Fatalf("SmokeSpec().WantMetrics = %v, want contract metric", got)
	}
}

func TestNewContractSnapshotFeatureSpecAcceptsNilContract(t *testing.T) {
	t.Parallel()

	spec := NewContractSnapshotFeatureSpec[contractTestConfig, contractTestSnapshot](SpecOptions{}, nil)
	feature := NewFeature(spec)
	if got := feature.FeatureName(); got != "exporter" {
		t.Fatalf("FeatureName() = %q, want exporter", got)
	}
}

func TestNewContractFeatureBuildsFeature(t *testing.T) {
	t.Parallel()

	feature := NewContractFeature[contractTestConfig, contractTestSnapshot](SpecOptions{
		FeatureName: "contract",
	}, contractTestFeature{})
	if got := feature.FeatureName(); got != "contract" {
		t.Fatalf("FeatureName() = %q, want contract", got)
	}
	if got := feature.SmokeSpec().WantMetrics; !reflect.DeepEqual(got, []string{"contract_metric 1"}) {
		t.Fatalf("SmokeSpec().WantMetrics = %v, want contract metric", got)
	}
}

type contractTestFeature struct {
	FeatureDefaults[contractTestConfig, contractTestSnapshot]
}

func (contractTestFeature) DefaultRefreshInterval() time.Duration {
	return time.Minute
}

func (contractTestFeature) DefaultConfig() contractTestConfig {
	return contractTestConfig{Name: "default"}
}

func (contractTestFeature) RegisterFlags(app *kingpin.Application, ctx FlagContext, config *contractTestConfig) {
	app.Flag(ctx.FeatureName+".name", "test name").StringVar(&config.Name)
}

func (contractTestFeature) RuntimeConfig(ctx RuntimeConfigContext[contractTestConfig]) []any {
	return []any{"name", ctx.Config.Name}
}

func (contractTestFeature) SmokeSpec(ctx SmokeContext[contractTestConfig]) SmokeSpec {
	return SmokeSpec{WantMetrics: []string{ctx.FeatureName + "_metric 1"}}
}

func emptySmokeSpec(spec SmokeSpec) bool {
	return len(spec.ServerArgs) == 0 &&
		len(spec.WantMetrics) == 0 &&
		len(spec.RejectMetrics) == 0
}

func configValue(t *testing.T, config []any, key string) any {
	t.Helper()
	for i := 0; i < len(config)-1; i += 2 {
		if config[i] == key {
			return config[i+1]
		}
	}
	t.Fatalf("runtime config key %q not found in %#v", key, config)
	return nil
}
