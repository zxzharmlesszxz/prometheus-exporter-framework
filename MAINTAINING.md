# Maintaining

This repository is a reusable exporter framework, not a product binary.
The main goal is to keep the framework safe for downstream exporters to import
and copy from.

## Local Checks

Run the standard maintenance check before changing the framework:

```bash
make check
```

`make check` runs formatting checks, `go vet`, `staticcheck`, coverage threshold
checks, binary smoke tests, and race tests.
CI also runs scaffold compatibility by rendering a demo exporter from the local
`scaffold/` template against the current framework checkout.

Concrete exporter scaffolding lives in `scaffold/` in this repository. The
release workflow verifies both the framework and the local scaffold template
before publishing a new module tag and creating the GitHub Release.
If a module tag exists without a GitHub Release, rerun the workflow with the same
version to verify the tagged commit and create the GitHub Release.

## Version Tags

Downstream exporters may pin this module with `go get ...@vX.Y.Z`, so semver
tags are still useful for module consumption.
Those tags do not imply publishing this repository's framework binary or Docker
image as an end-user release artifact.

Before tagging:

1. Run `make check`.
2. Run `make scaffold-compatibility`.
3. Review the public API list in `ARCHITECTURE.md` if exported symbols changed.
4. Tag with semver when downstream projects need a stable module version.

## Version Metadata

The smoke test and concrete exporter build validation pass linker values to
`github.com/prometheus/common/version`:

- `Version`
- `Branch`
- `Revision`
- `BuildUser`
- `BuildDate`

The binary smoke test verifies this metadata through `--version` and the
`*_build_info` metric. Concrete exporter repositories should own their own
publishing flow and release metadata policy.

## Public API changes

Changes to:

- `exporter/*`
- `exporter/featurekit/*`

must update the corresponding `public_api.txt` golden file.
