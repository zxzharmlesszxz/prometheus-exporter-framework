package __FEATURE_NAME__

import "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit"

const metricExampleValue = "example_value"

var featureMetricSpecs = []featurekit.FeatureMetricSpec{
	{
		ID:    metricExampleValue,
		Scope: featurekit.MetricScopeFeature,
		Name:  "_example_value",
		Help:  "Example __FEATURE_NAME__ metric emitted by the generated exporter skeleton",
	},
}
