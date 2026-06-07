package app

import "strings"

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
	metadata.requireValid()
	metrics := StandardMetricInfo(metadata.MetricNamespace)
	smoke := SmokeInfo{
		ForbiddenUsageNames: []string{metadata.MetricNamespace},
		RenamedExecutable:   "renamed-" + metadata.FeatureName + "-exporter",
		ServerArgs:          []string{"--" + metadata.FeatureName + ".refresh-interval=100ms"},
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
	}
}

func StandardMetricInfo(namespace string) MetricInfo {
	return MetricInfo{
		BuildInfo:                                namespace + "_build_info",
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

func (m ProjectMetadata) requireValid() {
	RequireInjectedDefault("ProjectMetadata.ExporterName", m.ExporterName)
	RequireInjectedDefault("ProjectMetadata.ExporterDescription", m.ExporterDescription)
	RequireInjectedDefault("ProjectMetadata.FeatureName", m.FeatureName)
	RequireInjectedDefault("ProjectMetadata.MetricNamespace", m.MetricNamespace)
	RequireInjectedDefault("ProjectMetadata.DefaultListenAddress", m.DefaultListenAddress)
	RequireListenAddress(m.DefaultListenAddress)
}

func RequireInjectedDefault(name string, value string) string {
	if strings.TrimSpace(value) == "" {
		panic("missing Makefile-injected exporter metadata: " + name)
	}
	return value
}

func RequireListenAddress(value string) {
	if !strings.HasPrefix(value, ":") {
		panic("invalid Makefile-injected exporter metadata: default listen address must start with :")
	}
}
