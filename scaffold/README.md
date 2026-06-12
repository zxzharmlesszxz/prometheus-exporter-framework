# prometheus-exporter-scaffold

Scaffold repository for creating concrete Prometheus exporters from
`prometheus-exporter-framework`.

This repository owns generated-repository shape:

- project layout
- placeholder exporter feature
- typed snapshot collector wiring
- shared feature test suite usage
- example Prometheus, Grafana, and Docker Compose files
- GitHub Actions and GitLab CI starter workflows
- Dependabot starter configuration
- rendering script

The framework itself lives in
`github.com/zxzharmlesszxz/prometheus-exporter-framework`. Keep exporter runtime
behavior in the framework and keep generated-project boilerplate here.

## Render A New Exporter

```bash
make new-exporter \
  PROJECT_NAME=prometheus-demo-exporter \
  GO_MODULE=github.com/example/prometheus-demo-exporter \
  PROJECT_DESC="Prometheus Demo Exporter" \
  FEATURE_NAME=demo \
  METRIC_NAMESPACE=demo_exporter \
  DEFAULT_PORT=9888 \
  TARGET_DIR=/tmp/prometheus-demo-exporter
```

Then validate the generated repository:

```bash
cd /tmp/prometheus-demo-exporter
go mod tidy
make go-check
make check
```

`FEATURE_NAME`, `METRIC_NAMESPACE`, `PROJECT_DESC`, `GO_MODULE`, and
`DEFAULT_PORT` have defaults, but passing them explicitly keeps the generated
repository predictable.

`TARGET_DIR` defaults to `rendered/$(PROJECT_NAME)` for local experiments.
Run `make check` in this scaffold repository to render a demo exporter, check
for unresolved placeholders, verify scaffold drift, and run generated Go-only
checks.

The generated `cmd/scaffold_main.go` is intentionally stable. Project metadata is
injected by Makefile linker flags from `Makefile.mk`, while the concrete feature
package owns domain behavior.

## Framework Version

`template/go.mod` track the
`prometheus-exporter-framework` version used by newly generated exporters.

When a new framework tag is published, the framework release workflow opens an
issue here requesting a scaffold update. The scaffold repository then consumes
the published framework tag through its own normal change flow and verifies a
rendered exporter.

This repository's own CI also renders a demo exporter and runs its Go-only
checks, so scaffold pull requests validate the generated code path directly.

## Update An Existing Exporter

Existing exporters are not coupled to this repository after rendering. To check
or sync scaffold-owned files against the current template, run:

```bash
make drift-check TARGET_DIR=../prometheus-demo-exporter
```

To update the default managed files:

```bash
make drift-sync TARGET_DIR=../prometheus-demo-exporter
```

The default managed set is intentionally conservative: CI files, ignore files,
`cmd/scaffold_main.go`, Dependabot config, `Makefile`, `Makefile.mk`, and the
thin scaffold-owned adapter in `internal/exporter/scaffold_exporter.go`. It also
includes the thin feature assembly file, shared feature test suite core, and
binary smoke test under `scaffold_*.go` names.
Those Go files are fully scaffold-owned and should not be edited in concrete
exporters. The stable feature contract itself lives in framework `featurekit`.
Concrete exporters keep domain logic in adjacent feature-package files and the
feature check package, so inspect those files separately instead of blindly
syncing them:

`feature_config_ext.go` owns the feature-specific `Config`, defaults, flag
specs, config validation, config-file merge behavior, and runtime config entries
that are wired into the framework-owned feature contract.

Feature contract tests should use `scaffold_feature_test_suite_test.go` for the
thin scaffold bridge into framework `exporter/exportertest/featuretest` and
register exporter-specific checks from `feature_test_suite_ext_test.go`.
Existing exporter test files can be migrated into that extension file instead
of editing scaffold-owned test code.

```bash
make drift-check TARGET_DIR=../prometheus-demo-exporter FILE=Makefile
make drift-check TARGET_DIR=../prometheus-demo-exporter FILE=Dockerfile
make drift-check TARGET_DIR=../prometheus-demo-exporter FILE=internal/exporter/scaffold_exporter.go
make drift-check TARGET_DIR=../prometheus-demo-exporter FILE=internal/demo/scaffold_feature.go
make drift-check TARGET_DIR=../prometheus-demo-exporter FILE=internal/demo/scaffold_feature_test_suite_test.go
make drift-check TARGET_DIR=../prometheus-demo-exporter FILE=internal/demo/snapshot_types.go
make drift-check TARGET_DIR=../prometheus-demo-exporter FILE=internal/demo/metrics.go
make drift-check TARGET_DIR=../prometheus-demo-exporter FILE=internal/demo/feature_config_ext.go
make drift-check TARGET_DIR=../prometheus-demo-exporter FILE=internal/demo/feature_metrics_ext.go
make drift-check TARGET_DIR=../prometheus-demo-exporter FILE=internal/demo/feature_snapshotter_ext.go
make drift-check TARGET_DIR=../prometheus-demo-exporter FILE=internal/demo/feature_smoke_ext.go
make drift-check TARGET_DIR=../prometheus-demo-exporter FILE=internal/demo/feature_test_suite_ext_test.go
```

Use `ALLOW_DIRTY=1` with `make drift-sync` when you intentionally want to sync
over already modified managed files. `make drift-list-files` prints the default
managed set.

New exporters include starter domain files such as `internal/<feature>/snapshot_types.go`
and `internal/<feature>check/*`. They are rendered by scaffold, but they are not
scaffold-owned after generation: a real exporter may replace the simple
`Snapshot` struct with an aggregate snapshot and may split the check package into
multiple domain packages.

`make drift-check` also compares the target exporter's
`prometheus-exporter-framework` requirement in `go.mod` with the scaffold
version from `template/go.mod`. If the target exporter uses an older framework,
the check prints an `OUTDATED framework ...` line and exits non-zero.

Older exporters may still have scaffold-owned bootstrap files under
`internal/exporter`, such as `defaults.go`, `feature.go`, `info.go`,
`standard_metrics.go`, and their tests. Current scaffold shape replaces that
set with `internal/exporter/scaffold_exporter.go`; project metadata, standard
metric names, and binary smoke metadata are supplied by the framework through
Makefile-injected linker variables. During drift sync, old scaffold-owned names
such as `cmd/main.go`, `internal/exporter/exporter.go`,
`internal/<feature>/feature.go`, and `smoke/binary_test.go` are treated as
obsolete in favor of their `scaffold_*.go` replacements.
When syncing one renamed file with `FILE=...`, the matching old filename is
also removed; unrelated obsolete files are left untouched.
