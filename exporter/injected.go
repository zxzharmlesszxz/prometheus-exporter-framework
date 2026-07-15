package exporter

import (
	"fmt"
	"os"

	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/internal/app"
)

// Injected metadata is a build-time contract.
// These values must be provided by generated exporter Makefiles via -ldflags.
// Missing values mean the binary was built incorrectly, so the public injected
// helpers intentionally fail fast instead of falling back to implicit defaults.
var (
	injectedExporterName        string
	injectedExporterDescription string
	injectedFeatureName         string
	injectedMetricNamespace     string
	injectedListenAddress       string
)

func InjectedExporterName() string {
	return app.RequireInjectedDefault("injectedExporterName", injectedExporterName)
}

func InjectedExporterDescription() string {
	return app.RequireInjectedDefault("injectedExporterDescription", injectedExporterDescription)
}

func InjectedFeatureName() string {
	return app.RequireInjectedDefault("injectedFeatureName", injectedFeatureName)
}

func InjectedMetricNamespace() string {
	namespace := app.RequireInjectedDefault("injectedMetricNamespace", injectedMetricNamespace)
	app.RequireMetricNamespace(namespace)

	return namespace
}

func InjectedDefaultListenAddress() string {
	listenAddress := app.RequireInjectedDefault("injectedListenAddress", injectedListenAddress)
	app.RequireListenAddress(listenAddress)

	return listenAddress
}

func InjectedProjectMetadata() ProjectMetadata {
	metadata, err := InjectedProjectMetadataErr()
	if err != nil {
		panic(err.Error())
	}
	return metadata
}

func InjectedProjectMetadataErr() (ProjectMetadata, error) {
	exporterName, err := app.InjectedDefault("injectedExporterName", injectedExporterName)
	if err != nil {
		return ProjectMetadata{}, err
	}
	description, err := app.InjectedDefault("injectedExporterDescription", injectedExporterDescription)
	if err != nil {
		return ProjectMetadata{}, err
	}
	featureName, err := app.InjectedDefault("injectedFeatureName", injectedFeatureName)
	if err != nil {
		return ProjectMetadata{}, err
	}
	metricNamespace, err := app.InjectedDefault("injectedMetricNamespace", injectedMetricNamespace)
	if err != nil {
		return ProjectMetadata{}, err
	}
	listenAddress, err := app.InjectedDefault("injectedListenAddress", injectedListenAddress)
	if err != nil {
		return ProjectMetadata{}, err
	}
	metadata := ProjectMetadata{
		ExporterName:         exporterName,
		ExporterDescription:  description,
		FeatureName:          featureName,
		MetricNamespace:      metricNamespace,
		DefaultListenAddress: listenAddress,
	}
	if err := metadata.Validate(); err != nil {
		return ProjectMetadata{}, fmt.Errorf("invalid Makefile-injected exporter metadata: %w", err)
	}
	return metadata, nil
}

func ConfigFromInjectedProject(features ...Feature) Config {
	cfg, err := ConfigFromInjectedProjectErr(features...)
	if err != nil {
		panic(err.Error())
	}
	return cfg
}

func ConfigFromInjectedProjectErr(features ...Feature) (Config, error) {
	metadata, err := InjectedProjectMetadataErr()
	if err != nil {
		return Config{}, err
	}
	return Config{
		Name:                 metadata.ExporterName,
		Namespace:            metadata.MetricNamespace,
		Description:          metadata.ExporterDescription,
		DefaultListenAddress: metadata.DefaultListenAddress,
		Features:             features,
	}, nil
}

func MainFromInjectedProject(features ...Feature) {
	if err := MainFromInjectedProjectErr(features...); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func MainFromInjectedProjectErr(features ...Feature) error {
	cfg, err := ConfigFromInjectedProjectErr(features...)
	if err != nil {
		return err
	}
	cfg.Name = app.ExecutableName(os.Args, cfg.Name)

	return app.Main(cfg)
}

func ExporterInfoFromInjectedProject(features ...Feature) ExporterInfo {
	info, err := ExporterInfoFromInjectedProjectErr(features...)
	if err != nil {
		panic(err.Error())
	}
	return info
}

func ExporterInfoFromInjectedProjectErr(features ...Feature) (ExporterInfo, error) {
	metadata, err := InjectedProjectMetadataErr()
	if err != nil {
		return ExporterInfo{}, err
	}
	return ExporterInfoFromProjectMetadataErr(metadata, features...)
}
