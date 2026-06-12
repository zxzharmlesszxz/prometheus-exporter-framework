#!/usr/bin/env bash
set -euo pipefail
export LC_ALL=C

usage() {
  cat <<'USAGE'
Usage:
  scripts/render.sh \
    --project-name prometheus-demo-exporter \
    --module prometheus-demo-exporter \
    --description "Prometheus Demo Exporter" \
    --feature-name demo \
    --namespace demo_exporter \
    --port 9888 \
    --feature-config-file prometheus-demo-exporter.yml \
    --docker-smoke-metric '$(FEATURE_NAME)_example_value 1' \
    --docker-smoke-run-options '-v "$(CURDIR)/$(FEATURE_CONFIG_PATH):$(FEATURE_CONFIG_CONTAINER_PATH):ro"' \
    --docker-smoke-exporter-args '' \
    --docker-smoke-extra-metrics '' \
    --target-dir /tmp/prometheus-demo-exporter

Required:
  --project-name
  --target-dir

Optional:
  --module       Defaults to --project-name.
  --description Defaults to --project-name.
  --feature-name Defaults to project name without prometheus- prefix and -exporter suffix, with '-' replaced by '_'.
  --namespace   Defaults to <feature-name>_exporter.
  --port        Defaults to 9888.
  --feature-config-file
               Defaults to <project-name>.yml.
  --docker-smoke-metric
               Defaults to '$(FEATURE_NAME)_example_value 1'.
  --docker-smoke-run-options
               Extra options passed to `docker run` before the image.
  --docker-smoke-exporter-args
               Defaults to '--$(FEATURE_NAME).config-file=$(FEATURE_CONFIG_CONTAINER_PATH)'.
  --docker-smoke-extra-metrics
               Additional metric assertions separated by '|'.
USAGE
}

project_name=""
go_module=""
project_desc=""
feature_name=""
metric_namespace=""
default_port="9888"
feature_config_file=""
docker_smoke_metric='$(FEATURE_NAME)_example_value 1'
docker_smoke_run_options='-v "$(CURDIR)/$(FEATURE_CONFIG_PATH):$(FEATURE_CONFIG_CONTAINER_PATH):ro"'
docker_smoke_exporter_args='--$(FEATURE_NAME).config-file=$(FEATURE_CONFIG_CONTAINER_PATH)'
docker_smoke_extra_metrics=""
target_dir=""

while [[ $# -gt 0 ]]; do
  case "$1" in
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
    --target-dir)
      target_dir="${2:-}"
      shift 2
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

if [[ -z "$project_name" || -z "$target_dir" ]]; then
  usage >&2
  exit 1
fi

go_module="${go_module:-$project_name}"
project_desc="${project_desc:-$project_name}"
if [[ -z "$feature_name" ]]; then
  stem="${project_name#prometheus-}"
  stem="${stem%-exporter}"
  feature_name="${stem//-/_}"
fi
metric_namespace="${metric_namespace:-${feature_name}_exporter}"
feature_config_file="${feature_config_file:-${project_name}.yml}"

if [[ ! "$feature_name" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]]; then
  echo "--feature-name must be a valid Go/Prometheus identifier fragment: $feature_name" >&2
  exit 1
fi
if [[ ! "$metric_namespace" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]]; then
  echo "--namespace must be a valid Prometheus metric namespace: $metric_namespace" >&2
  exit 1
fi
if [[ ! "$default_port" =~ ^[0-9]+$ ]]; then
  echo "--port must be numeric: $default_port" >&2
  exit 1
fi
if [[ -z "$docker_smoke_metric" ]]; then
  echo "--docker-smoke-metric must not be empty" >&2
  exit 1
fi

if [[ -e "$target_dir" && -n "$(find "$target_dir" -mindepth 1 -maxdepth 1 -print -quit)" ]]; then
  echo "target dir exists and is not empty: $target_dir" >&2
  exit 1
fi

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_dir="$(cd "$script_dir/.." && pwd)"
template_dir="$repo_dir/template"

mkdir -p "$target_dir"
cp -R "$template_dir/." "$target_dir/"

export PROJECT_NAME="$project_name"
export GO_MODULE="$go_module"
export PROJECT_DESC="$project_desc"
export FEATURE_NAME="$feature_name"
export METRIC_NAMESPACE="$metric_namespace"
export DEFAULT_PORT="$default_port"
export FEATURE_CONFIG_FILE="$feature_config_file"
export DOCKER_SMOKE_METRIC="$docker_smoke_metric"
export DOCKER_SMOKE_RUN_OPTIONS="$docker_smoke_run_options"
export DOCKER_SMOKE_EXPORTER_ARGS="$docker_smoke_exporter_args"
export DOCKER_SMOKE_EXTRA_METRICS="$docker_smoke_extra_metrics"

sed_replacement() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//&/\\&}"
  value="${value//|/\\|}"
  printf '%s' "$value"
}

project_name_sed="$(sed_replacement "$PROJECT_NAME")"
go_module_sed="$(sed_replacement "$GO_MODULE")"
project_desc_sed="$(sed_replacement "$PROJECT_DESC")"
feature_name_sed="$(sed_replacement "$FEATURE_NAME")"
metric_namespace_sed="$(sed_replacement "$METRIC_NAMESPACE")"
default_port_sed="$(sed_replacement "$DEFAULT_PORT")"
feature_config_file_sed="$(sed_replacement "$FEATURE_CONFIG_FILE")"
docker_smoke_metric_sed="$(sed_replacement "$DOCKER_SMOKE_METRIC")"
docker_smoke_run_options_sed="$(sed_replacement "$DOCKER_SMOKE_RUN_OPTIONS")"
docker_smoke_exporter_args_sed="$(sed_replacement "$DOCKER_SMOKE_EXPORTER_ARGS")"
docker_smoke_extra_metrics_sed="$(sed_replacement "$DOCKER_SMOKE_EXTRA_METRICS")"

find "$target_dir" -type f -print0 | while IFS= read -r -d '' file; do
  sed -i.bak \
    -e "s|__PROJECT_NAME__|$project_name_sed|g" \
    -e "s|__GO_MODULE__|$go_module_sed|g" \
    -e "s|__PROJECT_DESC__|$project_desc_sed|g" \
    -e "s|__FEATURE_NAME__|$feature_name_sed|g" \
    -e "s|__METRIC_NAMESPACE__|$metric_namespace_sed|g" \
    -e "s|__DEFAULT_PORT__|$default_port_sed|g" \
    -e "s|__FEATURE_CONFIG_FILE__|$feature_config_file_sed|g" \
    -e "s|__DOCKER_SMOKE_METRIC__|$docker_smoke_metric_sed|g" \
    -e "s|__DOCKER_SMOKE_RUN_OPTIONS__|$docker_smoke_run_options_sed|g" \
    -e "s|__DOCKER_SMOKE_EXPORTER_ARGS__|$docker_smoke_exporter_args_sed|g" \
    -e "s|__DOCKER_SMOKE_EXTRA_METRICS__|$docker_smoke_extra_metrics_sed|g" \
    "$file"
  rm -f "$file.bak"
done

find "$target_dir" -depth -name '*__PROJECT_NAME__*' -print | while IFS= read -r path; do
  new_path="${path//__PROJECT_NAME__/$project_name}"
  mv "$path" "$new_path"
done

find "$target_dir" -depth -name '*__FEATURE_CONFIG_FILE__*' -print | while IFS= read -r path; do
  new_path="${path//__FEATURE_CONFIG_FILE__/$feature_config_file}"
  mv "$path" "$new_path"
done

find "$target_dir" -depth -name '*__FEATURE_NAME__*' -print | while IFS= read -r path; do
  new_path="${path//__FEATURE_NAME__/$feature_name}"
  mv "$path" "$new_path"
done

cat <<EOF
Rendered $project_name into $target_dir

Next:
  cd "$target_dir"
  go mod tidy
  make go-check
EOF
