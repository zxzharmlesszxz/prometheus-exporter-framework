package app

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

type ProjectMetadata struct {
	ExporterName         string
	ExporterDescription  string
	FeatureName          string
	MetricNamespace      string
	DefaultListenAddress string
}

type ExporterInfo struct {
	Name                 string
	Description          string
	FeatureName          string
	MetricNamespace      string
	DefaultListenAddress string
	Metrics              MetricInfo
	Smoke                SmokeInfo
}

type MetricInfo struct {
	BuildInfo                                string
	CollectionDurationSeconds                string
	LastCollectionSuccess                    string
	LastCollectionTimestampSeconds           string
	LastSuccessfulCollectionTimestampSeconds string
}

type SmokeInfo struct {
	ForbiddenUsageNames []string
	RenamedExecutable   string
	ServerArgs          []string
	WantMetrics         []string
	RejectMetrics       []string
}

func ExporterInfoFromProjectMetadata(metadata ProjectMetadata, features ...Feature) ExporterInfo {
	info, err := ExporterInfoFromProjectMetadataErr(metadata, features...)
	if err != nil {
		panic(err.Error())
	}
	return info
}

func ExporterInfoFromProjectMetadataErr(metadata ProjectMetadata, features ...Feature) (ExporterInfo, error) {
	if err := metadata.Validate(); err != nil {
		return ExporterInfo{}, err
	}
	metrics := StandardMetricInfo(metadata.MetricNamespace)
	smoke := SmokeInfo{
		ForbiddenUsageNames: []string{metadata.MetricNamespace},
		RenamedExecutable:   "renamed-" + metadata.FeatureName + "-exporter",
		WantMetrics:         []string{metrics.LastCollectionSuccess + " 1"},
		RejectMetrics:       []string{metrics.LastCollectionSuccess + " 0"},
	}
	for _, feature := range features {
		provider, ok := feature.(SmokeSpecProvider)
		if !ok {
			continue
		}
		smoke = appendSmokeSpec(smoke, provider.SmokeSpec())
	}
	return ExporterInfo{
		Name:                 metadata.ExporterName,
		Description:          metadata.ExporterDescription,
		FeatureName:          metadata.FeatureName,
		MetricNamespace:      metadata.MetricNamespace,
		DefaultListenAddress: metadata.DefaultListenAddress,
		Metrics:              metrics,
		Smoke:                smoke,
	}, nil
}

func StandardMetricInfo(namespace string) MetricInfo {
	return MetricInfo{
		BuildInfo:                                namespace + "_build_info",
		CollectionDurationSeconds:                namespace + "_collection_duration_seconds",
		LastCollectionSuccess:                    namespace + "_last_collection_success",
		LastCollectionTimestampSeconds:           namespace + "_last_collection_timestamp_seconds",
		LastSuccessfulCollectionTimestampSeconds: namespace + "_last_successful_collection_timestamp_seconds",
	}
}

func appendSmokeSpec(info SmokeInfo, spec SmokeSpec) SmokeInfo {
	info.ServerArgs = append(info.ServerArgs, spec.ServerArgs...)
	info.WantMetrics = append(info.WantMetrics, spec.WantMetrics...)
	info.RejectMetrics = append(info.RejectMetrics, spec.RejectMetrics...)
	return info
}

func (m ProjectMetadata) Validate() error {
	if _, err := InjectedDefault("ProjectMetadata.ExporterName", m.ExporterName); err != nil {
		return err
	}
	if _, err := InjectedDefault("ProjectMetadata.ExporterDescription", m.ExporterDescription); err != nil {
		return err
	}
	if _, err := InjectedDefault("ProjectMetadata.FeatureName", m.FeatureName); err != nil {
		return err
	}
	if _, err := InjectedDefault("ProjectMetadata.MetricNamespace", m.MetricNamespace); err != nil {
		return err
	}
	if err := validateMetricNamespace(m.MetricNamespace); err != nil {
		return fmt.Errorf("invalid metric namespace %q: %w", m.MetricNamespace, err)
	}
	if _, err := InjectedDefault("ProjectMetadata.DefaultListenAddress", m.DefaultListenAddress); err != nil {
		return err
	}
	return ValidateListenAddress(m.DefaultListenAddress)
}

func InjectedDefault(name string, value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("missing Makefile-injected exporter metadata: %s", name)
	}
	return value, nil
}

func RequireInjectedDefault(name string, value string) string {
	resolved, err := InjectedDefault(name, value)
	if err != nil {
		panic(err.Error())
	}
	return resolved
}

func ValidateListenAddress(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("default listen address is empty")
	}
	if strings.ContainsAny(value, " \t\r\n") {
		return errors.New("default listen address must not contain whitespace")
	}
	_, port, err := net.SplitHostPort(value)
	if err != nil || port == "" {
		return fmt.Errorf("default listen address must be :port or host:port")
	}
	return nil
}

func RequireListenAddress(value string) {
	if err := ValidateListenAddress(value); err != nil {
		panic("invalid Makefile-injected exporter metadata: " + err.Error())
	}
}

func RequireMetricNamespace(value string) {
	if err := validateMetricNamespace(value); err != nil {
		panic("invalid Makefile-injected exporter metadata: " + err.Error())
	}
}
