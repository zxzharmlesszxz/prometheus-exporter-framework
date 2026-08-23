// Package exporter provides the reusable shell for concrete Prometheus
// exporters.
//
// The package owns common exporter behavior: command-line bootstrap, standard
// web and logging flags, exporter-toolkit HTTP serving, health and metrics
// handlers, Prometheus registry wiring, standard Go/process/build collectors,
// optional typed snapshot refresh helpers, and version metadata hydration.
// Small metric value helpers and exporter-focused test helpers live here too,
// so concrete exporters do not need to copy boilerplate for common assertions
// or timestamp/boolean gauge conversion.
//
// API policy:
//   - This package is the stable public facade for generated exporters.
//   - Exported identifiers are intended for use by external exporter repositories.
//   - Add new exports only when they are expected to remain stable.
//   - Keep experimental helpers in internal packages or explicitly unstable
//     subpackages.
//
// Concrete exporters provide domain behavior by implementing Feature and
// passing one or more features to MainFromProject, MainFromInjectedProject,
// MainForProject, Main, RunCLIFromProject, or RunCLI. A feature registers its
// own flags and collectors; optional interfaces add feature names to logs,
// report runtime configuration fields, provide binary smoke metadata, or
// override the default listen address. Generated exporters usually use
// Makefile-injected project metadata through ConfigFromInjectedProject and
// ExporterInfoFromInjectedProject, which keeps project bootstrap code out of
// concrete repositories. Error-returning variants such as
// ConfigFromInjectedProjectErr, InjectedProjectMetadataErr,
// MainFromInjectedProjectErr, ExporterInfoFromInjectedProjectErr, and
// ExporterInfoFromProjectMetadataErr are available for bootstrap paths that
// need controlled error handling instead of panic/exit behavior. The matching
// helpers without an Err suffix are fail-fast convenience APIs for generated
// main packages and test bootstrap code; they panic or exit when injected
// metadata is missing or invalid.
// SnapshotCollector is available for features that need a background refresh
// worker, cached scrape-time snapshots, and common collection health metrics.
// Its SyncRefreshTimeout option bounds only scrape-triggered synchronous
// refreshes; leaving it zero keeps those refreshes unbounded for compatibility
// with slow exporters.
// The exporter/featurekit subpackage provides typed lifecycle helpers for
// generated exporters that want to avoid copying feature and collector
// boilerplate in each concrete repository.
// The exporter/exportertest/featuretest subpackage provides the matching
// reusable test suite for scaffolded feature packages.
// FileScraper and FileScrapeMetrics are available for file-backed scrape-time
// collectors that share mtime, up, valid, scrape duration, and read or parse
// error metrics. FileScraper counters are cumulative for the counter instance;
// use separate counter instances per source when a collector exports per-source
// read or parse error totals.
// Use exporter/featurekit.FileScrapeMetricSpecs to keep source-health metric
// names and descriptors consistent across scaffolded exporters.
//
// For programmatic embedding, Run and NewServer construct the same registry and
// HTTP stack without using process arguments. RunContext and NewRegistryContext
// let embedding callers pass a lifecycle context to startable collectors.
// RunCLIContext does the same for CLI-driven startup. NewServerChecked and
// NewHandlerChecked return validation errors for invalid metrics paths; NewHandler
// returns an HTTP error handler when validation fails.
package exporter
