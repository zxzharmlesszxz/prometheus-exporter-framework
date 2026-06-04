package exporter

import "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/internal/app"

func HydrateVersionMetadata() {
	app.HydrateVersionMetadata()
}

func ResolveVersionMetadata(currentVersion string, currentBranch string, currentRevision string, computedRevision string, buildMainVersion string, buildBranch string, buildRevision string) (string, string, string) {
	return app.ResolveVersionMetadata(currentVersion, currentBranch, currentRevision, computedRevision, buildMainVersion, buildBranch, buildRevision)
}
