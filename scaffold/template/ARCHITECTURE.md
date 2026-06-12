# Architecture

`__PROJECT_NAME__` is a thin concrete exporter built on `prometheus-exporter-framework`.

## Package Layout

- `cmd`
  Minimal process entrypoint. The generated entrypoint file is
  `scaffold_main.go` and should stay scaffold-owned.
- `internal/exporter`
  Thin adapter that asks the feature package for a contract-backed feature and
  delegates bootstrap metadata to the framework. Files named `scaffold_*.go`
  are fully scaffold-owned.
- `internal/__FEATURE_NAME__`
  Concrete feature package. `scaffold_feature.go` owns the scaffold-compatible
  `featurekit.SnapshotFeatureExtension` assembly and wires config-file flags,
  feature config flag specs, runtime config, collector construction, metrics,
  snapshot status, and smoke behavior through feature-specific hooks.
  `snapshot_types.go` is a generated starter struct wrapping
  `internal/__FEATURE_NAME__check.Snapshot`, but it is feature-owned after
  generation so a concrete exporter can replace it with an aggregate snapshot.
  Feature-specific defaults and hook functions live in adjacent feature files:
  `feature_config_ext.go`, `feature_metrics_ext.go`,
  `feature_snapshotter_ext.go`, and `feature_smoke_ext.go`.
  `scaffold_feature_test_suite_test.go` owns the thin scaffold bridge into
  framework `exporter/exportertest/featuretest`. Register feature-specific
  test cases from `feature_test_suite_ext_test.go` instead of editing
  scaffold-owned tests.
- `smoke`
  Binary smoke tests that build the real executable and verify CLI, HTTP, and
  metric behavior. The scaffold-owned smoke test is `scaffold_binary_test.go`.

Concrete exporter logic belongs in non-`scaffold_*.go` files and domain check
packages. Treat `scaffold_*.go` files as generated contract glue and update
them through the scaffold sync flow only.

## Data Flow

1. `cmd/scaffold_main.go` delegates to `internal/exporter.Main()`, which runs `framework.MainFromInjectedProject(...)`.
2. `internal/exporter` creates the concrete feature through
   `internal/__FEATURE_NAME__.NewFeature(...)` and framework-injected feature
   metadata.
3. Framework `featurekit.Feature` registers common flags such as `--__FEATURE_NAME__.refresh-interval` and `--__FEATURE_NAME__.config-file`, then delegates feature-specific flag specs and behavior through the framework-owned feature contract.
4. Framework `featurekit.Feature` builds a typed snapshotter and collector from the extension-backed spec, then registers and starts the collector.
5. `framework.SnapshotCollector` refreshes data in a background worker every `--__FEATURE_NAME__.refresh-interval`; scrapes read the latest completed snapshot.
6. The collector exports domain metrics and collection health metrics.

## Failure Semantics

If refresh fails before the first successful snapshot, the exporter exposes collection health metrics, but no business metrics.

If the latest refresh fails, the exporter exposes collection health metrics, but no business metrics, and sets:

- `__METRIC_NAMESPACE___last_collection_success = 0`

The `/healthz` endpoint remains `200 OK` while the process is alive even if the latest collection failed.
