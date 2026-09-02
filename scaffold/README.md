# prometheus-exporter-framework scaffold

Scaffold directory for creating concrete Prometheus exporters from
`prometheus-exporter-framework`.

This directory owns generated-repository shape:

- project layout
- placeholder exporter feature
- typed snapshot collector wiring
- shared feature test suite usage
- example Prometheus, Grafana, and Docker Compose files
- GitHub Actions reusable-workflow wrapper and GitLab CI starter workflow
- Dependabot starter configuration
- rendering script

Exporter runtime behavior belongs in the framework packages; generated-project
boilerplate belongs here.

## Render A New Exporter

```bash
make new-exporter \
  PROJECT_NAME=prometheus-demo-exporter \
  GO_MODULE=github.com/example/prometheus-demo-exporter \
  PROJECT_DESC="Prometheus Demo Exporter" \
  FEATURE_NAME=demo \
  FEATURE_NAMESPACE=demo \
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

`FEATURE_NAME`, `FEATURE_NAMESPACE`, `METRIC_NAMESPACE`, `PROJECT_DESC`,
`GO_MODULE`, and `DEFAULT_PORT` have defaults, but passing them explicitly keeps
the generated repository predictable.
Use a canonical importable `GO_MODULE` such as
`github.com/example/prometheus-demo-exporter` for real repositories; the default
module value is only meant for local throwaway renders.
Set `DOCKER_SMOKE_METRIC`, `DOCKER_SMOKE_RUN_OPTIONS`,
`DOCKER_SMOKE_EXPORTER_ARGS`, and `DOCKER_SMOKE_EXTRA_METRICS` when the generated
exporter needs domain-specific Docker smoke-test wiring. Set
`DOCKER_SMOKE_RUN_OPTIONS=` or `DOCKER_SMOKE_EXPORTER_ARGS=` explicitly to render
an empty value instead of the scaffold default.

Metric namespaces are intentionally split:

- `FEATURE_NAME` names the Go feature package and runtime flags such as
  `--demo.config-file`.
- `FEATURE_NAMESPACE` prefixes domain metrics owned by the concrete exporter,
  for example `demo_example_value`.
- `METRIC_NAMESPACE` prefixes framework-owned exporter health metrics, for
  example `demo_exporter_last_collection_success`.

`TARGET_DIR` defaults to `rendered/$(PROJECT_NAME)` for local experiments.
Run `make check` in this scaffold repository to render a demo exporter, add a
temporary `replace` to the local framework checkout, check for unresolved
placeholders, verify scaffold drift, verify `go mod tidy` idempotence, and run
generated Go-only checks.

The rendered GitHub Actions CI file is intentionally a thin wrapper around the
framework-owned reusable workflow at
`zxzharmlesszxz/prometheus-exporter-framework/.github/workflows/exporter-ci.yml`.
The wrapper pins the workflow to the framework version from `template/go.mod`, so
exporter repositories keep their own checkout/build context without copying the
full CI implementation. The reusable workflow owns shared checks such as Go
checks, Docker smoke tests, release artifact builds, Trivy repository scanning,
and pre-push Docker image version/vulnerability gates.

The generated `cmd/scaffold_main.go` is intentionally stable. Project metadata is
injected by Makefile linker flags from `Makefile.mk`, while the concrete feature
package owns domain behavior.

## Framework Version

`template/go.mod` tracks the latest released
`prometheus-exporter-framework` version used by newly generated exporters.

Before publishing a new framework tag, run `make scaffold-check-local` from the
repository root, or `make check-local` inside `scaffold/`. The check renders a
demo exporter, adds a temporary `replace` directive to this local framework
checkout, verifies generated module files after `go mod tidy`, and runs the
generated exporter's Go-only checks. The target also fails if any
`__PLACEHOLDER__` values remain in rendered files.

The release workflow creates the framework tag first. After the GitHub Release
exists and the Go proxy can resolve the new module version, the workflow updates
`template/go.mod` to that released version, validates the pinned scaffold path,
and pushes a follow-up scaffold pin commit to the default branch.

This repository's own CI uses the root `make scaffold-check-local` path through
the compatibility workflow, so local and CI scaffold checks validate the same
generated code path against the current framework checkout.

When `template/go.mod` points at the latest published framework tag, also run
`make scaffold-check-pinned` from the repository root, or `make check-pinned`
inside `scaffold/`, before release. That verifies the rendered exporter against
the pinned published dependency instead of the local checkout.

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
