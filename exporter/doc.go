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
// concrete repositories.
// SnapshotCollector is available for features that need a background refresh
// worker, cached scrape-time snapshots, and common collection health metrics.
// The exporter/featurekit subpackage provides typed lifecycle helpers for
// generated exporters that want to avoid copying feature and collector
// boilerplate in each concrete repository.
// FileScrapeMetrics is available for file-backed scrape-time collectors that
// share mtime, scrape duration, and read or parse error counters.
//
// For programmatic embedding, Run and NewServer construct the same registry and
// HTTP stack without using process arguments. NewServerChecked and
// NewHandlerChecked return validation errors for invalid metrics paths. NewHandler
// is a lower-level constructor for focused tests or custom embedding; callers
// using it directly are responsible for passing valid handler options.
package exporter
