package exporter

import (
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
	return app.RequireInjectedDefault("injectedMetricNamespace", injectedMetricNamespace)
}

func InjectedDefaultListenAddress() string {
	listenAddress := app.RequireInjectedDefault("injectedListenAddress", injectedListenAddress)
	app.RequireListenAddress(listenAddress)

	return listenAddress
}

func InjectedProjectMetadata() ProjectMetadata {
	return ProjectMetadata{
		ExporterName:         InjectedExporterName(),
		ExporterDescription:  InjectedExporterDescription(),
		FeatureName:          InjectedFeatureName(),
		MetricNamespace:      InjectedMetricNamespace(),
		DefaultListenAddress: InjectedDefaultListenAddress(),
	}
}

func ConfigFromInjectedProject(features ...Feature) Config {
	metadata := InjectedProjectMetadata()

	return Config{
		Name:                 metadata.ExporterName,
		Namespace:            metadata.MetricNamespace,
		Description:          metadata.ExporterDescription,
		DefaultListenAddress: metadata.DefaultListenAddress,
		Features:             features,
	}
}

func MainFromInjectedProject(features ...Feature) {
	cfg := ConfigFromInjectedProject(features...)
	cfg.Name = app.ExecutableName(os.Args, cfg.Name)

	Main(cfg)
}

func ExporterInfoFromInjectedProject(features ...Feature) ExporterInfo {
	return ExporterInfoFromProjectMetadata(InjectedProjectMetadata(), features...)
}
