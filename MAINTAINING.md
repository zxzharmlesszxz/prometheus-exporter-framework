# Maintaining

This repository is a reusable exporter template, not a product binary.
The main goal is to keep the template safe for downstream exporters to import
and copy from.

## Local Checks

Run the standard maintenance check before changing the template:

```bash
make check
```

`make check` runs formatting checks, `go vet`, `staticcheck`, coverage threshold
checks, binary smoke tests, and race tests.

Concrete exporter scaffolding lives in the separate
`prometheus-exporter-scaffold` repository. The release workflow opens a pull
request there after a new module tag is published.

When Docker is available, validate the runtime image separately:

```bash
make docker-smoke
```

Docker smoke is intentionally optional because not every development or CI
environment has a Docker daemon.

## Version Tags

Downstream exporters may pin this module with `go get ...@vX.Y.Z`, so semver
tags are still useful for module consumption.
Those tags do not imply publishing this repository's template binary or Docker
image as an end-user release artifact.

Before tagging:

1. Run `make check`.
2. Optionally run `make docker-smoke`.
3. Review the public API list in `ARCHITECTURE.md` if exported symbols changed.
4. Tag with semver when downstream projects need a stable module version.

## Version Metadata

The Dockerfile and CI build validation pass linker values to
`github.com/prometheus/common/version`:

- `Version`
- `Branch`
- `Revision`
- `BuildUser`
- `BuildDate`

The binary smoke test verifies this metadata through `--version` and the
`*_build_info` metric. Concrete exporter repositories should own their own
publishing flow and release metadata policy.
