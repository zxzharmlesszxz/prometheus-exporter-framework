// Package featurekit provides typed helpers for generated exporter features.
//
// It sits one layer above exporter.Feature: concrete exporters provide a typed
// FeatureSpec with their domain config, snapshot type, snapshotter factory, and
// collector factory; Feature handles the common flag, runtime config, collector
// registration, and collector startup lifecycle.
package featurekit
