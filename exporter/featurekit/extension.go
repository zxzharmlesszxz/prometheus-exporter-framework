package featurekit

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/kingpin/v2"
	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
	"go.yaml.in/yaml/v3"
)

type FeatureConfigFileFunc[C any] func(config *C) *string

type FeatureConfigResolver[C any] func(featureName string, config C) (C, string, bool, error)

type SnapshotFeatureExtension[C any, S any] struct {
	DefaultRefreshInterval time.Duration
	// SyncRefreshTimeout bounds scrape-triggered synchronous refreshes when the
	// cache is empty or stale and the background refresh loop is not running. A
	// zero value preserves the core collector's historical unbounded behavior.
	SyncRefreshTimeout    time.Duration
	DefaultConfigFunc     func() C
	ConfigFileFunc        FeatureConfigFileFunc[C]
	ConfigFlagSpecs       []FeatureConfigFlagSpec[C]
	RegisterFlagsFunc     func(app *kingpin.Application, ctx FlagContext, config *C)
	ValidateConfigFunc    func(config C) error
	ResolveConfigFunc     FeatureConfigResolver[C]
	RuntimeConfigFunc     func(ctx RuntimeConfigContext[C], config C) []any
	SnapshotEngineFactory SnapshotEngineFactory[C, S]
	DefaultSnapshotEngine SnapshotEngine[S]
	NewSnapshotterFunc    func(ctx CollectorContext[C]) (framework.Snapshotter[S], error)
	DefaultSnapshotter    framework.Snapshotter[S]
	MetricSpecs           []FeatureMetricSpec
	MetricHandlers        FeatureMetricHandlers[S]
	MetricsFunc           SnapshotMetricsFunc[S]
	StatusFunc            func(S) framework.SnapshotStatus
	ErrorLogFunc          func(*slog.Logger, S)
	Smoke                 SmokeSpec
	SmokeFunc             func(ctx SmokeContext[C]) SmokeSpec
}

func NewSnapshotExtensionFeatureSpec[C any, S any](options SpecOptions, extension SnapshotFeatureExtension[C, S]) FeatureSpec[C, S] {
	configState := newSnapshotExtensionConfigState(extension)
	return NewSnapshotFeatureSpec(SnapshotFeatureSpec[C, S]{
		Options:                options,
		DefaultRefreshInterval: extension.DefaultRefreshInterval,
		SyncRefreshTimeout:     extension.SyncRefreshTimeout,
		Config:                 defaultFeatureConfig(extension.DefaultConfigFunc),
		RegisterFlagsFunc:      snapshotExtensionRegisterFlags(extension),
		PrepareConfigFunc:      configState.prepareConfigFunc(),
		ValidateConfigFunc:     extension.ValidateConfigFunc,
		NewSnapshotterFunc:     snapshotExtensionNewSnapshotter(extension),
		DefaultSnapshotter:     snapshotExtensionDefaultSnapshotter(extension),
		MetricsFunc:            snapshotExtensionMetricsFunc(extension),
		StatusFunc:             extension.StatusFunc,
		ErrorLogFunc:           extension.ErrorLogFunc,
		RuntimeConfigFunc:      configState.runtimeConfigFunc(),
		Smoke:                  extension.Smoke,
		SmokeFunc:              extension.SmokeFunc,
	})
}

func NewSnapshotExtensionFeature[C any, S any](options SpecOptions, extension SnapshotFeatureExtension[C, S]) *Feature[C, S] {
	return NewFeature(NewSnapshotExtensionFeatureSpec(options, extension))
}

func snapshotExtensionNewSnapshotter[C any, S any](extension SnapshotFeatureExtension[C, S]) func(ctx CollectorContext[C]) (framework.Snapshotter[S], error) {
	if extension.NewSnapshotterFunc != nil {
		return extension.NewSnapshotterFunc
	}
	if extension.SnapshotEngineFactory == nil {
		return nil
	}
	return func(ctx CollectorContext[C]) (framework.Snapshotter[S], error) {
		return extension.SnapshotEngineFactory(ctx)
	}
}

func snapshotExtensionDefaultSnapshotter[C any, S any](extension SnapshotFeatureExtension[C, S]) framework.Snapshotter[S] {
	if extension.DefaultSnapshotter != nil {
		return extension.DefaultSnapshotter
	}
	return extension.DefaultSnapshotEngine
}

func snapshotExtensionMetricsFunc[C any, S any](extension SnapshotFeatureExtension[C, S]) SnapshotMetricsFunc[S] {
	if extension.MetricsFunc != nil {
		return extension.MetricsFunc
	}
	if len(extension.MetricSpecs) == 0 && extension.MetricHandlers.Collect == nil && extension.MetricHandlers.LogError == nil {
		return nil
	}
	return func(ctx SnapshotMetricsContext[S]) SnapshotMetrics[S] {
		return NewFeatureMetrics(ctx, extension.MetricSpecs, extension.MetricHandlers)
	}
}

type snapshotExtensionConfigState[C any, S any] struct {
	extension  SnapshotFeatureExtension[C, S]
	mu         sync.Mutex
	configFile string
	loaded     bool
	prepared   bool
}

func newSnapshotExtensionConfigState[C any, S any](extension SnapshotFeatureExtension[C, S]) *snapshotExtensionConfigState[C, S] {
	return &snapshotExtensionConfigState[C, S]{extension: extension}
}

func (s *snapshotExtensionConfigState[C, S]) prepareConfigFunc() func(string, C) (C, error) {
	if s.extension.ConfigFileFunc == nil && s.extension.ResolveConfigFunc == nil {
		return nil
	}
	return func(featureName string, config C) (C, error) {
		resolved, configFile, loaded, err := ResolveFeatureConfig(featureName, config, s.extension.ConfigFileFunc, s.extension.ResolveConfigFunc)
		if err != nil {
			return resolved, err
		}
		s.mu.Lock()
		defer s.mu.Unlock()
		s.configFile = configFile
		s.loaded = loaded
		s.prepared = true
		return resolved, err
	}
}

func DefaultFeatureConfigFile(featureName string) string {
	name := strings.TrimSpace(featureName)
	if name == "" {
		name = "exporter"
	}
	return path.Join("/etc/prometheus", "prometheus-"+name+"-exporter.yml")
}

func LoadFeatureConfigFile(featureName string, explicitPath string, target any) (string, bool, error) {
	configPath := strings.TrimSpace(explicitPath)
	required := configPath != ""
	if configPath == "" {
		configPath = DefaultFeatureConfigFile(featureName)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if !required && errors.Is(err, os.ErrNotExist) {
			return configPath, false, nil
		}
		return configPath, false, fmt.Errorf("read %s config file %q: %w", featureName, configPath, err)
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(target); err != nil {
		if errors.Is(err, io.EOF) {
			return configPath, true, nil
		}
		return configPath, false, fmt.Errorf("parse %s config file %q: %w", featureName, configPath, err)
	}
	return configPath, true, nil
}

func ResolveFeatureConfig[C any](featureName string, config C, configFileFunc FeatureConfigFileFunc[C], resolveFunc FeatureConfigResolver[C]) (C, string, bool, error) {
	if resolveFunc != nil {
		return resolveFunc(featureName, config)
	}
	if configFileFunc == nil {
		return config, "", false, nil
	}
	configFile := featureConfigFile(config, configFileFunc)
	if configFile == "" {
		configFile = DefaultFeatureConfigFile(featureName)
	}
	return config, configFile, false, nil
}

func defaultFeatureConfig[C any](defaultConfigFunc func() C) C {
	if defaultConfigFunc != nil {
		return defaultConfigFunc()
	}
	var config C
	return config
}

func snapshotExtensionRegisterFlags[C any, S any](extension SnapshotFeatureExtension[C, S]) func(*kingpin.Application, FlagContext, *C) {
	return func(app *kingpin.Application, ctx FlagContext, config *C) {
		if extension.ConfigFileFunc != nil {
			if configFile := extension.ConfigFileFunc(config); configFile != nil {
				app.Flag(
					ctx.FeatureName+".config-file",
					"YAML config file. If unset, "+DefaultFeatureConfigFile(ctx.FeatureName)+" is used when it exists",
				).StringVar(configFile)
			}
		}
		if len(extension.ConfigFlagSpecs) > 0 {
			RegisterFeatureConfigFlagSpecs(app, ctx, config, extension.ConfigFlagSpecs)
		}
		if extension.RegisterFlagsFunc != nil {
			extension.RegisterFlagsFunc(app, ctx, config)
		}
	}
}

func (s *snapshotExtensionConfigState[C, S]) runtimeConfigFunc() func(RuntimeConfigContext[C]) []any {
	return func(ctx RuntimeConfigContext[C]) []any {
		values := make([]any, 0, 4)
		if s.extension.ConfigFileFunc != nil {
			configFile, loaded, prepared := s.configSnapshot()
			if !prepared {
				configFile = featureConfigFile(ctx.Config, s.extension.ConfigFileFunc)
				if configFile == "" {
					configFile = DefaultFeatureConfigFile(ctx.FeatureName)
				}
			}
			values = append(values,
				"config_file", configFile,
				"config_file_loaded", loaded,
			)
		}
		if s.extension.RuntimeConfigFunc != nil {
			values = append(values, s.extension.RuntimeConfigFunc(ctx, ctx.Config)...)
		}
		return values
	}
}

func (s *snapshotExtensionConfigState[C, S]) configSnapshot() (string, bool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.configFile, s.loaded, s.prepared
}

func featureConfigFile[C any](config C, configFileFunc FeatureConfigFileFunc[C]) string {
	if configFileFunc == nil {
		return ""
	}
	configFile := configFileFunc(&config)
	if configFile == nil {
		return ""
	}
	return strings.TrimSpace(*configFile)
}
