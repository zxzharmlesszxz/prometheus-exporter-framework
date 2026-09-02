#!/usr/bin/env bash
set -euo pipefail
export LC_ALL=C

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_dir="$(cd "$script_dir/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

target_dir="$tmp/prometheus-demo-exporter"

if make -C "$repo_dir/template" docker-smoke-build >"$tmp/template-docker-smoke.out" 2>&1; then
  echo "raw scaffold template docker smoke unexpectedly passed" >&2
  exit 1
fi
grep -F "scaffold template must be rendered before running build targets" "$tmp/template-docker-smoke.out" >/dev/null

"$repo_dir/scripts/scaffold-drift.sh" --list-files >"$tmp/list-files.out"
grep -Fx "Makefile" "$tmp/list-files.out" >/dev/null
grep -Fx ".dockerignore" "$tmp/list-files.out" >/dev/null
grep -Fx "internal/__FEATURE_NAME__/scaffold_feature.go" "$tmp/list-files.out" >/dev/null

"$repo_dir/scripts/render.sh" \
  --project-name prometheus-demo-exporter \
  --module github.com/zxzharmlesszxz/prometheus-demo-exporter \
  --description "Prometheus Demo Exporter" \
  --feature-name demo \
  --feature-namespace demo \
  --namespace demo_exporter \
  --port 9888 \
  --feature-config-file prometheus-demo-exporter.yml \
  --target-dir "$target_dir" >/dev/null

if grep -R -n -E '__[A-Z0-9_]+__' "$target_dir"; then
  echo "rendered exporter still contains scaffold placeholders" >&2
  exit 1
fi

printf '%s\n' \
  'include Makefile' \
  'print-vars:' \
  '	@printf "%s\n" "DOCKER_PROJECT_NAME=$(DOCKER_PROJECT_NAME)" "COMPOSE_PROJECT_NAME=$(COMPOSE_PROJECT_NAME)" "COMPOSE_FEATURE_NAME=$(COMPOSE_FEATURE_NAME)" "COMPOSE_EXPORTER_PORT=$(COMPOSE_EXPORTER_PORT)"' \
  >"$tmp/rendered-vars.mk"
make -C "$target_dir" --no-print-directory -f "$tmp/rendered-vars.mk" print-vars >"$tmp/rendered-make-vars.out"
grep -Fx "DOCKER_PROJECT_NAME=prometheus-demo-exporter" "$tmp/rendered-make-vars.out" >/dev/null
grep -Fx "COMPOSE_PROJECT_NAME=prometheus-demo-exporter" "$tmp/rendered-make-vars.out" >/dev/null
grep -Fx "COMPOSE_FEATURE_NAME=demo" "$tmp/rendered-make-vars.out" >/dev/null
grep -Fx "COMPOSE_EXPORTER_PORT=9888" "$tmp/rendered-make-vars.out" >/dev/null

grep -F "uses: zxzharmlesszxz/prometheus-exporter-framework/.github/workflows/exporter-ci.yml@v" \
  "$target_dir/.github/workflows/ci.yml" >/dev/null

git -C "$target_dir" init -q
git -C "$target_dir" add .
git -C "$target_dir" \
  -c user.name=scaffold-test \
  -c user.email=scaffold-test@example.invalid \
  commit -qm initial

"$repo_dir/scripts/scaffold-drift.sh" --target-dir "$target_dir" --file Makefile >/dev/null
"$repo_dir/scripts/scaffold-drift.sh" --target-dir "$target_dir" --sync --file Makefile >/dev/null
"$repo_dir/scripts/scaffold-drift.sh" --target-dir "$target_dir" --all-files >/dev/null

printf '\n# local drift\n' >> "$target_dir/Makefile"

if "$repo_dir/scripts/scaffold-drift.sh" --target-dir "$target_dir" --file Makefile >"$tmp/drift.out" 2>&1; then
  echo "drift-check passed after managed Makefile drift" >&2
  exit 1
fi
grep -F "DRIFT   Makefile" "$tmp/drift.out" >/dev/null

"$repo_dir/scripts/scaffold-drift.sh" \
  --target-dir "$target_dir" \
  --sync \
  --allow-dirty \
  --file Makefile >/dev/null

"$repo_dir/scripts/scaffold-drift.sh" --target-dir "$target_dir" --file Makefile >/dev/null

awk '{
  gsub("return feature.NewFeature\\(featurekit.SpecOptions\\{FeatureName: framework.InjectedFeatureName\\(\\)\\}\\)", "return newFeature(framework.InjectedFeatureName())")
  print
}' "$target_dir/internal/exporter/scaffold_exporter.go" >"$tmp/scaffold_exporter.go"
mv "$tmp/scaffold_exporter.go" "$target_dir/internal/exporter/scaffold_exporter.go"

if "$repo_dir/scripts/scaffold-drift.sh" \
  --target-dir "$target_dir" \
  --symbol-diff \
  --file internal/exporter/scaffold_exporter.go >"$tmp/symbol-diff.out" 2>&1; then
  echo "symbol drift-check passed after scaffold_exporter.go drift" >&2
  exit 1
fi
grep -F "DRIFT   internal/exporter/scaffold_exporter.go" "$tmp/symbol-diff.out" >/dev/null
grep -F "SYMBOL DIFF func NewFeature" "$tmp/symbol-diff.out" >/dev/null

"$repo_dir/scripts/scaffold-drift.sh" \
  --target-dir "$target_dir" \
  --sync \
  --allow-dirty \
  --file internal/exporter/scaffold_exporter.go >/dev/null

mkdir -p "$target_dir/smoke"
printf 'package smoke\n' >"$target_dir/smoke/binary_test.go"
if "$repo_dir/scripts/scaffold-drift.sh" \
  --target-dir "$target_dir" \
  --file smoke/scaffold_binary_test.go >"$tmp/obsolete.out" 2>&1; then
  echo "drift-check passed with obsolete smoke/binary_test.go" >&2
  exit 1
fi
grep -F "OBSOLETE smoke/binary_test.go" "$tmp/obsolete.out" >/dev/null
"$repo_dir/scripts/scaffold-drift.sh" \
  --target-dir "$target_dir" \
  --sync \
  --allow-dirty \
  --file smoke/scaffold_binary_test.go >/dev/null
test ! -e "$target_dir/smoke/binary_test.go"

awk '
  $1 == "github.com/zxzharmlesszxz/prometheus-exporter-framework" {
    $2 = "v0.0.1"
  }
  { print }
' "$target_dir/go.mod" >"$tmp/go.mod"
mv "$tmp/go.mod" "$target_dir/go.mod"
if "$repo_dir/scripts/scaffold-drift.sh" --target-dir "$target_dir" --file Makefile >"$tmp/outdated.out" 2>&1; then
  echo "drift-check passed with outdated framework dependency" >&2
  exit 1
fi
grep -F "OUTDATED framework github.com/zxzharmlesszxz/prometheus-exporter-framework: target uses v0.0.1" "$tmp/outdated.out" >/dev/null
