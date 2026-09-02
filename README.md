# prometheus-exporter-framework

Reusable Go framework for Prometheus exporters.

This repository owns the stable exporter shell:

- CLI bootstrap and standard flags
- structured `promslog` logging
- exporter-toolkit HTTP serving
- `/metrics`, `/healthz`, landing page, and optional pprof endpoints
- Prometheus registry wiring
- `build_info`, Go runtime, and process collectors
- optional snapshot cache and background refresh collector helper
- typed generated-feature lifecycle helpers
- small helpers for common metric values and exporter tests
- version metadata hydration from linker flags or Go build info

Concrete exporters add domain behavior through `exporter.Feature` implementations in their own repositories.
Concrete exporter scaffolding now lives in the `scaffold/` directory of this repository.
No framework code in this repository needs to change when a new exporter feature is added.

## Extension Model

A feature owns two things:

- domain-specific flags
- domain-specific Prometheus collectors

Minimal example:

```go
package main

import (
	"fmt"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"

	framework "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
)

type Feature struct {
	inputPath *string
}

func (f *Feature) RegisterFlags(app *kingpin.Application) {
	f.inputPath = app.Flag("input.path", "Path to exporter input").Required().String()
}

func (f *Feature) RegisterCollectors(ctx framework.FeatureContext, registry *prometheus.Registry) error {
	if f.inputPath == nil {
		return fmt.Errorf("input.path flag was not registered")
	}
	collector := NewDomainCollector(ctx.Logger, *f.inputPath)
	return framework.RegisterCollectors(registry, collector)
}

func (f *Feature) RuntimeConfig() []any {
	if f.inputPath == nil {
		return nil
	}
	return []any{"input_path", *f.inputPath}
}

func main() {
	framework.Main(framework.Config{
		Name:                 "demo_exporter",
		Namespace:            "demo_exporter",
		Description:          "Prometheus Demo Exporter",
		DefaultListenAddress: ":9901",
		Features: []framework.Feature{
			&Feature{},
		},
	})
}
```

A compiling example feature is available in `examples/custom-feature`.

## Snapshot Collectors

Features that periodically read external state can use `SnapshotCollector` instead of reimplementing refresh and cache logic.
The feature supplies a typed `Snapshotter`, a small status adapter, and domain metric callbacks:

```go
collector := framework.NewSnapshotCollector(framework.SnapshotCollectorOptions[DomainSnapshot]{
	Namespace:       ctx.Namespace,
	Logger:          ctx.Logger,
	Snapshotter:     domainSnapshotter,
	RefreshInterval: refreshInterval,
	StatusFunc: func(snapshot DomainSnapshot) framework.SnapshotStatus {
		return framework.SnapshotStatus{
			AttemptTime: snapshot.AttemptTime,
			Success:     snapshot.Success,
		}
	},
	DescribeFunc: describeDomainMetrics,
	CollectFunc:  collectDomainMetrics,
})
```

`SnapshotCollector` owns the background refresh worker, scrape-time cache fallback, and common collection metrics.
The concrete exporter still owns the domain snapshot type and all business metrics.
Set `SyncRefreshTimeout` on snapshot/featurekit specs when scrape-triggered
synchronous refreshes must be bounded; leaving it zero preserves the historical
unbounded behavior for compatibility. The option does not wrap background
refreshes; those use the lifecycle context passed to collector startup.

## Generated Feature Helpers

Generated exporters can import `github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/featurekit`.
That subpackage wraps the common generated-exporter pattern: a typed domain config, common refresh flag, runtime config, snapshotter factory, collector factory, smoke-test metadata, and collector startup lifecycle.
If config preparation fails, generated feature runtime config includes
`config_error` so startup logs show the real YAML/config-file failure instead
of only reporting that the config file was not loaded.
Feature config files are decoded with YAML v3 and strict known-field checking:
unknown fields and duplicate keys fail, while empty or `null` files load as a
zero-value config.

Concrete exporters still own their domain flags, snapshot type, metric descriptors, and snapshot-to-metrics adapter.
The scaffold/ directory in this repository owns only the glue that passes a typed `featurekit.FeatureSpec` to the framework.

Generated exporters keep metric ownership split into two namespaces:

- exporter/framework metrics use `METRIC_NAMESPACE`, for example `demo_exporter_last_collection_success`
- domain metrics use `FEATURE_NAMESPACE`, for example `demo_example_value`

`FEATURE_NAME` remains the Go package, config flag, and log-context name. By
default `FEATURE_NAMESPACE` equals `FEATURE_NAME`, but scaffold rendering lets
an exporter choose it explicitly.

Framework snapshot collectors also expose
`<metric_namespace>_collection_duration_seconds`, a histogram of exporter
refresh-loop duration. This is separate from Prometheus scrape duration.

## Utility Helpers

Concrete exporters can reuse small metric helpers instead of carrying local copies:

- `BoolFloat(bool)` for `0`/`1` gauge values
- `UnixTimestamp(time.Time)` for fractional Unix timestamp metrics with zero-time handling
- `FileMTimeSeconds(path)` for fractional file mtime gauges that return `0` when the file cannot be statted
- `FileScrapeMetrics` for file-backed collectors that expose labeled mtime, up, valid, scrape duration, and read/parse metrics
- `featurekit.FileScrapeMetricSpecs(source, labels)` for the matching source-health descriptor contract in scaffolded feature metrics
- `NormalizeDuration(value, fallback)` for duration flags where non-positive values should fall back to defaults
- `RegisterAndStartCollectors(ctx, registry, collectors...)` for collectors with a background `Start(context.Context)` lifecycle

`FileScraper` read/parse counters are cumulative for the counter instance. Use
one counter pair per source when the collector exports per-source error totals;
reusing a single scraper across multiple paths intentionally reports aggregate
totals.

Tests can import `github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/exportertest` for common registry/gather helpers, metric lookup, metric presence/value/label assertions, histogram lookup, and polling metrics that are updated by background refresh loops.
Scaffolded feature tests can additionally import `github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/exportertest/featuretest` for the standard `FeatureTestSuite` contract and keep only domain-specific test registration in concrete exporter repositories.

Scaffold-generated exporters use Makefile-injected metadata through
`MainFromInjectedProject`, `ConfigFromInjectedProject`, and
`ExporterInfoFromInjectedProject`. This is the supported path for concrete
exporter repositories because exporter identity, metric namespace, default
listen address, Docker entrypoint, dashboards, and alert examples are rendered
from one scaffold contract.

Use `Config{...}` directly only for embedded programs or focused tests that
need lower-level runtime control. The framework no longer derives exporter
identity from Go module or project names.

For bootstrap code that must return errors instead of panicking or exiting, use
the `...Err` variants such as `MainFromInjectedProjectErr`,
`ConfigFromInjectedProjectErr`, and `ExporterInfoFromProjectMetadataErr`.
Helpers without the `Err` suffix are fail-fast convenience APIs for generated
main packages and tests.

`SmokeSpec.ServerArgs` is the supported place for smoke-only flag overrides,
including short refresh intervals or local config-file paths. Those values are
test inputs, not framework runtime defaults.

For embedded use, `RunContext`, `RunCLIContext`, and `NewRegistryContext` pass a caller-owned lifecycle context to startable collectors. `RunContext` also shuts the HTTP server down cleanly when that context is canceled.

## Applying This To New Exporters

Each exporter can become a thin concrete repository:

- `prometheus-demo-exporter`
  - place specific code at `internal/*`
  - exposes a feature that registers
  - `cmd/scaffold_main.go` calls the scaffold-owned internal exporter bridge

Add this framework module as a dependency:

```bash
go get github.com/zxzharmlesszxz/prometheus-exporter-framework@latest
```

For reproducible builds, pin a released version:

```go
require github.com/zxzharmlesszxz/prometheus-exporter-framework v*.*.*
```

## Built-In Flags

Every exporter built on this framework gets:

```bash
--web.listen-address
--web.telemetry-path
--web.config.file
--web.enable-pprof
--log.level
--log.format
```

`--web.telemetry-path` must be a literal URL path that starts with `/`.
`/healthz` and `/debug/pprof/*` are reserved for built-in handlers.

The concrete feature decides which domain flags to add.

## Local Run

Run the framework shell:

```bash
go run ./cmd --web.listen-address=:9900
```

It exposes only common metrics until a concrete exporter passes one or more features.

`pprof` is disabled by default:

```bash
go run ./cmd \
  --web.listen-address=:9900 \
  --web.enable-pprof
```

Endpoints:

- `http://localhost:9900`
- `http://localhost:9900/metrics`
- `http://localhost:9900/healthz`

## Tests

```bash
make check
```

`make check` runs formatting checks, `go vet`, `staticcheck`, `golangci-lint`,
`govulncheck`, coverage threshold checks, binary smoke tests, and race tests. The
coverage pass runs `go test ./...`, which includes public API golden checks; use
`make public-api-check` for targeted public API verification.

`make coverage-check` enforces `COVERAGE_THRESHOLD`, which defaults to `90.0`.
Override it when needed:

```bash
make coverage-check COVERAGE_THRESHOLD=95.0
```

The public API surface of `exporter`, `exporter/featurekit`, and
`exporter/exportertest` including its scaffold-used subpackages is tracked by
golden-file tests. The guard tracks exported identifiers, type alias targets,
exported methods, exported struct fields, and exported interface methods. The
root `exporter` guard also tracks exported fields, methods, and interface
methods exposed through aliases to local internal facade types. If one of those
changes intentionally, run `make public-api-update` and review the
`testdata/public_api.txt` diffs. The AST walker lives in
`exporter/internal/publicapitest`; keep guard behavior changes there so every
public package uses the same rules. Commit the golden-file changes together with
the code change.

`make smoke` builds the binary with injected version metadata, checks `--version`,
verifies telemetry-path validation, and probes `/healthz` plus `/metrics`.
Dockerfiles, Docker Compose examples, Prometheus configs, and Grafana dashboards
belong to the scaffold template and concrete exporter repositories, not to the
framework root.

See `MAINTAINING.md` for maintenance notes.

## Releases

Releases are manual because tags are public Go module versions for downstream exporters.
Pushes and pull requests run CI checks, including a scaffold compatibility job that uses the local scaffold/ template to render a demo exporter against the current framework checkout.

To publish a module version, run the `Release` workflow from the default branch and enter a tag such as `v0.1.0`.
The workflow runs `make check`, verifies the scaffold against the current
framework checkout with `make scaffold-check-local`, conditionally verifies the
scaffold against its pinned published framework dependency with
`make scaffold-check-pinned` when `scaffold/template/go.mod` does not point at
the release tag being created, verifies that no tracked or untracked
non-artifact files changed after checks, creates an annotated git tag, creates a
GitHub Release without binary or Docker artifacts, and records release notes
generated by GitHub (grouped by `.github/release.yml`).
After the GitHub Release is created, the workflow updates
`scaffold/template/go.mod` to the released framework version, validates the
pinned scaffold path, and pushes a follow-up scaffold pin commit to the default
branch.
If the tag already exists but the GitHub Release does not, the workflow treats that as a recovery run, verifies the tagged commit, and continues with release publication.
After a release, maintainers should update downstream scaffold consumers with
the drift tooling documented in [`scaffold/README.md`](scaffold/README.md), for
example `make -C scaffold drift-check TARGET_DIR=../prometheus-demo-exporter`
and `make -C scaffold drift-sync TARGET_DIR=../prometheus-demo-exporter`.

## Version Metadata

The smoke test and concrete exporter builds inject Prometheus `version` metadata through linker flags:

- `Version`
- `Branch`
- `Revision`
- `BuildUser`
- `BuildDate`

When linker flags are absent, the framework falls back to Go build info and then to `dev`.

## Requirements

Go 1.27 or newer is required intentionally.

This project is intended as a modern exporter framework and does not aim to support legacy Go versions.
