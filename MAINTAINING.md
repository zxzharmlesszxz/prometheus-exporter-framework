# Maintaining

This repository is a reusable exporter framework, not a product binary.
The main goal is to keep the framework safe for downstream exporters to import
and copy from.

## Local Checks

Run the standard maintenance check before changing the framework:

```bash
make check
```

`make check` runs formatting checks, `go vet`, `staticcheck`, `golangci-lint`,
`govulncheck`, coverage threshold checks, binary smoke tests, and race tests. The
coverage pass runs `go test ./...`, which includes public API golden checks; use
`make public-api-check` for targeted public API verification.
CI also runs scaffold compatibility by rendering a demo exporter from the local
`scaffold/` template against the current framework checkout, checking for
unresolved placeholders, checking generated module-file tidiness, and running
the generated exporter's Go-only checks.

Concrete exporter scaffolding lives in `scaffold/` in this repository. The
release workflow verifies both the framework and the local scaffold template
before publishing a new module tag and creating the GitHub Release. When
`scaffold/template/go.mod` points at an already published framework dependency
instead of the release tag being created, the workflow also runs scaffold checks
without the local framework `replace`.
If a module tag exists without a GitHub Release, rerun the workflow with the same
version to verify the tagged commit and create the GitHub Release.
If the post-release scaffold pin update fails after the GitHub Release is
created, rerun the release workflow with the same version after fixing the
failure; the workflow reuses the existing tag and release state.
The framework root keeps a stricter default coverage threshold (`90.0`) than
generated exporters (`85.0`) because downstream domain packages usually start
from sparse scaffold code and harden coverage as exporter logic grows.

## Version Tags

Downstream exporters may pin this module with `go get ...@vX.Y.Z`, so semver
tags are still useful for module consumption.
Those tags do not imply publishing this repository's framework binary or Docker
image as an end-user release artifact.

Before tagging:

1. Run `make check`.
2. Run `make scaffold-check-local`.
3. Review the public API list in `ARCHITECTURE.md` if exported identifiers,
   alias targets, alias-exposed members, methods, struct fields, or interface
   methods changed.
4. Ensure tracked files and non-artifact untracked files are clean after checks.
5. Tag with semver when downstream projects need a stable module version.

## Next Release Backlog

- Evaluate replacing `go.yaml.in/yaml/v3` with `github.com/goccy/go-yaml` for
  feature config parsing. Treat this as a parser behavior change, not a
  mechanical dependency update. Before switching, prove compatibility for
  unknown fields, duplicate keys, empty files, `null`, scalar coercion,
  duration/string/list fields used by concrete exporter configs, and parse error
  messages surfaced through `config_error`.
- Evaluate CI security hardening further: decide whether GitHub Actions should
  be pinned by SHA in root workflows and/or the scaffold template.
- Keep `scaffold-drift.sh` changes conservative until edge cases around legacy
  exporters and feature-owned files have broader regression coverage.

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
- `exporter/exportertest/*`
- `exporter/exportertest/adaptertest/*`
- `exporter/exportertest/featuretest/*`
- `exporter/exportertest/smoketest/*`

must update the corresponding `public_api.txt` golden file. Use
`make public-api-update` when an exported identifier, alias target,
alias-exposed member, method, struct field, or interface method change is
intentional, then review the golden-file diff before committing. Public API
guard behavior belongs in `exporter/internal/publicapitest`; do not copy the AST
walker into individual package tests.
