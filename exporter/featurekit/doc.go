// Package featurekit provides typed helpers for generated exporter features.
//
// It sits one layer above exporter.Feature: concrete exporters provide a typed
// FeatureSpec with their domain config, snapshot type, snapshotter factory, and
// collector factory; Feature handles the common flag, runtime config, collector
// registration, smoke-test metadata, and collector startup lifecycle.
//
// FeatureContract and FeatureDefaults provide the stable contract shape for
// generated exporters. Concrete feature packages embed the defaults and override
// feature-specific behavior in their own files, while the framework keeps the
// standard method set, spec wiring, feature construction, snapshot engine
// wiring, config flag spec registration, and metric descriptor loading reusable.
// Runtime config includes config_error when config preparation fails, so
// startup logs surface bad YAML or config-file resolution errors without
// changing the stable RuntimeConfig []any API.
// RuntimeConfig and RegisterCollectors share the same prepared config cache;
// callers should register and parse flags before either method is called.
// DefaultFeatureConfigFile, LoadFeatureConfigFile, ResolveFeatureConfig, and
// FeatureConfigFlagSpec cover the common config-file plus CLI merge flow used
// by scaffolded exporters.
// Snapshot specs can set SyncRefreshTimeout to bound scrape-triggered
// synchronous refreshes when no fresh background snapshot exists; leaving it
// zero preserves the core collector's historical unbounded behavior.
// Framework-owned collection metrics should use the exporter metric namespace;
// domain metrics may use absolute rendered names when a scaffolded exporter
// wants a domain-specific feature namespace distinct from the Go feature name.
// MetricScope, FeatureMetricSpec, FeatureMetricDescriptors, and
// LoadFeatureMetricDescriptors provide reusable descriptor naming and lookup.
// FileScrapeMetricSpecs provides the standard source-health metric contract for
// file-backed domain sources; combine it with exporter.FileScraper and
// exporter.FileScrapeMetrics in concrete feature code.
package featurekit
