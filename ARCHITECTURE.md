# Architecture

## Overview

`prometheus-exporter-framework` is a reusable exporter shell.

It intentionally does not know about any business domain such as Debian packages, Puppet agent files, or Puppetfile parsing.
Domain logic is supplied by external `exporter.Feature` implementations.

## Package Layout

- `cmd`
  Minimal binary that runs the shell without domain features.
- `exporter`
  Public framework facade used by concrete exporters. It preserves the stable
  `exporter.X` API while delegating implementation to focused internal packages.
- `exporter/internal/app`
  CLI bootstrap, config derivation, HTTP server wiring, registry setup,
  version hydration, and injected project metadata.
- `exporter/internal/feature`
  Feature contracts plus collector registration and startup helpers.
- `exporter/internal/snapshot`
  Typed snapshot cache and background refresh collector.
- `exporter/internal/files`
  File mtime and file scrape metric helpers.
- `exporter/internal/metric`
  Small metric value helpers.
- `exporter/featurekit`
  Typed generated-feature lifecycle helpers used by scaffolded exporters.
- `exporter/exportertest`
  Test helpers and downstream feature contract checks.
- `exporter/exportertest/featuretest`
  Standard feature test suite for scaffolded `featurekit` snapshot exporters.

## Data Flow

1. A concrete exporter calls `exporter.MainFromProject(features...)`, `exporter.MainForProject(projectName, description, features...)`, or `exporter.Main(exporter.Config{...})` when it needs explicit metadata overrides.
2. The framework registers common CLI flags and asks each feature to register its own flags.
3. After parsing, the framework creates the logger, runtime options, and Prometheus registry.
4. The registry always receives:
   - `*_build_info`
   - Go runtime collectors
   - process collectors
5. Each feature registers its own collectors directly or wraps domain collection in `SnapshotCollector`.
6. The HTTP server exposes the registry on the configured telemetry path.

## Extension Contract

Features implement:

```go
type Feature interface {
	RegisterFlags(app *kingpin.Application)
	RegisterCollectors(ctx FeatureContext, registry *prometheus.Registry) error
}
```

Optional interfaces:

- `NamedFeature`
  Adds a feature name to structured logs and registration errors.
- `RuntimeConfigReporter`
  Adds feature-specific fields to the startup runtime config log.
- `DefaultListenAddressProvider`
  Provides the feature's default `--web.listen-address` value.

The helper `CollectorFeature` can be used when a feature only needs callbacks instead of a dedicated type.
The helper `SnapshotCollector` can be used when a feature needs a typed snapshot cache, background refresh loop, and common collection health metrics.
The `exporter/featurekit` subpackage can be used by scaffolded exporters that want a typed feature spec instead of hand-written flag, runtime-config, collector-registration, and collector-startup boilerplate.
The package also exposes small value/lifecycle helpers (`BoolFloat`,
`UnixTimestamp`, `FileMTimeSeconds`, `FileScraper`, `FileScrapeMetrics`,
`FileScrapeResult`, `NormalizeDuration`, and `RegisterAndStartCollectors`) plus
the `exporter/exportertest` and `exporter/exportertest/featuretest` packages for
shared exporter test assertions and scaffolded feature test suites.

## Common HTTP Semantics

- The configured telemetry path exposes the Prometheus registry.
- The default telemetry path is `/metrics`.
- `/` serves the exporter-toolkit landing page when the telemetry path is not `/`.
- When the telemetry path is `/`, `/` serves metrics and no landing page is registered.
- `/healthz` returns `200 OK` with `ok\n` while the process is serving requests.
- `/debug/pprof/*` is disabled unless `--web.enable-pprof` is set.
- The landing page links to metrics and health endpoints.

The telemetry path must be a literal URL path that starts with `/`.
It must not include whitespace, query strings, fragments, or Go `http.ServeMux` wildcards.
`/healthz` and `/debug/pprof/*` are reserved for built-in handlers and cannot be used as telemetry paths.

`/healthz` reflects process health only.
Domain-source health belongs in feature collectors.

## Public API Stability

The public extension surface is:

- `Main`, `MainErr`, `MainFromProject`, `MainFromProjectErr`,
  `MainForProject`, `MainForProjectErr`, `MainFromInjectedProject`, and
  `MainFromInjectedProjectErr`
- `RunCLI`, `RunCLIContext`, and `RunCLIFromProject`
- `Config`, `ConfigFromProject`, `ConfigForProject`,
  `ConfigFromInjectedProject`, and `ConfigFromInjectedProjectErr`
- `Options`, `Run`, `RunContext`, `MustRun`, `NewServer`, and
  `NewServerChecked`
- `HandlerOptions`, `NewHandler`, and `NewHandlerChecked`
- `Feature`, `FeatureContext`, `CollectorFeature`, and `StartableCollector`
- `Snapshotter`, `SnapshotStatus`, `SnapshotCollectorOptions`,
  `SnapshotCollector`, `NewSnapshotCollector`, and
  `DefaultSnapshotRefreshInterval`
- `BoolFloat`, `UnixTimestamp`, `FileMTimeSeconds`, `FileScraper`,
  `FileScrapeMetrics`, `FileScrapeResult`, `FileReadFunc`, `Uint64Counter`,
  and `NormalizeDuration`
- `NamedFeature`, `RuntimeConfigReporter`, and `DefaultListenAddressProvider`
- `SmokeSpecProvider` and `SmokeSpec`
- `NewRegistry`, `NewRegistryContext`, `RegisterCollectors`, and
  `RegisterAndStartCollectors`
- `InjectedProjectMetadata`, `InjectedProjectMetadataErr`,
  `InjectedExporterName`, `InjectedExporterDescription`,
  `InjectedFeatureName`, `InjectedMetricNamespace`, and
  `InjectedDefaultListenAddress`
- `ExporterInfoFromProjectMetadata`, `ExporterInfoFromProjectMetadataErr`,
  `ExporterInfoFromInjectedProject`, `ExporterInfoFromInjectedProjectErr`,
  `StandardMetricInfo`, `ProjectMetadata`, `ExporterInfo`, `MetricInfo`, and
  `SmokeInfo`
- `ExporterNameFromProject` and `DescriptionFromProject`
- `HydrateVersionMetadata` and `ResolveVersionMetadata`

The `exporter/featurekit` subpackage is public support for generated exporters
and exposes `FeatureSpec`, `Feature`, `FeatureDefaults`, `FeatureContract`,
`SmokeSpec`, `SmokeContext`, `SnapshotCollectorOptions`,
`ResolveSnapshotCollectorOptions`, `NewSnapshotCollector`, config-file and
config-flag helpers, snapshot extension helpers, feature metric spec helpers,
file scrape metric helpers, and the exported methods on those types.
Public API golden checks also track exported fields and interface methods for
types defined directly in public packages, plus type alias targets and exported
members exposed through root facade aliases to local internal types. The shared
AST walker lives in `exporter/internal/publicapitest`; package-level
`public_api_test.go` files should stay thin wrappers around it.

The `exporter/exportertest` subpackage is public test support for downstream
exporters. Its `adaptertest`, `featuretest`, and `smoketest` subpackages are
also public because scaffolded exporters import them directly. The
`exporter/exportertest/featuretest` subpackage owns the reusable
`FeatureTestSuite` for scaffolded feature packages; concrete exporters pass
feature-specific snapshot/config/metric hooks into that suite and register only
domain-specific test cases locally.

`NewHandler` is a lower-level constructor for embedding or focused tests.
Production entrypoints should prefer `RunCLI`, `Run`, or `NewServerChecked`, which apply option normalization and telemetry-path validation before constructing handlers.
`NewServer` keeps its original no-error signature and normalizes options, but callers that need explicit errors instead of `http.ServeMux` panics should use `NewHandlerChecked` or `NewServerChecked`.
Bootstrap helpers without an `Err` suffix preserve legacy panic or exit
behavior where applicable. New integration code that needs controlled failure
handling should prefer the matching `...Err` or context-aware variant.
Smoke-test flag overrides belong in `SmokeSpec.ServerArgs`; those arguments are
test inputs for binary smoke checks and must not be treated as framework
runtime defaults.

Concrete exporters should treat unexported functions and types as internal implementation details.
Breaking changes to the exported extension surface should be released with a major version bump.

## Metric Ownership

The framework owns only common exporter metrics:

- `*_build_info`
- Go runtime metrics
- process metrics
- `*_last_collection_*` metrics when a feature opts into `SnapshotCollector`

Every business metric and domain diagnostic metric is owned by the concrete feature.
For example:

- Debian package metrics belong to the package feature.
- Puppet agent source-health metrics belong to the Puppet feature.
- Puppetfile module metrics belong to the Puppetfile feature.

## Failure Semantics

The framework fails startup when a feature cannot register its collectors.
Runtime scrape failures are domain-specific and should be represented by feature collectors.
When a feature uses `SnapshotCollector`, the framework stores the latest typed snapshot and emits common collection timestamps and success state from the feature-provided `SnapshotStatus`.

For example, a feature can:

- emit `*_up = 0`
- serve cached metrics after a failed refresh
- omit business metrics when its source is unreadable
- increment read or parse error counters

For file-backed source-health helpers, `*_up` means the source was readable.
Parse or domain-validity failures should keep `*_up = 1` and report invalid data
through a separate `*_valid = 0` metric.

The framework does not impose one policy because the existing exporters use different failure semantics.
