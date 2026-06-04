package exporter

import "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/internal/app"

type ProjectMetadata = app.ProjectMetadata
type ExporterInfo = app.ExporterInfo
type MetricInfo = app.MetricInfo
type SmokeInfo = app.SmokeInfo

func ExporterInfoFromProjectMetadata(metadata ProjectMetadata, features ...Feature) ExporterInfo {
	return app.ExporterInfoFromProjectMetadata(metadata, features...)
}

func StandardMetricInfo(namespace string) MetricInfo {
	return app.StandardMetricInfo(namespace)
}
