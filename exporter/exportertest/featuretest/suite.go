package featuretest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/exportertest"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"
)

// FeatureTestFunc is a feature-specific test registered in a FeatureTestSuite.
type FeatureTestFunc func(t *testing.T)

// NewFeatureFunc constructs a featurekit feature for a test suite.
type NewFeatureFunc[C any, S any] func(featurekit.SpecOptions) *featurekit.Feature[C, S]

// FeatureTestSpec describes the shared feature contract and domain hooks tested
// by FeatureTestSuite.
type FeatureTestSpec[C any, S any] struct {
	FeatureName                             string
	MetricNamespace                         string
	ExporterName                            string
	DefaultRefreshInterval                  time.Duration
	DefaultFeatureConfigFileName            string
	NewFeature                              NewFeatureFunc[C, S]
	NewDefaultConfig                        func() C
	FeatureConfigFile                       featurekit.FeatureConfigFileFunc[C]
	NewConfigFileTarget                     func() any
	MetricSpecs                             []featurekit.FeatureMetricSpec
	MetricsFunc                             featurekit.SnapshotMetricsFunc[S]
	StatusFunc                              func(S) framework.SnapshotStatus
	DefaultSnapshotter                      framework.Snapshotter[S]
	SuccessfulSnapshot                      func(time.Time) S
	FailedSnapshot                          func(time.Time, error) S
	ContractFlagArgs                        []string
	ContractRuntimeConfig                   map[string]any
	SkipContractLastCollectionSuccessMetric bool
	CollectorFlagArgs                       []string
	SkipRegisterCollectorsTest              bool
	DefaultRuntimeConfig                    map[string]any
	CheckDefaultSnapshotter                 bool
}

// FeatureTestSuite runs standard scaffolded feature tests plus domain-specific
// tests registered by a concrete exporter.
type FeatureTestSuite[C any, S any] struct {
	spec  FeatureTestSpec[C, S]
	tests []featureTest
}

type featureTest struct {
	name string
	run  FeatureTestFunc
}

// NewFeatureTestSuite creates a suite with the standard scaffolded feature
// tests already registered.
func NewFeatureTestSuite[C any, S any](spec FeatureTestSpec[C, S]) *FeatureTestSuite[C, S] {
	suite := &FeatureTestSuite[C, S]{spec: spec}
	suite.Register("exporter_contract", suite.testExporterContract)
	if !spec.SkipRegisterCollectorsTest {
		suite.Register("exporter_registers_collectors", suite.testExporterRegistersCollectors)
	}
	suite.Register("contract_feature_defaults", suite.testContractFeatureDefaults)
	suite.Register("feature_config_file_hook", suite.testFeatureConfigFileHook)
	suite.Register("feature_config_file_loader", suite.testFeatureConfigFileLoader)
	suite.Register("smoke_spec_includes_config_file", suite.testSmokeSpecIncludesConfigFile)
	suite.Register("metric_name_contract", suite.testMetricNameContract)
	suite.Register("collector_defaults_and_failure_metrics", suite.testCollectorDefaultsAndFailureMetrics)
	suite.Register("collector_background_refresh_updates_snapshot_outside_scrape", suite.testCollectorBackgroundRefreshUpdatesSnapshotOutsideScrape)
	if spec.CheckDefaultSnapshotter {
		suite.Register("collector_uses_default_snapshotter", suite.testCollectorUsesDefaultSnapshotter)
	}
	return suite
}

// Register adds a concrete feature-specific test to the suite.
func (s *FeatureTestSuite[C, S]) Register(name string, run FeatureTestFunc) {
	if name == "" {
		panic("feature test name is required")
	}
	if run == nil {
		panic("feature test function is required")
	}
	s.tests = append(s.tests, featureTest{name: name, run: run})
}

// RunTests executes all registered suite tests as parallel subtests.
func (s *FeatureTestSuite[C, S]) RunTests(t *testing.T) {
	t.Helper()
	if len(s.tests) == 0 {
		t.Fatal("feature test suite has no registered tests")
	}
	for _, test := range s.tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			test.run(t)
		})
	}
}

// NewFeature constructs a feature through FeatureTestSpec.NewFeature.
func (s *FeatureTestSuite[C, S]) NewFeature(options featurekit.SpecOptions) *featurekit.Feature[C, S] {
	if s.spec.NewFeature == nil {
		panic("FeatureTestSpec.NewFeature is required")
	}
	return s.spec.NewFeature(options)
}

// NewNamedFeature constructs a feature with the suite feature name.
func (s *FeatureTestSuite[C, S]) NewNamedFeature() *featurekit.Feature[C, S] {
	return s.NewFeature(featurekit.SpecOptions{FeatureName: s.featureName()})
}

// FeatureContext returns the standard framework feature context for this suite.
func (s *FeatureTestSuite[C, S]) FeatureContext() framework.FeatureContext {
	return framework.FeatureContext{
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		ExporterName: s.exporterName(),
		Namespace:    s.metricNamespace(),
	}
}

// ParseFeatureFlags registers and parses feature flags for a feature under test.
func (s *FeatureTestSuite[C, S]) ParseFeatureFlags(t *testing.T, feature *featurekit.Feature[C, S], args []string) {
	t.Helper()

	exportertest.ParseFeatureFlags(t, feature, args)
}

// RegisterFeatureCollectors registers a feature's collectors in a fresh
// registry using the suite feature context.
func (s *FeatureTestSuite[C, S]) RegisterFeatureCollectors(t *testing.T, feature interface {
	RegisterCollectors(framework.FeatureContext, *prometheus.Registry) error
}) *prometheus.Registry {
	t.Helper()

	registry := prometheus.NewRegistry()
	if err := feature.RegisterCollectors(s.FeatureContext(), registry); err != nil {
		t.Fatalf("RegisterCollectors() error = %v", err)
	}
	return registry
}

// NewFakeSnapshotter returns a mutable snapshotter useful for collector tests.
func (s *FeatureTestSuite[C, S]) NewFakeSnapshotter(snapshot S) *FakeSnapshotter[S] {
	return NewFakeSnapshotter(snapshot)
}

// NewCollector creates a standard featurekit snapshot metrics collector.
func (s *FeatureTestSuite[C, S]) NewCollector(featureName string, namespace string, snapshotter framework.Snapshotter[S], refreshInterval time.Duration) framework.StartableCollector {
	return s.NewCollectorWithNow(featureName, namespace, nil, snapshotter, refreshInterval, nil)
}

// NewCollectorWithNow creates a standard featurekit snapshot metrics collector
// with an optional clock override.
func (s *FeatureTestSuite[C, S]) NewCollectorWithNow(featureName string, namespace string, logger *slog.Logger, snapshotter framework.Snapshotter[S], refreshInterval time.Duration, now func() time.Time) framework.StartableCollector {
	return featurekit.NewSnapshotMetricsCollector(featurekit.SnapshotMetricsCollectorOptions[S]{
		SnapshotCollectorOptions: featurekit.SnapshotCollectorOptions[S]{
			FeatureName:            featureName,
			Namespace:              namespace,
			Logger:                 logger,
			Snapshotter:            snapshotter,
			DefaultSnapshotter:     s.spec.DefaultSnapshotter,
			RefreshInterval:        refreshInterval,
			DefaultRefreshInterval: s.defaultRefreshInterval(),
			StatusFunc:             s.spec.StatusFunc,
			Now:                    now,
		},
		MetricsFunc: s.spec.MetricsFunc,
	})
}

// StartCollector starts a collector and registers it in a fresh registry.
func (s *FeatureTestSuite[C, S]) StartCollector(t *testing.T, collector framework.StartableCollector) *prometheus.Registry {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	collector.Start(ctx)

	registry := prometheus.NewRegistry()
	exportertest.Register(t, registry, collector)
	return registry
}

// MetricName resolves a metric ID through the suite feature metric specs.
func (s *FeatureTestSuite[C, S]) MetricName(featureName string, namespace string, id string) string {
	return featurekit.FeatureMetricName(featureName, namespace, id, s.spec.MetricSpecs)
}

// LastCollectionSuccessMetric returns the standard last-collection success
// metric name for the suite namespace.
func (s *FeatureTestSuite[C, S]) LastCollectionSuccessMetric() string {
	return s.metricNamespace() + "_last_collection_success"
}

// LastCollectionTimestampMetric returns the standard last-collection timestamp
// metric name for the suite namespace.
func (s *FeatureTestSuite[C, S]) LastCollectionTimestampMetric() string {
	return s.metricNamespace() + "_last_collection_timestamp_seconds"
}

// LastSuccessfulCollectionTimestampMetric returns the standard successful
// collection timestamp metric name for the suite namespace.
func (s *FeatureTestSuite[C, S]) LastSuccessfulCollectionTimestampMetric() string {
	return s.metricNamespace() + "_last_successful_collection_timestamp_seconds"
}

// WriteConfig writes a temporary feature config file and returns its path.
func (s *FeatureTestSuite[C, S]) WriteConfig(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "feature.yml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
	return path
}

func (s *FeatureTestSuite[C, S]) successfulSnapshot(t *testing.T, at time.Time) S {
	t.Helper()
	if s.spec.SuccessfulSnapshot == nil {
		t.Fatal("FeatureTestSpec.SuccessfulSnapshot is required")
	}
	return s.spec.SuccessfulSnapshot(at)
}

func (s *FeatureTestSuite[C, S]) failedSnapshot(t *testing.T, at time.Time, err error) S {
	t.Helper()
	if s.spec.FailedSnapshot == nil {
		t.Fatal("FeatureTestSpec.FailedSnapshot is required")
	}
	return s.spec.FailedSnapshot(at, err)
}

func (s *FeatureTestSuite[C, S]) testExporterContract(t *testing.T) {
	flagArgs := []string{
		"--" + s.featureName() + ".refresh-interval=30s",
	}
	flagArgs = append(flagArgs, s.spec.ContractFlagArgs...)
	wantRuntimeConfig := mergeRuntimeConfig(map[string]any{
		"refresh_interval":   30 * time.Second,
		"config_file":        featurekit.DefaultFeatureConfigFile(s.featureName()),
		"config_file_loaded": false,
	}, s.spec.ContractRuntimeConfig)
	lastCollectionSuccessMetric := s.LastCollectionSuccessMetric()
	if s.spec.SkipContractLastCollectionSuccessMetric {
		lastCollectionSuccessMetric = ""
	}

	exportertest.RunFeatureContract(t, exportertest.FeatureContractConfig{
		NewFeature: func() exportertest.FeatureContractFeature {
			return s.NewNamedFeature()
		},
		FeatureContext:              s.FeatureContext(),
		FlagArgs:                    flagArgs,
		WantRuntimeConfig:           wantRuntimeConfig,
		DuplicateRegistration:       true,
		LastCollectionSuccessMetric: lastCollectionSuccessMetric,
	})
}

func (s *FeatureTestSuite[C, S]) testExporterRegistersCollectors(t *testing.T) {
	feature := s.NewNamedFeature()
	s.ParseFeatureFlags(t, feature, s.spec.CollectorFlagArgs)
	registry := s.RegisterFeatureCollectors(t, feature)
	exportertest.WaitForMetricValue(t, registry, s.LastCollectionSuccessMetric(), nil, 1)
}

func (s *FeatureTestSuite[C, S]) testContractFeatureDefaults(t *testing.T) {
	feature := s.NewFeature(featurekit.SpecOptions{})
	s.ParseFeatureFlags(t, feature, []string{})
	config := feature.RuntimeConfig()
	wantRuntimeConfig := mergeRuntimeConfig(map[string]any{
		"refresh_interval":   s.defaultRefreshInterval(),
		"config_file":        featurekit.DefaultFeatureConfigFile(""),
		"config_file_loaded": false,
	}, s.spec.DefaultRuntimeConfig)
	exportertest.AssertRuntimeConfigValues(t, config, wantRuntimeConfig)
}

func (s *FeatureTestSuite[C, S]) testFeatureConfigFileHook(t *testing.T) {
	if s.spec.NewDefaultConfig == nil {
		t.Fatal("FeatureTestSpec.NewDefaultConfig is required")
	}
	if s.spec.FeatureConfigFile == nil {
		t.Fatal("FeatureTestSpec.FeatureConfigFile is required")
	}

	config := s.spec.NewDefaultConfig()
	configFile := s.WriteConfig(t, "{}\n")
	*s.spec.FeatureConfigFile(&config) = configFile
	if got := *s.spec.FeatureConfigFile(&config); got != configFile {
		t.Fatalf("ConfigFile = %q, want %q", got, configFile)
	}

	feature := s.NewNamedFeature()
	s.ParseFeatureFlags(t, feature, []string{"--" + s.featureName() + ".config-file=" + configFile})
	if got := exportertest.RuntimeConfigValue(t, feature.RuntimeConfig(), "config_file"); got != configFile {
		t.Fatalf("config_file = %q, want %q", got, configFile)
	}
	if got := exportertest.RuntimeConfigValue(t, feature.RuntimeConfig(), "config_file_loaded"); got != true {
		t.Fatalf("config_file_loaded = %v, want true", got)
	}

	missingFeature := s.NewNamedFeature()
	s.ParseFeatureFlags(t, missingFeature, []string{"--" + s.featureName() + ".config-file=" + filepath.Join(t.TempDir(), "missing.yml")})
	if err := missingFeature.RegisterCollectors(s.FeatureContext(), prometheus.NewRegistry()); err == nil {
		t.Fatal("RegisterCollectors() error = nil, want missing explicit config file error")
	}
}

func (s *FeatureTestSuite[C, S]) testFeatureConfigFileLoader(t *testing.T) {
	if s.spec.NewConfigFileTarget == nil {
		t.Fatal("FeatureTestSpec.NewConfigFileTarget is required")
	}

	if got := featurekit.DefaultFeatureConfigFile(" custom "); got != filepath.Join("/etc/prometheus", "prometheus-custom-exporter.yml") {
		t.Fatalf("DefaultFeatureConfigFile(custom) = %q, want default custom path", got)
	}
	if got := featurekit.DefaultFeatureConfigFile(" "); got != filepath.Join("/etc/prometheus", "prometheus-exporter-exporter.yml") {
		t.Fatalf("DefaultFeatureConfigFile(empty) = %q, want default exporter path", got)
	}

	missingPath := filepath.Join(t.TempDir(), "missing.yml")
	path, loaded, err := featurekit.LoadFeatureConfigFile(s.featureName(), missingPath, s.spec.NewConfigFileTarget())
	if err == nil {
		t.Fatal("LoadFeatureConfigFile() error = nil, want missing explicit file error")
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("LoadFeatureConfigFile() error = %v, want os.ErrNotExist", err)
	}
	if path != missingPath || loaded {
		t.Fatalf("LoadFeatureConfigFile() path/loaded = %q/%v, want %q/false", path, loaded, missingPath)
	}

	badPath := s.WriteConfig(t, "unknown: true\n")
	if _, loaded, err := featurekit.LoadFeatureConfigFile(s.featureName(), badPath, s.spec.NewConfigFileTarget()); err == nil || loaded {
		t.Fatalf("LoadFeatureConfigFile(strict) loaded/error = %v/%v, want false/error", loaded, err)
	}

	configPath := s.WriteConfig(t, "{}\n")
	path, loaded, err = featurekit.LoadFeatureConfigFile(s.featureName(), " "+configPath+" ", s.spec.NewConfigFileTarget())
	if err != nil {
		t.Fatalf("LoadFeatureConfigFile(valid) error = %v, want nil", err)
	}
	if path != configPath || !loaded {
		t.Fatalf("LoadFeatureConfigFile(valid) path/loaded = %q/%v, want %q/true", path, loaded, configPath)
	}
}

func (s *FeatureTestSuite[C, S]) testSmokeSpecIncludesConfigFile(t *testing.T) {
	spec := s.NewNamedFeature().SmokeSpec()
	wantConfig := "--" + s.featureName() + ".config-file=../examples/" + s.defaultFeatureConfigFileName()
	if !HasString(spec.ServerArgs, wantConfig) {
		t.Fatalf("SmokeSpec().ServerArgs = %v, want %q", spec.ServerArgs, wantConfig)
	}
}

func (s *FeatureTestSuite[C, S]) testMetricNameContract(t *testing.T) {
	if len(s.spec.MetricSpecs) == 0 {
		t.Fatal("FeatureTestSpec.MetricSpecs is empty")
	}
	if got := s.MetricName("feature", "namespace", s.spec.MetricSpecs[0].ID); got != s.spec.MetricSpecs[0].MetricName("feature", "namespace") {
		t.Fatalf("MetricName(known) = %q, want descriptor spec name", got)
	}
	assertPanic(t, func() {
		s.MetricName("feature", "namespace", "missing_metric")
	})
}

func (s *FeatureTestSuite[C, S]) testCollectorDefaultsAndFailureMetrics(t *testing.T) {
	collector := s.NewCollector("", "", s.NewFakeSnapshotter(s.failedSnapshot(t, time.Time{}, errors.New("refresh failed"))), 0)

	expected := fmt.Sprintf(`
# HELP %[1]s Whether the last %[4]s data collection succeeded
# TYPE %[1]s gauge
%[1]s 0
# HELP %[2]s Unix timestamp of the last %[4]s data collection attempt
# TYPE %[2]s gauge
%[2]s 0
# HELP %[3]s Unix timestamp of the last successful %[4]s data collection
# TYPE %[3]s gauge
%[3]s 0
`, "exporter_last_collection_success", "exporter_last_collection_timestamp_seconds", "exporter_last_successful_collection_timestamp_seconds", "exporter")

	if err := testutil.CollectAndCompare(collector, strings.NewReader(expected),
		"exporter_last_collection_success",
		"exporter_last_collection_timestamp_seconds",
		"exporter_last_successful_collection_timestamp_seconds",
	); err != nil {
		t.Fatalf("CollectAndCompare() error = %v", err)
	}
}

func (s *FeatureTestSuite[C, S]) testCollectorBackgroundRefreshUpdatesSnapshotOutsideScrape(t *testing.T) {
	start := time.Unix(1700000000, 0)
	snapshotter := s.NewFakeSnapshotter(s.successfulSnapshot(t, start))
	collector := s.NewCollectorWithNow(s.featureName(), s.metricNamespace(), slog.New(slog.NewTextHandler(io.Discard, nil)), snapshotter, 20*time.Millisecond, nil)

	registry := s.StartCollector(t, collector)
	exportertest.WaitForMetricValue(t, registry, s.LastCollectionSuccessMetric(), nil, 1)

	snapshotter.Set(s.failedSnapshot(t, start.Add(time.Minute), errors.New("refresh failed")))
	exportertest.WaitForMetricValue(t, registry, s.LastCollectionSuccessMetric(), nil, 0)
}

func (s *FeatureTestSuite[C, S]) testCollectorUsesDefaultSnapshotter(t *testing.T) {
	now := time.Unix(1700000000, 0)
	collector := s.NewCollectorWithNow(s.featureName(), s.metricNamespace(), slog.New(slog.NewTextHandler(io.Discard, nil)), nil, s.defaultRefreshInterval(), func() time.Time {
		return now
	})

	families := exportertest.RegisterAndGather(t, collector)
	exportertest.AssertMetricValue(t, families, s.LastCollectionSuccessMetric(), nil, 1)
	exportertest.AssertMetricValue(t, families, s.LastCollectionTimestampMetric(), nil, float64(now.Unix()))
	exportertest.AssertMetricValue(t, families, s.LastSuccessfulCollectionTimestampMetric(), nil, float64(now.Unix()))
}

func (s *FeatureTestSuite[C, S]) featureName() string {
	if s.spec.FeatureName == "" {
		return "exporter"
	}
	return s.spec.FeatureName
}

func (s *FeatureTestSuite[C, S]) metricNamespace() string {
	if s.spec.MetricNamespace == "" {
		return "exporter"
	}
	return s.spec.MetricNamespace
}

func (s *FeatureTestSuite[C, S]) exporterName() string {
	if s.spec.ExporterName == "" {
		return "prometheus-exporter"
	}
	return s.spec.ExporterName
}

func (s *FeatureTestSuite[C, S]) defaultRefreshInterval() time.Duration {
	if s.spec.DefaultRefreshInterval <= 0 {
		return framework.DefaultSnapshotRefreshInterval
	}
	return s.spec.DefaultRefreshInterval
}

func (s *FeatureTestSuite[C, S]) defaultFeatureConfigFileName() string {
	if s.spec.DefaultFeatureConfigFileName == "" {
		return "prometheus-" + s.featureName() + "-exporter.yml"
	}
	return s.spec.DefaultFeatureConfigFileName
}

func mergeRuntimeConfig(base map[string]any, overrides map[string]any) map[string]any {
	merged := make(map[string]any, len(base)+len(overrides))
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range overrides {
		merged[key] = value
	}
	return merged
}

// HasString reports whether values contains want.
func HasString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

// FakeSnapshotter is a mutable framework.Snapshotter implementation for tests.
type FakeSnapshotter[S any] struct {
	snapshot atomic.Value
}

// NewFakeSnapshotter creates a fake snapshotter initialized with snapshot.
func NewFakeSnapshotter[S any](snapshot S) *FakeSnapshotter[S] {
	s := &FakeSnapshotter[S]{}
	s.snapshot.Store(snapshot)
	return s
}

// Snapshot returns the currently stored snapshot.
func (s *FakeSnapshotter[S]) Snapshot(context.Context, time.Time) S {
	return s.snapshot.Load().(S)
}

// Set replaces the snapshot returned by Snapshot.
func (s *FakeSnapshotter[S]) Set(snapshot S) {
	s.snapshot.Store(snapshot)
}

func assertPanic(t *testing.T, run func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("panic was not raised")
		}
	}()
	run()
}
