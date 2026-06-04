package exporter

import (
	"os"
	"path/filepath"
	"strings"
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
	return requireInjectedDefault("injectedExporterName", injectedExporterName)
}

func InjectedExporterDescription() string {
	return requireInjectedDefault("injectedExporterDescription", injectedExporterDescription)
}

func InjectedFeatureName() string {
	return requireInjectedDefault("injectedFeatureName", injectedFeatureName)
}

func InjectedMetricNamespace() string {
	return requireInjectedDefault("injectedMetricNamespace", injectedMetricNamespace)
}

func InjectedDefaultListenAddress() string {
	listenAddress := requireInjectedDefault("injectedListenAddress", injectedListenAddress)
	requireListenAddress(listenAddress)

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
	cfg.Name = executableName(os.Args, cfg.Name)

	Main(cfg)
}

func ExporterInfoFromInjectedProject(features ...Feature) ExporterInfo {
	return ExporterInfoFromProjectMetadata(InjectedProjectMetadata(), features...)
}

func executableName(args []string, fallback string) string {
	if len(args) == 0 {
		return fallback
	}

	name := filepath.Base(args[0])
	if name == "." || strings.TrimSpace(name) == "" {
		return fallback
	}

	return name
}

func requireInjectedDefault(name string, value string) string {
	if strings.TrimSpace(value) == "" {
		panic("missing Makefile-injected exporter metadata: " + name)
	}

	return value
}

func requireListenAddress(value string) {
	if !strings.HasPrefix(value, ":") {
		panic("invalid Makefile-injected exporter metadata: default listen address must start with :")
	}
}
