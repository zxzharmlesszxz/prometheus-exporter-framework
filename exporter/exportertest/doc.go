// Package exportertest provides test helpers for concrete exporters built on
// prometheus-exporter-framework.
//
// It centralizes common Prometheus registry/gather boilerplate, metric lookup,
// metric presence/value/label assertions, histogram lookup, runtime config
// assertions, and polling for metrics updated by background refresh loops.
// Concrete exporter repositories should prefer these helpers over local copies
// so framework and scaffold tests keep the same failure messages and semantics.
package exportertest
