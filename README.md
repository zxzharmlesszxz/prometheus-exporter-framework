# prometheus-template-exporter

Reusable Go template for Prometheus exporters.

This repository owns only the stable exporter shell:

- CLI bootstrap and standard flags
- structured `promslog` logging
- exporter-toolkit HTTP serving
- `/metrics`, `/healthz`, landing page, and optional pprof endpoints
- Prometheus registry wiring
- `build_info`, Go runtime, and process collectors
- version metadata hydration from linker flags or Go build info

Concrete exporters add domain behavior through `exporter.Feature` implementations in their own repositories.
No code in this repository needs to change when a new exporter feature is added.

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

	template "github.com/zxzharmlesszxz/prometheus-template-exporter/exporter"
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

func (f *Feature) RegisterCollectors(ctx template.FeatureContext, registry *prometheus.Registry) error {
	collector := NewDomainCollector(ctx.Logger, *f.inputPath)
	return template.RegisterCollectors(registry, collector)
}

func (f *Feature) RuntimeConfig() []any {
	return []any{"input_path", *f.inputPath}
}

func main() {
	template.MainFromProject(&Feature{})
}
```

A compiling example feature is available in `examples/custom-feature`.

`ConfigFromProject` derives exporter name and metric namespace from the Go module/project name.
For example, `prometheus-pkg-exporter` becomes `pkg_exporter`.
The default listen address is taken from the first feature that implements `DefaultListenAddress() string`, otherwise it falls back to `:9900`.
Use `MainForProject(projectName, description, features...)` when an exporter needs explicit project metadata while still deriving the landing page name from the executable.
Use `Config{...}` directly only when a concrete exporter needs to override that derived metadata.

## Applying This To New Exporters

Each exporter can become a thin concrete repository:

- `prometheus-pkg-exporter`
  - place specific code at `internal/*`
  - exposes a feature that registers
  - `main.go` only calls `template.MainFromProject(...)` or `template.MainForProject(...)`

Add this template module as a dependency:

```bash
go get github.com/zxzharmlesszxz/prometheus-template-exporter@latest
```

For reproducible builds, pin a released version:

```go
require github.com/zxzharmlesszxz/prometheus-template-exporter v*.*.*
```

## Built-In Flags

Every exporter built on this template gets:

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

Run the template shell:

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

## Version Metadata

The Dockerfile and CI build inject Prometheus `version` metadata through linker flags:

- `Version`
- `Branch`
- `Revision`
- `BuildUser`
- `BuildDate`

When linker flags are absent, the template falls back to Go build info and then to `dev`.

## Requirements

Go 1.26 or newer is required intentionally.

This project is intended as a modern exporter template and does not aim to support legacy Go versions.
