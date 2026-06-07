package featuretest

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"
)

const (
	testDefaultRefreshInterval = time.Minute
	testMetricExampleValue     = "example_value"
)

type testConfig struct {
	ConfigFile string
	Name       string
}

type testFileConfig struct {
	Name string `yaml:"name"`
}

type testSnapshot struct {
	AttemptTime time.Time
	Success     bool
	Value       float64
	Err         error
}

type testEngine struct {
	snapshot testSnapshot
}

func (e testEngine) Snapshot(context.Context, time.Time) testSnapshot {
	return e.snapshot
}

var testMetricSpecs = []featurekit.FeatureMetricSpec{
	{
		ID:    testMetricExampleValue,
		Scope: featurekit.MetricScopeFeature,
		Name:  "_example_value",
		Help:  "Example metric emitted by a feature test fixture.",
	},
}

func TestFeatureTestSuiteRunsStandardAndRegisteredTests(t *testing.T) {
	t.Parallel()

	suite := NewFeatureTestSuite(newTestFeatureSpec(FeatureTestSpec[testConfig, testSnapshot]{
		FeatureName:                  "demo",
		MetricNamespace:              "demo_exporter",
		ExporterName:                 "prometheus-demo-exporter",
		DefaultRefreshInterval:       testDefaultRefreshInterval,
		DefaultFeatureConfigFileName: "prometheus-demo-exporter.yml",
		ContractFlagArgs: []string{
			"--demo.name=custom",
		},
		ContractRuntimeConfig: map[string]any{
			"name": "custom",
		},
		DefaultRuntimeConfig: map[string]any{
			"name": "default",
		},
		CheckDefaultSnapshotter: true,
	}))
	suite.Register("collector_exports_snapshot", func(t *testing.T) {
		testCollectorExportsSnapshot(t, suite)
	})
	suite.Register("smoke_spec_includes_metric", func(t *testing.T) {
		testSmokeSpecIncludesMetric(t, suite)
	})
	suite.RunTests(t)
}

func TestFeatureTestSuiteRunsWithDefaultNamesAndSkippedCollectors(t *testing.T) {
	t.Parallel()

	suite := NewFeatureTestSuite(newTestFeatureSpec(FeatureTestSpec[testConfig, testSnapshot]{
		DefaultRefreshInterval:                  testDefaultRefreshInterval,
		SkipContractLastCollectionSuccessMetric: true,
		SkipRegisterCollectorsTest:              true,
		ContractFlagArgs: []string{
			"--exporter.name=custom",
		},
		ContractRuntimeConfig: map[string]any{
			"name": "custom",
		},
		DefaultRuntimeConfig: map[string]any{
			"name": "default",
		},
	}))
	suite.RunTests(t)
}

func TestFeatureTestSuiteRejectsInvalidRegisteredTests(t *testing.T) {
	t.Parallel()

	suite := NewFeatureTestSuite(newTestFeatureSpec(FeatureTestSpec[testConfig, testSnapshot]{
		SkipContractLastCollectionSuccessMetric: true,
		SkipRegisterCollectorsTest:              true,
	}))
	assertPanic(t, func() {
		suite.Register("", func(*testing.T) {})
	})
	assertPanic(t, func() {
		suite.Register("invalid", nil)
	})
}

func TestFeatureTestSuiteRequiresFeatureFactory(t *testing.T) {
	t.Parallel()

	suite := NewFeatureTestSuite(FeatureTestSpec[testConfig, testSnapshot]{
		SkipRegisterCollectorsTest: true,
	})
	assertPanic(t, func() {
		suite.NewFeature(featurekit.SpecOptions{})
	})
}

func TestFeatureTestSuiteRegisterFeatureCollectorsReportsError(t *testing.T) {
	t.Parallel()

	suite := NewFeatureTestSuite(newTestFeatureSpec(FeatureTestSpec[testConfig, testSnapshot]{
		SkipContractLastCollectionSuccessMetric: true,
		SkipRegisterCollectorsTest:              true,
	}))
	if HasString([]string{"a", "b"}, "c") {
		t.Fatal("HasString() = true, want false")
	}
	assertPanic(t, func() {
		suite.MetricName("demo", "", "missing")
	})
}

func testCollectorExportsSnapshot(t *testing.T, suite *FeatureTestSuite[testConfig, testSnapshot]) {
	now := time.Unix(1700000000, 0)
	collector := suite.NewCollectorWithNow(
		"demo",
		"demo_exporter",
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		suite.NewFakeSnapshotter(testSnapshot{
			AttemptTime: now,
			Success:     true,
			Value:       42,
		}),
		testDefaultRefreshInterval,
		func() time.Time { return now },
	)

	expected := fmt.Sprintf(`
# HELP %[1]s Example metric emitted by a feature test fixture.
# TYPE %[1]s gauge
%[1]s 42
# HELP %[2]s Whether the last demo data collection succeeded
# TYPE %[2]s gauge
%[2]s 1
# HELP %[3]s Unix timestamp of the last demo data collection attempt
# TYPE %[3]s gauge
%[3]s 1.7e+09
# HELP %[4]s Unix timestamp of the last successful demo data collection
# TYPE %[4]s gauge
%[4]s 1.7e+09
`, suite.MetricName("demo", "", testMetricExampleValue), suite.LastCollectionSuccessMetric(), suite.LastCollectionTimestampMetric(), suite.LastSuccessfulCollectionTimestampMetric())

	if err := testutil.CollectAndCompare(collector, strings.NewReader(expected),
		suite.MetricName("demo", "", testMetricExampleValue),
		suite.LastCollectionSuccessMetric(),
		suite.LastCollectionTimestampMetric(),
		suite.LastSuccessfulCollectionTimestampMetric(),
	); err != nil {
		t.Fatalf("CollectAndCompare() error = %v", err)
	}
}

func testSmokeSpecIncludesMetric(t *testing.T, suite *FeatureTestSuite[testConfig, testSnapshot]) {
	spec := suite.NewNamedFeature().SmokeSpec()
	want := suite.MetricName("demo", "", testMetricExampleValue) + " 1"
	if !HasString(spec.WantMetrics, want) {
		t.Fatalf("SmokeSpec().WantMetrics = %v, want %q", spec.WantMetrics, want)
	}
}

func newTestFeatureSpec(overrides FeatureTestSpec[testConfig, testSnapshot]) FeatureTestSpec[testConfig, testSnapshot] {
	spec := FeatureTestSpec[testConfig, testSnapshot]{
		NewFeature:          newTestFeature,
		NewDefaultConfig:    newTestDefaultConfig,
		FeatureConfigFile:   testFeatureConfigFile,
		NewConfigFileTarget: func() any { return &testFileConfig{} },
		MetricSpecs:         testMetricSpecs,
		MetricsFunc:         testMetricsFunc,
		StatusFunc:          testSnapshotStatus,
		DefaultSnapshotter:  testEngine{snapshot: testSnapshot{AttemptTime: time.Unix(1700000000, 0), Success: true, Value: 1}},
		SuccessfulSnapshot: func(at time.Time) testSnapshot {
			return testSnapshot{
				AttemptTime: at,
				Success:     true,
				Value:       1,
			}
		},
		FailedSnapshot: func(at time.Time, err error) testSnapshot {
			return testSnapshot{
				AttemptTime: at,
				Success:     false,
				Err:         err,
			}
		},
	}
	if overrides.FeatureName != "" {
		spec.FeatureName = overrides.FeatureName
	}
	if overrides.MetricNamespace != "" {
		spec.MetricNamespace = overrides.MetricNamespace
	}
	if overrides.ExporterName != "" {
		spec.ExporterName = overrides.ExporterName
	}
	if overrides.DefaultRefreshInterval != 0 {
		spec.DefaultRefreshInterval = overrides.DefaultRefreshInterval
	}
	if overrides.DefaultFeatureConfigFileName != "" {
		spec.DefaultFeatureConfigFileName = overrides.DefaultFeatureConfigFileName
	}
	if overrides.NewFeature != nil {
		spec.NewFeature = overrides.NewFeature
	}
	if overrides.NewDefaultConfig != nil {
		spec.NewDefaultConfig = overrides.NewDefaultConfig
	}
	if overrides.FeatureConfigFile != nil {
		spec.FeatureConfigFile = overrides.FeatureConfigFile
	}
	if overrides.NewConfigFileTarget != nil {
		spec.NewConfigFileTarget = overrides.NewConfigFileTarget
	}
	if overrides.MetricSpecs != nil {
		spec.MetricSpecs = overrides.MetricSpecs
	}
	if overrides.MetricsFunc != nil {
		spec.MetricsFunc = overrides.MetricsFunc
	}
	if overrides.StatusFunc != nil {
		spec.StatusFunc = overrides.StatusFunc
	}
	if overrides.DefaultSnapshotter != nil {
		spec.DefaultSnapshotter = overrides.DefaultSnapshotter
	}
	if overrides.SuccessfulSnapshot != nil {
		spec.SuccessfulSnapshot = overrides.SuccessfulSnapshot
	}
	if overrides.FailedSnapshot != nil {
		spec.FailedSnapshot = overrides.FailedSnapshot
	}
	spec.ContractFlagArgs = overrides.ContractFlagArgs
	spec.ContractRuntimeConfig = overrides.ContractRuntimeConfig
	spec.SkipContractLastCollectionSuccessMetric = overrides.SkipContractLastCollectionSuccessMetric
	spec.CollectorFlagArgs = overrides.CollectorFlagArgs
	spec.SkipRegisterCollectorsTest = overrides.SkipRegisterCollectorsTest
	spec.DefaultRuntimeConfig = overrides.DefaultRuntimeConfig
	spec.CheckDefaultSnapshotter = overrides.CheckDefaultSnapshotter
	return spec
}

func newTestFeature(options featurekit.SpecOptions) *featurekit.Feature[testConfig, testSnapshot] {
	return featurekit.NewSnapshotExtensionFeature(options, featurekit.SnapshotFeatureExtension[testConfig, testSnapshot]{
		DefaultRefreshInterval: testDefaultRefreshInterval,
		DefaultConfigFunc:      newTestDefaultConfig,
		ConfigFileFunc:         testFeatureConfigFile,
		ConfigFlagSpecs: []featurekit.FeatureConfigFlagSpec[testConfig]{
			{
				Name:    "name",
				Help:    "Test name",
				Default: "default",
				Bind: func(flag *kingpin.FlagClause, config *testConfig) {
					flag.StringVar(&config.Name)
				},
			},
		},
		ResolveConfigFunc: testResolveConfig,
		RuntimeConfigFunc: func(_ featurekit.RuntimeConfigContext[testConfig], config testConfig) []any {
			return []any{"name", config.Name}
		},
		SnapshotEngineFactory: func(ctx featurekit.CollectorContext[testConfig]) (featurekit.SnapshotEngine[testSnapshot], error) {
			config, _, _, err := testResolveConfig(ctx.FeatureName, ctx.Config)
			if err != nil {
				return nil, err
			}
			value := 1.0
			if config.Name == "custom" {
				value = 2
			}
			return testEngine{snapshot: testSnapshot{AttemptTime: time.Unix(1700000000, 0), Success: true, Value: value}}, nil
		},
		DefaultSnapshotEngine: testEngine{snapshot: testSnapshot{AttemptTime: time.Unix(1700000000, 0), Success: true, Value: 1}},
		MetricSpecs:           testMetricSpecs,
		MetricHandlers: featurekit.FeatureMetricHandlers[testSnapshot]{
			Collect: func(ctx featurekit.FeatureMetricsContext[testSnapshot], ch chan<- prometheus.Metric, snapshot testSnapshot, _ time.Time) {
				ch <- prometheus.MustNewConstMetric(ctx.Descriptors.Get(testMetricExampleValue), prometheus.GaugeValue, snapshot.Value)
			},
		},
		StatusFunc: testSnapshotStatus,
		SmokeFunc: func(ctx featurekit.SmokeContext[testConfig]) featurekit.SmokeSpec {
			return featurekit.SmokeSpec{
				ServerArgs: []string{
					"--" + ctx.FeatureName + ".config-file=../examples/prometheus-" + ctx.FeatureName + "-exporter.yml",
				},
				WantMetrics: []string{
					featurekit.FeatureMetricName(ctx.FeatureName, "", testMetricExampleValue, testMetricSpecs) + " 1",
				},
			}
		},
	})
}

func newTestDefaultConfig() testConfig {
	return testConfig{Name: "default"}
}

func testFeatureConfigFile(config *testConfig) *string {
	return &config.ConfigFile
}

func testResolveConfig(featureName string, config testConfig) (testConfig, string, bool, error) {
	var fileConfig testFileConfig
	path, loaded, err := featurekit.LoadFeatureConfigFile(featureName, config.ConfigFile, &fileConfig)
	if err != nil || !loaded {
		return config, path, loaded, err
	}
	if config.Name == "default" && fileConfig.Name != "" {
		config.Name = fileConfig.Name
	}
	return config, path, true, nil
}

func testMetricsFunc(ctx featurekit.SnapshotMetricsContext[testSnapshot]) featurekit.SnapshotMetrics[testSnapshot] {
	return featurekit.NewFeatureMetrics(ctx, testMetricSpecs, featurekit.FeatureMetricHandlers[testSnapshot]{
		Collect: func(ctx featurekit.FeatureMetricsContext[testSnapshot], ch chan<- prometheus.Metric, snapshot testSnapshot, _ time.Time) {
			ch <- prometheus.MustNewConstMetric(ctx.Descriptors.Get(testMetricExampleValue), prometheus.GaugeValue, snapshot.Value)
		},
	})
}

func testSnapshotStatus(snapshot testSnapshot) framework.SnapshotStatus {
	return framework.SnapshotStatus{
		AttemptTime: snapshot.AttemptTime,
		Success:     snapshot.Success,
	}
}
