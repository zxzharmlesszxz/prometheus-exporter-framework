// Package smoketest provides reusable binary smoke tests for exporters built on
// prometheus-exporter-framework.
//
// The runner builds or uses a prebuilt exporter binary, verifies version/help
// output, starts the server when metric expectations are configured, and checks
// health and metrics endpoints. TelemetryPath and HealthPath are HTTP paths and
// must start with "/".
package smoketest
