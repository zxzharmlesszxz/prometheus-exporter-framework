#!/usr/bin/env bash
set -euo pipefail
export LC_ALL=C

usage() {
  cat <<'USAGE'
Usage:
  scripts/scaffold-drift.sh --target-dir ../prometheus-demo-exporter
  scripts/scaffold-drift.sh --target-dir ../prometheus-demo-exporter --sync

Checks or syncs scaffold-owned files in an existing exporter against the current
template, or checks every rendered scaffold file with --all-files. By default
the script only reports drift and exits non-zero when drift is found.

Required:
  --target-dir DIR       Existing exporter repository to compare.

Modes:
  --check                Report drift only. This is the default.
  --sync                 Copy rendered scaffold-owned files into --target-dir.
  --allow-dirty          Allow --sync when managed files already have git changes.
  --symbol-diff          For drifted Go files, print per-symbol differences for
                         funcs, methods, types, vars, and consts.

Render metadata overrides:
  --project-name NAME    Defaults to basename of --target-dir.
  --module MODULE        Defaults to module path from go.mod.
  --description TEXT     Defaults to rendered exporter description, README H1, or project name.
  --feature-name NAME    Defaults to FEATURE_NAME from Makefile.mk or derived.
  --namespace NAME       Defaults to METRIC_NAMESPACE from Makefile.mk or derived.
  --port PORT            Defaults to DEFAULT_PORT from Makefile.mk or 9888.
  --feature-config-file NAME
                         Defaults to FEATURE_CONFIG_FILE from Makefile.mk or <project-name>.yml.
  --docker-smoke-metric TEXT
                         Defaults to DOCKER_SMOKE_METRIC from Makefile.mk or skeleton default.
  --docker-smoke-run-options TEXT
                         Extra options passed to `docker run` before the image.
  --docker-smoke-exporter-args TEXT
                         Defaults to DOCKER_SMOKE_EXPORTER_ARGS from Makefile.mk or the config-file arg.
  --docker-smoke-extra-metrics TEXT
                         Additional metric assertions separated by '|'.

File selection:
  --file PATH            Compare/sync this rendered path. Can be repeated.
  --all-files            Check every file rendered by the scaffold. Check-only;
                         refused with --sync to avoid overwriting feature code.
  --list-files           Print the default managed file list and exit.

Default managed files:
  LICENSE
  Makefile
  Makefile.mk
  docker-compose.yml
  .dockerignore
  .github/dependabot.yml
  .github/workflows/ci.yml
  .gitignore
  .gitlab-ci.yml
  cmd/scaffold_main.go
  internal/exporter/scaffold_exporter.go
  internal/exporter/scaffold_exporter_test.go
  internal/__FEATURE_NAME__/scaffold_feature.go
  internal/__FEATURE_NAME__/scaffold_feature_test_suite_test.go
  smoke/scaffold_binary_test.go

Framework version:
  Check mode compares the target exporter's go.mod
  github.com/zxzharmlesszxz/prometheus-exporter-framework version with the
  scaffold version in template/go.mod. Older target versions are reported as
  OUTDATED and make the check fail.

Makefile should stay scaffold-managed. Domain-specific Docker smoke mounts,
exporter arguments, and extra metric checks belong in Makefile.mk variables.
docker-compose.yml should stay scaffold-managed. Domain-specific Compose
commands, mounts, configs, and local example wiring belong in
docker-compose.override.yml.
Dockerfiles can also be domain-specific when exporters need runtime packages.
Generated Go files named scaffold_*.go are fully scaffold-owned. Do not edit
them in concrete exporters; add feature behavior in adjacent non-scaffold files.
Legacy exporters may still keep older scaffold-owned files under
internal/exporter. Current scaffold shape keeps only a thin adapter in
internal/exporter/scaffold_exporter.go and moves reusable bootstrap/info behavior
into the framework.
Domain-specific metric constants, metric implementations, snapshots, snapshotters,
check packages, and feature-specific tests should live outside the adapter
package, normally under internal/<feature-name> and internal/<feature-name>check.
The scaffold-owned feature lifecycle is split from domain behavior. The files
internal/<feature-name>/scaffold_feature.go,
internal/<feature-name>/scaffold_feature_test_suite_test.go define the
stable feature assembly and shared feature test suite core. These files should
stay identical to the rendered scaffold; feature
construction, config-file flag registration, feature config flag spec loading,
runtime config, collector construction, metrics wiring, snapshot status wiring,
and smoke wiring belong to framework featurekit, while domain behavior belongs
in feature config extension hooks for defaults, flags, validation, config
resolution, and runtime config; feature metrics extension, feature snapshot
engine extension, feature smoke extension, feature test suite extension, and
check/lookup files. Newly rendered exporters get starter snapshot/check files,
but those are feature-owned after generation because real exporters may replace
the simple Snapshot alias with an aggregate snapshot backed by multiple domain
packages.
Inspect domain-specific skeleton files with concrete rendered paths such as
--file internal/demo/feature_config_ext.go or
--file internal/domain/feature_metrics_ext.go; these files are intentionally not
part of the default managed set.
The stable exporter feature adapter is intentionally compact:
`internal/exporter/scaffold_exporter.go` only imports the domain package, creates the
feature with framework-injected metadata, and delegates Main()/ExporterInfo()
to the framework. Older split files such as `feature.go`, `defaults.go`,
`info.go`, `standard_metrics.go`, and their tests are obsolete and are removed
during default `--sync`.
USAGE
}

mode="check"
allow_dirty=0
list_files=0
target_dir=""
project_name=""
go_module=""
project_desc=""
feature_name=""
metric_namespace=""
default_port=""
feature_config_file=""
docker_smoke_metric=""
docker_smoke_run_options=""
docker_smoke_exporter_args=""
docker_smoke_extra_metrics=""
custom_files=()
symbol_diff=0
all_files=0
framework_module="github.com/zxzharmlesszxz/prometheus-exporter-framework"

default_files=(
  "LICENSE"
  "Makefile"
  "Makefile.mk"
  "docker-compose.yml"
  ".dockerignore"
  ".github/dependabot.yml"
  ".github/workflows/ci.yml"
  ".gitignore"
  ".gitlab-ci.yml"
  "cmd/scaffold_main.go"
  "internal/exporter/scaffold_exporter.go"
  "internal/exporter/scaffold_exporter_test.go"
  "internal/__FEATURE_NAME__/scaffold_feature.go"
  "internal/__FEATURE_NAME__/scaffold_feature_test_suite_test.go"
  "smoke/scaffold_binary_test.go"
)

obsolete_files=(
  "cmd/main.go"
  "internal/exporter/exporter.go"
  "internal/exporter/exporter_test.go"
  "internal/__FEATURE_NAME__/feature.go"
  "internal/__FEATURE_NAME__/feature_config_flags.go"
  "internal/__FEATURE_NAME__/scaffold_feature_config_flags.go"
  "internal/__FEATURE_NAME__/collector_test_helpers_test.go"
  "internal/__FEATURE_NAME__/scaffold_collector_test_helpers_test.go"
  "smoke/binary_test.go"
  "internal/__FEATURE_NAME__/feature_spec.go"
  "internal/__FEATURE_NAME__/feature_config.go"
  "internal/__FEATURE_NAME__/feature_metrics.go"
  "internal/__FEATURE_NAME__/feature_snapshot.go"
  "internal/__FEATURE_NAME__/feature_snapshotter.go"
  "internal/__FEATURE_NAME__/feature_smoke.go"
  "internal/exporter/defaults.go"
  "internal/exporter/feature.go"
  "internal/exporter/feature_collectors.go"
  "internal/exporter/feature_collectors_test.go"
  "internal/exporter/feature_flags.go"
  "internal/exporter/feature_flags_test.go"
  "internal/exporter/feature_integration_test.go"
  "internal/exporter/feature_integration_test_helpers_test.go"
  "internal/exporter/feature_test.go"
  "internal/exporter/featurekit/feature.go"
  "internal/exporter/featurekit/feature_test.go"
  "internal/exporter/featurekit/snapshot.go"
  "internal/exporter/featurekit/snapshot_test.go"
  "internal/exporter/feature_test_helpers_test.go"
  "internal/exporter/identity.go"
  "internal/exporter/identity_test.go"
  "internal/exporter/info.go"
  "internal/exporter/info_test.go"
  "internal/exporter/main.go"
  "internal/exporter/runtime_config.go"
  "internal/exporter/runtime_config_test.go"
  "internal/exporter/standard_metrics.go"
  "internal/__FEATURE_NAME__/collector.go"
  "internal/__FEATURE_NAME__/collector_metrics.go"
  "internal/__FEATURE_NAME__/exporter.go"
  "internal/__FEATURE_NAME__/smoke.go"
  "internal/__FEATURE_NAME__/smoke_test.go"
  "internal/__FEATURE_NAME__/snapshot.go"
  "internal/__FEATURE_NAME__/snapshotter.go"
  "internal/__FEATURE_NAME__/spec.go"
)

companion_obsolete_files_for_managed_file() {
  local file="$1"
  case "$file" in
    cmd/scaffold_main.go)
      printf '%s\n' "cmd/main.go"
      ;;
    internal/exporter/scaffold_exporter.go)
      printf '%s\n' "internal/exporter/exporter.go"
      ;;
    internal/exporter/scaffold_exporter_test.go)
      printf '%s\n' "internal/exporter/exporter_test.go"
      ;;
    internal/*/scaffold_feature.go)
      printf '%s\n' "${file%/scaffold_feature.go}/feature.go"
      ;;
    internal/*/scaffold_feature_test_suite_test.go)
      printf '%s\n' "${file%/scaffold_feature_test_suite_test.go}/collector_test_helpers_test.go"
      printf '%s\n' "${file%/scaffold_feature_test_suite_test.go}/scaffold_collector_test_helpers_test.go"
      ;;
    smoke/scaffold_binary_test.go)
      printf '%s\n' "smoke/binary_test.go"
      ;;
  esac
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --target-dir|--exporter-dir)
      target_dir="${2:-}"
      shift 2
      ;;
    --check)
      mode="check"
      shift
      ;;
    --sync)
      mode="sync"
      shift
      ;;
    --allow-dirty)
      allow_dirty=1
      shift
      ;;
    --project-name)
      project_name="${2:-}"
      shift 2
      ;;
    --module)
      go_module="${2:-}"
      shift 2
      ;;
    --description)
      project_desc="${2:-}"
      shift 2
      ;;
    --feature-name)
      feature_name="${2:-}"
      shift 2
      ;;
    --namespace)
      metric_namespace="${2:-}"
      shift 2
      ;;
    --port)
      default_port="${2:-}"
      shift 2
      ;;
    --feature-config-file)
      feature_config_file="${2:-}"
      shift 2
      ;;
    --docker-smoke-metric)
      docker_smoke_metric="${2:-}"
      shift 2
      ;;
    --docker-smoke-run-options)
      docker_smoke_run_options="${2:-}"
      shift 2
      ;;
    --docker-smoke-exporter-args)
      docker_smoke_exporter_args="${2:-}"
      shift 2
      ;;
    --docker-smoke-extra-metrics)
      docker_smoke_extra_metrics="${2:-}"
      shift 2
      ;;
    --file)
      custom_files+=("${2:-}")
      shift 2
      ;;
    --all-files|--all-rendered-files)
      all_files=1
      shift
      ;;
    --symbol-diff|--compare-symbols)
      symbol_diff=1
      shift
      ;;
    --list-files)
      list_files=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ "$list_files" -eq 1 ]]; then
  printf '%s\n' "${default_files[@]}"
  exit 0
fi

if [[ -z "$target_dir" ]]; then
  usage >&2
  exit 1
fi
if [[ "$all_files" -eq 1 && "$mode" == "sync" ]]; then
  echo "--all-files is check-only; use repeated --file with --sync for intentional overwrites" >&2
  exit 2
fi
if [[ ! -d "$target_dir" ]]; then
  echo "target dir does not exist: $target_dir" >&2
  exit 1
fi

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_dir="$(cd "$script_dir/.." && pwd)"
target_dir="$(cd "$target_dir" && pwd)"

managed_files=("${default_files[@]}")
managed_obsolete_files=("${obsolete_files[@]}")
if [[ "${#custom_files[@]}" -gt 0 ]]; then
  managed_files=("${custom_files[@]}")
  managed_obsolete_files=()
  for file in "${custom_files[@]}"; do
    while IFS= read -r obsolete_file; do
      [[ -n "$obsolete_file" ]] || continue
      managed_obsolete_files+=("$obsolete_file")
    done < <(companion_obsolete_files_for_managed_file "$file")
  done
fi
if [[ "$all_files" -eq 1 && "${#custom_files[@]}" -gt 0 ]]; then
  echo "--all-files cannot be combined with --file" >&2
  exit 2
fi

detect_module() {
  [[ -f "$target_dir/go.mod" ]] || return 0
  awk '$1 == "module" {print $2; exit}' "$target_dir/go.mod"
}

detect_go_mod_required_version() {
  local go_mod="$1"
  local module="$2"
  [[ -f "$go_mod" ]] || return 0
  awk -v module="$module" '
    $1 == "require" && $2 == module {
      print $3
      exit
    }
    $1 == module {
      print $2
      exit
    }
  ' "$go_mod"
}

version_core() {
  local version="$1"
  version="${version#v}"
  version="${version%%+*}"
  version="${version%%-*}"
  printf '%s' "$version"
}

compare_versions() {
  local left right left_core right_core
  local left_major left_minor left_patch right_major right_minor right_patch
  left="$1"
  right="$2"

  if [[ "$left" == "$right" ]]; then
    echo "equal"
    return 0
  fi

  left_core="$(version_core "$left")"
  right_core="$(version_core "$right")"
  if [[ ! "$left_core" =~ ^[0-9]+(\.[0-9]+){0,2}$ || ! "$right_core" =~ ^[0-9]+(\.[0-9]+){0,2}$ ]]; then
    echo "unknown"
    return 0
  fi

  IFS=. read -r left_major left_minor left_patch <<<"$left_core"
  IFS=. read -r right_major right_minor right_patch <<<"$right_core"
  left_minor="${left_minor:-0}"
  left_patch="${left_patch:-0}"
  right_minor="${right_minor:-0}"
  right_patch="${right_patch:-0}"

  if (( left_major < right_major )); then
    echo "older"
  elif (( left_major > right_major )); then
    echo "newer"
  elif (( left_minor < right_minor )); then
    echo "older"
  elif (( left_minor > right_minor )); then
    echo "newer"
  elif (( left_patch < right_patch )); then
    echo "older"
  elif (( left_patch > right_patch )); then
    echo "newer"
  else
    echo "unknown"
  fi
}

detect_project_name() {
  local file value
  for file in "$target_dir/Makefile.mk" "$target_dir/Makefile"; do
    [[ -f "$file" ]] || continue
    value="$(awk -F '\\?=' '
      /^[[:space:]]*PROJECT_NAME[[:space:]]*\?=/ {
        value = $2
        sub(/^[[:space:]]*/, "", value)
        sub(/[[:space:]]*$/, "", value)
        print value
        exit
      }
    ' "$file")"
    if [[ -n "$value" && "$value" != *'$('* ]]; then
      printf '%s' "$value"
      return 0
    fi
  done
  if [[ -d "$target_dir/internal/exporter" ]]; then
    while IFS= read -r file; do
      value="$(awk '
        /ExporterName[[:space:]]*=/ && /"/ {
          line = $0
          sub(/^.*ExporterName[[:space:]]*=[[:space:]]*"/, "", line)
          sub(/".*$/, "", line)
          print line
          exit
        }
        /DefaultExporterName[[:space:]]*=/ && /"/ {
          line = $0
          sub(/^.*DefaultExporterName[[:space:]]*=[[:space:]]*"/, "", line)
          sub(/".*$/, "", line)
          print line
          exit
        }
      ' "$file")"
      if [[ -n "$value" ]]; then
        printf '%s' "$value"
        return 0
      fi
    done < <(find "$target_dir/internal/exporter" -maxdepth 1 -type f -name '*.go' -print 2>/dev/null | sort)
  fi
}

detect_readme_h1() {
  [[ -f "$target_dir/README.md" ]] || return 0
  awk '/^#[[:space:]]+/ {sub(/^#[[:space:]]+/, ""); print; exit}' "$target_dir/README.md"
}

detect_exporter_description() {
  local dir="$target_dir/internal/exporter"
  local file value
  for file in "$target_dir/Makefile.mk" "$target_dir/Makefile"; do
    [[ -f "$file" ]] || continue
    value="$(awk -F '\\?=' '
      /^[[:space:]]*PROJECT_DESC[[:space:]]*\?=/ {
        value = $2
        sub(/^[[:space:]]*/, "", value)
        sub(/[[:space:]]*$/, "", value)
        print value
        exit
      }
    ' "$file")"
    if [[ -n "$value" && "$value" != *'$('* ]]; then
      printf '%s' "$value"
      return 0
    fi
  done
  [[ -d "$dir" ]] || return 0
  while IFS= read -r file; do
    value="$(awk '
      /ExporterDescription[[:space:]]*=/ && /"/ {
        line = $0
        sub(/^.*ExporterDescription[[:space:]]*=[[:space:]]*"/, "", line)
        sub(/".*$/, "", line)
        print line
        exit
      }
      /DefaultExporterDescription[[:space:]]*=/ && /"/ {
        line = $0
        sub(/^.*DefaultExporterDescription[[:space:]]*=[[:space:]]*"/, "", line)
        sub(/".*$/, "", line)
        print line
        exit
      }
    ' "$file")"
    if [[ -n "$value" ]]; then
      printf '%s' "$value"
      return 0
    fi
  done < <(find "$dir" -maxdepth 1 -type f -name '*.go' -print 2>/dev/null | sort)
}

detect_feature_name() {
  local dir="$target_dir/internal/exporter"
  local file value
  for file in "$target_dir/Makefile.mk" "$target_dir/Makefile"; do
    [[ -f "$file" ]] || continue
    value="$(awk -F '\\?=' '
      /^[[:space:]]*FEATURE_NAME[[:space:]]*\?=/ {
        value = $2
        sub(/^[[:space:]]*/, "", value)
        sub(/[[:space:]]*$/, "", value)
        print value
        exit
      }
    ' "$file")"
    if [[ -n "$value" && "$value" != *'$('* ]]; then
      printf '%s' "$value"
      return 0
    fi
  done
  [[ -d "$dir" ]] || return 0
  while IFS= read -r file; do
    value="$(awk '
      /defaultFeatureName[[:space:]]*=/ && /"/ {
        line = $0
        sub(/^.*defaultFeatureName[[:space:]]*=[[:space:]]*"/, "", line)
        sub(/".*$/, "", line)
        print line
        exit
      }
      /DefaultFeatureName[[:space:]]*=/ && /"/ {
        line = $0
        sub(/^.*DefaultFeatureName[[:space:]]*=[[:space:]]*"/, "", line)
        sub(/".*$/, "", line)
        print line
        exit
      }
      /FeatureName[[:space:]]*=/ && /"/ {
        line = $0
        sub(/^.*FeatureName[[:space:]]*=[[:space:]]*"/, "", line)
        sub(/".*$/, "", line)
        print line
        exit
      }
      /FeatureName\(\)[[:space:]]+string/ {in_func = 1}
      in_func && /return[[:space:]]+"/ {
        line = $0
        sub(/^.*return[[:space:]]+"/, "", line)
        sub(/".*$/, "", line)
        print line
        exit
      }
      in_func && /^}/ {in_func = 0}
    ' "$file")"
    if [[ -n "$value" ]]; then
      printf '%s' "$value"
      return 0
    fi
  done < <(find "$dir" -maxdepth 1 -type f -name '*.go' -print 2>/dev/null | sort)
}

detect_default_port() {
  local dir="$target_dir/internal/exporter"
  local file value
  for file in "$target_dir/Makefile.mk" "$target_dir/Makefile"; do
    [[ -f "$file" ]] || continue
    value="$(awk -F '\\?=' '
      /^[[:space:]]*DEFAULT_PORT[[:space:]]*\?=/ {
        value = $2
        sub(/^[[:space:]]*/, "", value)
        sub(/[[:space:]]*$/, "", value)
        sub(/^:/, "", value)
        print value
        exit
      }
    ' "$file")"
    if [[ -n "$value" && "$value" != *'$('* ]]; then
      printf '%s' "$value"
      return 0
    fi
  done
  [[ -d "$dir" ]] || return 0
  while IFS= read -r file; do
    value="$(awk '
      /ListenAddress[[:space:]]*=/ && /":[0-9]+"/ {
        line = $0
        sub(/^.*:"?/, "", line)
        sub(/".*$/, "", line)
        print line
        exit
      }
      /defaultListenAddress[[:space:]]*=/ && /":[0-9]+"/ {
        line = $0
        sub(/^.*:"?/, "", line)
        sub(/".*$/, "", line)
        print line
        exit
      }
      /DefaultListenAddress[[:space:]]*=/ && /":[0-9]+"/ {
        line = $0
        sub(/^.*:"?/, "", line)
        sub(/".*$/, "", line)
        print line
        exit
      }
    ' "$file")"
    if [[ -n "$value" ]]; then
      printf '%s' "$value"
      return 0
    fi
  done < <(find "$dir" -maxdepth 1 -type f -name '*.go' -print 2>/dev/null | sort)
}

sanitize_metric_namespace() {
  local value="$1"
  value="$(printf '%s' "$value" | tr '[:upper:]' '[:lower:]' | sed -e 's/[^a-z0-9]/_/g' -e 's/_\{1,\}/_/g' -e 's/^_*//' -e 's/_*$//')"
  if [[ -z "$value" ]]; then
    value="exporter_framework"
  fi
  if [[ "$value" =~ ^[0-9] ]]; then
    value="_$value"
  fi
  if [[ "$value" != "exporter_framework" && "$value" != *_exporter ]]; then
    value="${value}_exporter"
  fi
  printf '%s' "$value"
}

derive_namespace_from_project() {
  local project="$1"
  local base="${project##*/}"
  base="${base#prometheus-}"
  base="${base%-exporter}"
  sanitize_metric_namespace "$base"
}

detect_namespace() {
  local match=""
  local file value
  for file in "$target_dir/Makefile.mk" "$target_dir/Makefile"; do
    [[ -f "$file" ]] || continue
    value="$(awk -F '\\?=' '
      /^[[:space:]]*METRIC_NAMESPACE[[:space:]]*\?=/ {
        value = $2
        sub(/^[[:space:]]*/, "", value)
        sub(/[[:space:]]*$/, "", value)
        print value
        exit
      }
    ' "$file")"
    if [[ -n "$value" && "$value" != *'$('* ]]; then
      printf '%s' "$value"
      return 0
    fi
  done
  if [[ -d "$target_dir/internal/exporter" ]]; then
    match="$(find "$target_dir/internal/exporter" -maxdepth 1 -type f -name '*.go' -print 2>/dev/null | sort | while IFS= read -r file; do
      sed -n \
        -e 's/.*MetricNamespace[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' \
        -e 's/.*DefaultMetricNamespace[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' \
        -e 's/.*defaultMetricNamespace[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' \
        -e 's/.*Namespace:[[:space:]]*"\([^"]*\)".*/\1/p' \
        "$file"
    done | head -n 1)"
  fi
  if [[ -n "$match" ]]; then
    printf '%s' "$match"
    return 0
  fi
  derive_namespace_from_project "${go_module:-$project_name}"
}

detect_makefile_mk_var() {
  local name="$1"
  [[ -f "$target_dir/Makefile.mk" ]] || return 0
  awk -v name="$name" -F '\\?=' '
    $1 ~ "^[[:space:]]*" name "[[:space:]]*$" {
      value = $2
      sub(/^[[:space:]]*/, "", value)
      sub(/[[:space:]]*$/, "", value)
      print value
      exit
    }
  ' "$target_dir/Makefile.mk"
}

detect_docker_smoke_metric() {
  detect_makefile_mk_var "DOCKER_SMOKE_METRIC"
}

feature_go_defines() {
  local pattern="$1"
  local file="$target_dir/internal/exporter/feature.go"
  [[ -f "$file" ]] || return 1
  grep -Eq "$pattern" "$file"
}

exporter_go_defines_except() {
  local skip_path="$target_dir/$1"
  local pattern="$2"
  local dir="$target_dir/internal/exporter"
  local file
  [[ -d "$dir" ]] || return 1
  while IFS= read -r file; do
    if [[ "$file" == "$skip_path" ]]; then
      continue
    fi
    if grep -Eq "$pattern" "$file"; then
      return 0
    fi
  done < <(find "$dir" -maxdepth 1 -type f -name '*.go' -print 2>/dev/null | sort)
  return 1
}

legacy_managed_go_reason() {
  local file="$1"
  case "$file" in
    internal/exporter/main.go)
      if feature_go_defines '^[[:space:]]*func[[:space:]]+Main[[:space:]]*\('; then
        echo "Main() is still defined in internal/exporter/feature.go"
        return 0
      fi
      ;;
    internal/exporter/identity.go)
      local reasons=()
      if feature_go_defines '^[[:space:]]*func[[:space:]]*\([^)]*\)[[:space:]]+FeatureName[[:space:]]*\('; then
        reasons+=("FeatureName()")
      fi
      if feature_go_defines '^[[:space:]]*func[[:space:]]*\([^)]*\)[[:space:]]+DefaultListenAddress[[:space:]]*\('; then
        reasons+=("DefaultListenAddress()")
      fi
      if [[ "${#reasons[@]}" -gt 0 ]]; then
        echo "${reasons[*]} still defined in internal/exporter/feature.go"
        return 0
      fi
      ;;
    internal/exporter/defaults.go)
      local reasons=()
      if exporter_go_defines_except "$file" '^[[:space:]]*defaultExporterName[[:space:]]*='; then
        reasons+=("defaultExporterName")
      fi
      if exporter_go_defines_except "$file" '^[[:space:]]*defaultExporterDescription[[:space:]]*='; then
        reasons+=("defaultExporterDescription")
      fi
      if exporter_go_defines_except "$file" '^[[:space:]]*defaultFeatureName[[:space:]]*='; then
        reasons+=("defaultFeatureName")
      fi
      if exporter_go_defines_except "$file" '^[[:space:]]*defaultMetricNamespace[[:space:]]*='; then
        reasons+=("defaultMetricNamespace")
      fi
      if exporter_go_defines_except "$file" '^[[:space:]]*defaultListenAddress[[:space:]]*='; then
        reasons+=("defaultListenAddress")
      fi
      if [[ "${#reasons[@]}" -gt 0 ]]; then
        echo "${reasons[*]} still defined outside internal/exporter/defaults.go"
        return 0
      fi
      ;;
    internal/exporter/standard_metrics.go)
      local reasons=()
      if exporter_go_defines_except "$file" '^[[:space:]]*metricBuildInfo[[:space:]]*='; then
        reasons+=("metricBuildInfo")
      fi
      if exporter_go_defines_except "$file" '^[[:space:]]*metricLastCollectionSuccess[[:space:]]*='; then
        reasons+=("metricLastCollectionSuccess")
      fi
      if exporter_go_defines_except "$file" '^[[:space:]]*metricLastCollectionTimestampSeconds[[:space:]]*='; then
        reasons+=("metricLastCollectionTimestampSeconds")
      fi
      if exporter_go_defines_except "$file" '^[[:space:]]*metricLastSuccessfulCollectionTimestampSeconds[[:space:]]*='; then
        reasons+=("metricLastSuccessfulCollectionTimestampSeconds")
      fi
      if [[ "${#reasons[@]}" -gt 0 ]]; then
        echo "${reasons[*]} still defined outside internal/exporter/standard_metrics.go"
        return 0
      fi
      ;;
    internal/exporter/feature_flags.go)
      if feature_go_defines '^[[:space:]]*func[[:space:]]*\([^)]*\)[[:space:]]+RegisterFlags[[:space:]]*\('; then
        echo "RegisterFlags() still defined in internal/exporter/feature.go"
        return 0
      fi
      ;;
    internal/exporter/feature_collectors.go)
      if feature_go_defines '^[[:space:]]*func[[:space:]]*\([^)]*\)[[:space:]]+RegisterCollectors[[:space:]]*\('; then
        echo "RegisterCollectors() still defined in internal/exporter/feature.go"
        return 0
      fi
      ;;
    internal/exporter/runtime_config.go)
      if feature_go_defines '^[[:space:]]*func[[:space:]]*\([^)]*\)[[:space:]]+RuntimeConfig[[:space:]]*\('; then
        echo "RuntimeConfig() still defined in internal/exporter/feature.go"
        return 0
      fi
      ;;
    internal/exporter/collector_types.go)
      local reasons=()
      local collector_file="$target_dir/internal/exporter/collector.go"
      if [[ -f "$collector_file" ]]; then
        if grep -Eq '^[[:space:]]*type[[:space:]]+Snapshot([[:space:]]|$)' "$collector_file"; then
          reasons+=("Snapshot")
        fi
        if grep -Eq '^[[:space:]]*type[[:space:]]+SnapshotGatherer([[:space:]]|$)' "$collector_file"; then
          reasons+=("SnapshotGatherer")
        fi
        if grep -Eq '^[[:space:]]*type[[:space:]]+Collector([[:space:]]|$)' "$collector_file"; then
          reasons+=("Collector")
        fi
      fi
      if [[ "${#reasons[@]}" -gt 0 ]]; then
        echo "${reasons[*]} still defined in internal/exporter/collector.go"
        return 0
      fi
      ;;
    internal/exporter/collector_metrics.go)
      local reasons=()
      local collector_file="$target_dir/internal/exporter/collector.go"
      if [[ -f "$collector_file" ]]; then
        if grep -Eq '^[[:space:]]*func[[:space:]]*\([^)]*\)[[:space:]]+describeSnapshotMetrics[[:space:]]*\(' "$collector_file"; then
          reasons+=("describeSnapshotMetrics()")
        fi
        if grep -Eq '^[[:space:]]*func[[:space:]]*\([^)]*\)[[:space:]]+collectSnapshotMetrics[[:space:]]*\(' "$collector_file"; then
          reasons+=("collectSnapshotMetrics()")
        fi
      fi
      if [[ "${#reasons[@]}" -gt 0 ]]; then
        echo "${reasons[*]} still defined in internal/exporter/collector.go"
        return 0
      fi
      ;;
    internal/exporter/collector_test_helpers_test.go)
      local reasons=()
      local collector_test_file="$target_dir/internal/exporter/collector_test.go"
      if [[ -f "$collector_test_file" ]]; then
        if grep -Eq '^[[:space:]]*type[[:space:]]+fakeSnapshotter([[:space:]]|$)' "$collector_test_file"; then
          reasons+=("fakeSnapshotter")
        fi
        if grep -Eq '^[[:space:]]*func[[:space:]]+newFakeSnapshotter[[:space:]]*\(' "$collector_test_file"; then
          reasons+=("newFakeSnapshotter()")
        fi
        if grep -Eq '^[[:space:]]*func[[:space:]]*\([^)]*\)[[:space:]]+Snapshot[[:space:]]*\(' "$collector_test_file"; then
          reasons+=("fakeSnapshotter.Snapshot()")
        fi
        if grep -Eq '^[[:space:]]*func[[:space:]]*\([^)]*\)[[:space:]]+set[[:space:]]*\(' "$collector_test_file"; then
          reasons+=("fakeSnapshotter.set()")
        fi
      fi
      if [[ "${#reasons[@]}" -gt 0 ]]; then
        echo "${reasons[*]} still defined in internal/exporter/collector_test.go"
        return 0
      fi
      ;;
    internal/exporter/collector_snapshot_test.go)
      local collector_test_file="$target_dir/internal/exporter/collector_test.go"
      if [[ -f "$collector_test_file" ]] && grep -Eq '^[[:space:]]*func[[:space:]]+TestCollectorExportsSnapshot[[:space:]]*\(' "$collector_test_file"; then
        echo "TestCollectorExportsSnapshot() still defined in internal/exporter/collector_test.go"
        return 0
      fi
      ;;
    internal/exporter/collector_refresh_test.go)
      local collector_test_file="$target_dir/internal/exporter/collector_test.go"
      if [[ -f "$collector_test_file" ]] && grep -Eq '^[[:space:]]*func[[:space:]]+TestCollectorBackgroundRefresh[^[:space:]]*[[:space:]]*\(' "$collector_test_file"; then
        echo "TestCollectorBackgroundRefresh*() still defined in internal/exporter/collector_test.go"
        return 0
      fi
      ;;
    internal/exporter/collector_defaults_test.go)
      local reasons=()
      local collector_test_file="$target_dir/internal/exporter/collector_test.go"
      if [[ -f "$collector_test_file" ]]; then
        if grep -Eq '^[[:space:]]*func[[:space:]]+TestCollectorDefaults[^[:space:]]*[[:space:]]*\(' "$collector_test_file"; then
          reasons+=("TestCollectorDefaults*()")
        fi
        if grep -Eq '^[[:space:]]*func[[:space:]]+TestCollectorUsesDefaultSnapshotter[[:space:]]*\(' "$collector_test_file"; then
          reasons+=("TestCollectorUsesDefaultSnapshotter()")
        fi
      fi
      if [[ "${#reasons[@]}" -gt 0 ]]; then
        echo "${reasons[*]} still defined in internal/exporter/collector_test.go"
        return 0
      fi
      ;;
    internal/exporter/feature_test_helpers_test.go)
      local feature_test_file="$target_dir/internal/exporter/feature_test.go"
      if [[ -f "$feature_test_file" ]] && grep -Eq '^[[:space:]]*func[[:space:]]+testFeatureContext[[:space:]]*\(' "$feature_test_file"; then
        echo "testFeatureContext() still defined in internal/exporter/feature_test.go"
        return 0
      fi
      ;;
    internal/exporter/feature_integration_test_helpers_test.go)
      local reasons=()
      local feature_integration_test_file="$target_dir/internal/exporter/feature_integration_test.go"
      if [[ -f "$feature_integration_test_file" ]]; then
        if grep -Eq '^[[:space:]]*func[[:space:]]+newTestHandler[[:space:]]*\(' "$feature_integration_test_file"; then
          reasons+=("newTestHandler()")
        fi
        if grep -Eq '^[[:space:]]*func[[:space:]]+waitForHandlerMetrics[[:space:]]*\(' "$feature_integration_test_file"; then
          reasons+=("waitForHandlerMetrics()")
        fi
      fi
      if [[ "${#reasons[@]}" -gt 0 ]]; then
        echo "${reasons[*]} still defined in internal/exporter/feature_integration_test.go"
        return 0
      fi
      ;;
    internal/exporter/feature_flags_test.go)
      local feature_test_file="$target_dir/internal/exporter/feature_test.go"
      if [[ -f "$feature_test_file" ]] && grep -Eq '^[[:space:]]*func[[:space:]]+TestFeatureRegistersAndParsesFlags[[:space:]]*\(' "$feature_test_file"; then
        echo "TestFeatureRegistersAndParsesFlags() still defined in internal/exporter/feature_test.go"
        return 0
      fi
      ;;
    internal/exporter/feature_collectors_test.go)
      local reasons=()
      local feature_test_file="$target_dir/internal/exporter/feature_test.go"
      if [[ -f "$feature_test_file" ]]; then
        if grep -Eq '^[[:space:]]*func[[:space:]]+TestFeatureRegistersCollector[[:space:]]*\(' "$feature_test_file"; then
          reasons+=("TestFeatureRegistersCollector()")
        fi
        if grep -Eq '^[[:space:]]*func[[:space:]]+TestFeatureReportsCollectorRegistrationError[[:space:]]*\(' "$feature_test_file"; then
          reasons+=("TestFeatureReportsCollectorRegistrationError()")
        fi
      fi
      if [[ "${#reasons[@]}" -gt 0 ]]; then
        echo "${reasons[*]} still defined in internal/exporter/feature_test.go"
        return 0
      fi
      ;;
    internal/exporter/runtime_config_test.go)
      local feature_test_file="$target_dir/internal/exporter/feature_test.go"
      if [[ -f "$feature_test_file" ]] && grep -Eq '^[[:space:]]*func[[:space:]]+TestFeatureRuntimeConfig[^[:space:]]*[[:space:]]*\(' "$feature_test_file"; then
        echo "TestFeatureRuntimeConfig*() still defined in internal/exporter/feature_test.go"
        return 0
      fi
      ;;
    internal/exporter/identity_test.go)
      local feature_test_file="$target_dir/internal/exporter/feature_test.go"
      if [[ -f "$feature_test_file" ]] && grep -Eq '^[[:space:]]*func[[:space:]]+TestFeatureMetadata[[:space:]]*\(' "$feature_test_file"; then
        echo "TestFeatureMetadata() still defined in internal/exporter/feature_test.go"
        return 0
      fi
      ;;
    internal/exporter/snapshot.go)
      local reasons=()
      local collector_file="$target_dir/internal/exporter/collector.go"
      if [[ -f "$collector_file" ]]; then
        if grep -Eq '^[[:space:]]*func[[:space:]]*\(SnapshotGatherer\)[[:space:]]+Snapshot[[:space:]]*\(' "$collector_file"; then
          reasons+=("SnapshotGatherer.Snapshot()")
        fi
        if grep -Eq '^[[:space:]]*func[[:space:]]+snapshotStatus[[:space:]]*\(' "$collector_file"; then
          reasons+=("snapshotStatus()")
        fi
        if grep -Eq '^[[:space:]]*func[[:space:]]+logSnapshotError[[:space:]]*\(' "$collector_file"; then
          reasons+=("logSnapshotError()")
        fi
      fi
      if [[ "${#reasons[@]}" -gt 0 ]]; then
        echo "${reasons[*]} still defined in internal/exporter/collector.go"
        return 0
      fi
      ;;
  esac
  return 1
}

if [[ -z "$go_module" ]]; then
  go_module="$(detect_module)"
fi
if [[ -z "$project_name" ]]; then
  project_name="$(detect_project_name)"
fi
if [[ -z "$project_name" ]]; then
  project_name="$(basename "$target_dir")"
fi
if [[ -z "$project_desc" ]]; then
  project_desc="$(detect_exporter_description)"
fi
if [[ -z "$project_desc" ]]; then
  project_desc="$(detect_readme_h1)"
fi
if [[ -z "$project_desc" ]]; then
  project_desc="$project_name"
fi
if [[ -z "$feature_name" ]]; then
  feature_name="$(detect_feature_name)"
fi
if [[ -z "$feature_name" ]]; then
  stem="${project_name#prometheus-}"
  stem="${stem%-exporter}"
  feature_name="${stem//-/_}"
fi
if [[ -z "$metric_namespace" ]]; then
  metric_namespace="$(detect_namespace)"
fi
if [[ -z "$default_port" ]]; then
  default_port="$(detect_default_port)"
fi
if [[ -z "$default_port" ]]; then
  default_port="9888"
fi
if [[ -z "$feature_config_file" ]]; then
  feature_config_file="$(detect_makefile_mk_var "FEATURE_CONFIG_FILE")"
fi
if [[ -z "$feature_config_file" ]]; then
  feature_config_file="${project_name}.yml"
fi
if [[ -z "$docker_smoke_metric" ]]; then
  docker_smoke_metric="$(detect_docker_smoke_metric)"
fi
if [[ -z "$docker_smoke_metric" ]]; then
  docker_smoke_metric='$(FEATURE_NAME)_example_value 1'
fi
if [[ -z "$docker_smoke_run_options" ]]; then
  docker_smoke_run_options="$(detect_makefile_mk_var "DOCKER_SMOKE_RUN_OPTIONS")"
fi
if [[ -z "$docker_smoke_exporter_args" ]]; then
  docker_smoke_exporter_args="$(detect_makefile_mk_var "DOCKER_SMOKE_EXPORTER_ARGS")"
fi
if [[ -z "$docker_smoke_extra_metrics" ]]; then
  docker_smoke_extra_metrics="$(detect_makefile_mk_var "DOCKER_SMOKE_EXTRA_METRICS")"
fi

resolved_managed_files=()
for file in "${managed_files[@]}"; do
  file="${file//__FEATURE_NAME__/$feature_name}"
  resolved_managed_files+=("${file//__FEATURE_CONFIG_FILE__/$feature_config_file}")
done
managed_files=("${resolved_managed_files[@]}")

resolved_obsolete_files=()
if [[ "${#managed_obsolete_files[@]}" -gt 0 ]]; then
  for file in "${managed_obsolete_files[@]}"; do
    file="${file//__FEATURE_NAME__/$feature_name}"
    resolved_obsolete_files+=("${file//__FEATURE_CONFIG_FILE__/$feature_config_file}")
  done
  managed_obsolete_files=("${resolved_obsolete_files[@]}")
else
  managed_obsolete_files=()
fi

rendered_dir="$(mktemp -d)"
trap 'rm -rf "$rendered_dir"' EXIT

"$repo_dir/scripts/render.sh" \
  --project-name "$project_name" \
  --module "${go_module:-$project_name}" \
  --description "$project_desc" \
  --feature-name "$feature_name" \
  --namespace "$metric_namespace" \
  --port "$default_port" \
  --feature-config-file "$feature_config_file" \
  --docker-smoke-metric "$docker_smoke_metric" \
  --docker-smoke-run-options "$docker_smoke_run_options" \
  --docker-smoke-exporter-args "$docker_smoke_exporter_args" \
  --docker-smoke-extra-metrics "$docker_smoke_extra_metrics" \
  --target-dir "$rendered_dir" >/dev/null

format_rendered_go() {
  local gofmt_bin="${GOFMT:-gofmt}"
  if ! command -v "$gofmt_bin" >/dev/null 2>&1; then
    if [[ -x "$HOME/sdk/go1.26.3/bin/gofmt" ]]; then
      gofmt_bin="$HOME/sdk/go1.26.3/bin/gofmt"
    else
      return 0
    fi
  fi

  while IFS= read -r file; do
    "$gofmt_bin" -w "$file"
  done < <(find "$rendered_dir" -type f -name '*.go' -print 2>/dev/null | sort)
}

format_rendered_go

if [[ "$all_files" -eq 1 ]]; then
  managed_files=()
  while IFS= read -r file; do
    managed_files+=("$file")
  done < <(
    cd "$rendered_dir"
    find . -type f -print | sed 's#^\./##' | sort
  )
fi

symbol_diff_go() {
  local target_file="$1"
  local rendered_file="$2"
  local file="$3"
  local go_bin="${GO:-go}"
  if ! command -v "$go_bin" >/dev/null 2>&1; then
    if [[ -x "$HOME/sdk/go1.26.3/bin/go" ]]; then
      go_bin="$HOME/sdk/go1.26.3/bin/go"
    else
      return 1
    fi
  fi
  (
    cd "$repo_dir"
    "$go_bin" run ./scripts/go-symbol-diff.go \
    --left-label "target:$file" \
    --right-label "scaffold:$file" \
    "$target_file" \
    "$rendered_file"
  )
}

target_framework_version="$(detect_go_mod_required_version "$target_dir/go.mod" "$framework_module")"
current_framework_version="$(detect_go_mod_required_version "$repo_dir/template/go.mod" "$framework_module")"
framework_issue=""
framework_drift=0
if [[ -n "$current_framework_version" && -z "$target_framework_version" ]]; then
  framework_issue="MISSING framework $framework_module: target go.mod does not require it; scaffold expects $current_framework_version"
  framework_drift=1
elif [[ -n "$current_framework_version" && -n "$target_framework_version" ]]; then
  framework_compare="$(compare_versions "$target_framework_version" "$current_framework_version")"
  if [[ "$framework_compare" == "older" ]]; then
    framework_issue="OUTDATED framework $framework_module: target uses $target_framework_version; scaffold expects $current_framework_version"
    framework_drift=1
  elif [[ "$framework_compare" == "newer" ]]; then
    framework_issue="NEWER framework $framework_module: target uses $target_framework_version; scaffold expects $current_framework_version"
  elif [[ "$framework_compare" == "unknown" && "$target_framework_version" != "$current_framework_version" ]]; then
    framework_issue="DIFFERENT framework $framework_module: target uses $target_framework_version; scaffold expects $current_framework_version"
  fi
fi

printf 'Scaffold metadata:\n'
printf '  target:       %s\n' "$target_dir"
printf '  project-name: %s\n' "$project_name"
printf '  module:       %s\n' "${go_module:-$project_name}"
printf '  description:  %s\n' "$project_desc"
printf '  feature-name: %s\n' "$feature_name"
printf '  namespace:    %s\n' "$metric_namespace"
printf '  port:         %s\n' "$default_port"
printf '  config-file:  %s\n' "$feature_config_file"
printf '  framework:    %s (scaffold: %s)\n' "${target_framework_version:-<missing>}" "${current_framework_version:-<unknown>}"
printf '  smoke-metric: %s\n' "$docker_smoke_metric"
printf '  smoke-run-options: %s\n' "${docker_smoke_run_options:-<empty>}"
printf '  smoke-exporter-args: %s\n' "${docker_smoke_exporter_args:-<empty>}"
printf '  smoke-extra-metrics: %s\n' "${docker_smoke_extra_metrics:-<empty>}"

if [[ "$mode" == "sync" && "$allow_dirty" -ne 1 ]] && git -C "$target_dir" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  dirty="$(git -C "$target_dir" status --short -- "${managed_files[@]}" "${managed_obsolete_files[@]}")"
  if [[ -n "$dirty" ]]; then
    echo
    echo "managed files already have git changes; commit/stash them or pass --allow-dirty:" >&2
    echo "$dirty" >&2
    exit 1
  fi
fi

drift=0
echo
if [[ -n "$framework_issue" ]]; then
  echo "$framework_issue"
  if [[ "$framework_drift" -ne 0 ]]; then
    drift=1
  fi
fi

for file in "${managed_files[@]}"; do
  rendered_file="$rendered_dir/$file"
  target_file="$target_dir/$file"

  legacy_reason="$(legacy_managed_go_reason "$file" || true)"
  if [[ -n "$legacy_reason" ]]; then
    echo "LEGACY $file ($legacy_reason; migrate before syncing this file)"
    drift=1
    continue
  fi

  if [[ ! -e "$rendered_file" ]]; then
    echo "SKIP    $file (not rendered)"
    continue
  fi

  if [[ "$mode" == "sync" ]]; then
    if [[ -e "$target_file" ]] && cmp -s "$rendered_file" "$target_file"; then
      echo "OK      $file"
      continue
    fi
    mkdir -p "$(dirname "$target_file")"
    cp "$rendered_file" "$target_file"
    echo "SYNCED  $file"
    continue
  fi

  if [[ ! -e "$target_file" ]]; then
    echo "MISSING $file"
    drift=1
    continue
  fi
  if cmp -s "$rendered_file" "$target_file"; then
    echo "OK      $file"
    continue
  fi

  echo "DRIFT   $file"
  if [[ "$symbol_diff" -eq 1 && "$file" == *.go ]]; then
    symbol_diff_go "$target_file" "$rendered_file" "$file" || diff -u "$target_file" "$rendered_file" || true
  else
    diff -u "$target_file" "$rendered_file" || true
  fi
  drift=1
done

if [[ "${#managed_obsolete_files[@]}" -gt 0 ]]; then
  for file in "${managed_obsolete_files[@]}"; do
    target_file="$target_dir/$file"
    if [[ ! -e "$target_file" ]]; then
      continue
    fi
    if [[ "$mode" == "sync" ]]; then
      rm -f "$target_file"
      echo "REMOVED $file"
      continue
    fi
    echo "OBSOLETE $file (removed from current scaffold decomposition)"
    drift=1
  done
fi

if [[ "$mode" == "check" && "$drift" -ne 0 ]]; then
  echo
  echo "scaffold drift found; rerun with --sync to update managed files"
  exit 1
fi
