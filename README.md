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
- small helpers for common metric values and exporter tests
- version metadata hydration from linker flags or Go build info

Concrete exporters add domain behavior through `exporter.Feature` implementations in their own repositories.
Concrete exporter scaffolding lives in the separate `prometheus-exporter-scaffold` repository.
No framework code in this repository needs to change when a new exporter feature is added.

## Extension Model

A feature owns two things:

- domain-specific flags
- domain-specific Prometheus collectors

Minimal example:

```go
package main

import (
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

func (f *Feature) DefaultListenAddress() string {
	return ":9901"
}

func (f *Feature) RegisterCollectors(ctx framework.FeatureContext, registry *prometheus.Registry) error {
	collector := NewDomainCollector(ctx.Logger, *f.inputPath)
	return framework.RegisterCollectors(registry, collector)
}

func (f *Feature) RuntimeConfig() []any {
	return []any{"input_path", *f.inputPath}
}

func main() {
	framework.MainFromProject(&Feature{})
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

## Utility Helpers

Concrete exporters can reuse small metric helpers instead of carrying local copies:

- `BoolFloat(bool)` for `0`/`1` gauge values
- `UnixTimestamp(time.Time)` for timestamp metrics with zero-time handling
- `FileMTimeSeconds(path)` for file mtime gauges that return `0` when the file cannot be statted
- `FileScrapeMetrics` for file-backed collectors that expose mtime, scrape duration, and read/parse counters
- `NormalizeDuration(value, fallback)` for duration flags where non-positive values should fall back to defaults
- `RegisterAndStartCollectors(ctx, registry, collectors...)` for collectors with a background `Start(context.Context)` lifecycle

Tests can import `github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/exportertest` for common registry/gather helpers, metric lookup, metric value assertions, histogram lookup, and polling metrics that are updated by background refresh loops.

`ConfigFromProject` derives exporter name and metric namespace from the Go module/project name.
For example, `prometheus-demo-exporter` becomes `demo_exporter`.
The default listen address is taken from the first feature that implements `DefaultListenAddress() string`, otherwise it falls back to `:9900`.
`MainFromProject(features...)` derives metric namespace and description from the Go module path while using the executable file name for CLI usage and the landing page.
Use `MainForProject(projectName, description, features...)` only when an exporter needs explicit project metadata.
Use `Config{...}` directly only when a concrete exporter needs lower-level overrides.

## Applying This To New Exporters

Each exporter can become a thin concrete repository:

- `prometheus-demo-exporter`
  - place specific code at `internal/*`
  - exposes a feature that registers
  - `main.go` only calls `framework.MainFromProject(...)` or `framework.MainForProject(...)`

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

## Grafana Dashboard

`examples/grafana/prometheus-exporter-framework-overview.json` is a starter dashboard for common exporter health.
It uses Prometheus scrape metadata plus the framework's Go, process, and `*_build_info` metrics.
Domain-specific feature dashboards should live in concrete exporter repositories.

## Tests

```bash
make check
```

`make check` runs formatting checks, `go vet`, `staticcheck`, coverage threshold checks, binary smoke tests, and race tests.

`make coverage-check` enforces `COVERAGE_THRESHOLD`, which defaults to `90.0`.
Override it when needed:

```bash
make coverage-check COVERAGE_THRESHOLD=95.0
```

`make smoke` builds the binary with injected version metadata, checks `--version`,
verifies telemetry-path validation, and probes `/healthz` plus `/metrics`.
`make docker-smoke` performs the same version and endpoint checks against the
Docker runtime image when Docker is available. It is optional and not part of
the default check target.

See `MAINTAINING.md` for maintenance notes.

## Releases

Releases are manual because tags are public Go module versions for downstream exporters.
Pushes and pull requests only run CI checks.

To publish a module version, run the `Release` workflow from the default branch and enter a tag such as `v0.1.0`.
The workflow runs `make check`, verifies the scaffold against the current framework checkout, creates an annotated git tag, opens a scaffold dependency update pull request, and only then creates a GitHub Release without binary or Docker artifacts.
If the tag already exists but the GitHub Release does not, the workflow treats that as a recovery run, verifies the tagged commit, and continues with the scaffold update and release publication.
Release notes are generated by GitHub and grouped by `.github/release.yml`; the generated notes include the compare link for the full changelog.
Configure `SCAFFOLD_REPO_TOKEN` with contents read/write and pull request write access to `zxzharmlesszxz/prometheus-exporter-scaffold` before using this automation; the GitHub Release is not published if the scaffold update fails.

## Version Metadata

The Dockerfile and CI build inject Prometheus `version` metadata through linker flags:

- `Version`
- `Branch`
- `Revision`
- `BuildUser`
- `BuildDate`

When linker flags are absent, the framework falls back to Go build info and then to `dev`.

## Requirements

Go 1.26 or newer is required intentionally.

This project is intended as a modern exporter framework and does not aim to support legacy Go versions.
