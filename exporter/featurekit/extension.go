package featurekit

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alecthomas/kingpin/v2"
	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
	"go.yaml.in/yaml/v2"
)

type FeatureConfigFileFunc[C any] func(config *C) *string

type FeatureConfigResolver[C any] func(featureName string, config C) (C, string, bool, error)

type SnapshotFeatureExtension[C any, S any] struct {
	DefaultRefreshInterval time.Duration
	DefaultConfigFunc      func() C
	ConfigFileFunc         FeatureConfigFileFunc[C]
	ConfigFlagSpecs        []FeatureConfigFlagSpec[C]
	RegisterFlagsFunc      func(app *kingpin.Application, ctx FlagContext, config *C)
	ValidateConfigFunc     func(config C) error
	ResolveConfigFunc      FeatureConfigResolver[C]
	RuntimeConfigFunc      func(ctx RuntimeConfigContext[C], config C) []any
	NewSnapshotEngineFunc  SnapshotEngineFunc[C, S]
	DefaultSnapshotEngine  SnapshotEngine[S]
	NewSnapshotterFunc     func(ctx CollectorContext[C]) (framework.Snapshotter[S], error)
	DefaultSnapshotter     framework.Snapshotter[S]
	MetricSpecs            []FeatureMetricSpec
	MetricHandlers         FeatureMetricHandlers[S]
	MetricsFunc            SnapshotMetricsFunc[S]
	StatusFunc             func(S) framework.SnapshotStatus
	ErrorLogFunc           func(*slog.Logger, S)
	Smoke                  SmokeSpec
	SmokeFunc              func(ctx SmokeContext[C]) SmokeSpec
}

func NewSnapshotExtensionFeatureSpec[C any, S any](options SpecOptions, extension SnapshotFeatureExtension[C, S]) FeatureSpec[C, S] {
	return NewSnapshotFeatureSpec(SnapshotFeatureSpec[C, S]{
		Options:                options,
		DefaultRefreshInterval: extension.DefaultRefreshInterval,
		Config:                 defaultFeatureConfig(extension.DefaultConfigFunc),
		RegisterFlagsFunc:      snapshotExtensionRegisterFlags(extension),
		ValidateConfigFunc:     extension.ValidateConfigFunc,
		NewSnapshotterFunc:     snapshotExtensionNewSnapshotter(extension),
		DefaultSnapshotter:     snapshotExtensionDefaultSnapshotter(extension),
		MetricsFunc:            snapshotExtensionMetricsFunc(extension),
		StatusFunc:             extension.StatusFunc,
		ErrorLogFunc:           extension.ErrorLogFunc,
		RuntimeConfigFunc:      snapshotExtensionRuntimeConfig(extension),
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
	if extension.NewSnapshotEngineFunc == nil {
		return nil
	}
	return func(ctx CollectorContext[C]) (framework.Snapshotter[S], error) {
		return extension.NewSnapshotEngineFunc(ctx)
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

func DefaultFeatureConfigFile(featureName string) string {
	name := strings.TrimSpace(featureName)
	if name == "" {
		name = "exporter"
	}
	return filepath.Join("/etc/prometheus", "prometheus-"+name+"-exporter.yml")
}

func LoadFeatureConfigFile(featureName string, explicitPath string, target any) (string, bool, error) {
	path := strings.TrimSpace(explicitPath)
	required := path != ""
	if path == "" {
		path = DefaultFeatureConfigFile(featureName)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if !required && errors.Is(err, os.ErrNotExist) {
			return path, false, nil
		}
		return path, false, fmt.Errorf("read %s config file %q: %w", featureName, path, err)
	}
	if err := yaml.UnmarshalStrict(data, target); err != nil {
		return path, false, fmt.Errorf("parse %s config file %q: %w", featureName, path, err)
	}
	return path, true, nil
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

func snapshotExtensionRuntimeConfig[C any, S any](extension SnapshotFeatureExtension[C, S]) func(RuntimeConfigContext[C]) []any {
	return func(ctx RuntimeConfigContext[C]) []any {
		config, configFile, loaded, _ := ResolveFeatureConfig(ctx.FeatureName, ctx.Config, extension.ConfigFileFunc, extension.ResolveConfigFunc)
		values := make([]any, 0, 4)
		if extension.ConfigFileFunc != nil {
			values = append(values,
				"config_file", configFile,
				"config_file_loaded", loaded,
			)
		}
		if extension.RuntimeConfigFunc != nil {
			values = append(values, extension.RuntimeConfigFunc(ctx, config)...)
		}
		return values
	}
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
