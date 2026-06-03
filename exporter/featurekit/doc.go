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
// standard method set, spec wiring, feature construction, config flag spec
// registration, and metric descriptor loading reusable.
package featurekit
