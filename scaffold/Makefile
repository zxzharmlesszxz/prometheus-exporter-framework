GO ?= go
GOFMT ?= gofmt
FRAMEWORK_MODULE ?= github.com/zxzharmlesszxz/prometheus-exporter-framework
FRAMEWORK_REPLACE ?= $(abspath ..)

PROJECT_NAME ?=
GO_MODULE ?=
PROJECT_DESC ?=
FEATURE_NAME ?=
FEATURE_NAMESPACE ?=
METRIC_NAMESPACE ?=
DEFAULT_PORT ?=
FEATURE_CONFIG_FILE ?=
TARGET_DIR ?= $(if $(PROJECT_NAME),rendered/$(PROJECT_NAME),)
FILE ?=
SKIP_FILE ?=
ALLOW_DIRTY ?= 0
SYMBOL_DIFF ?= 0
ALL_FILES ?= 0

CHECK_PROJECT_NAME ?= prometheus-demo-exporter
CHECK_GO_MODULE ?= github.com/zxzharmlesszxz/prometheus-demo-exporter
CHECK_PROJECT_DESC ?= Prometheus Demo Exporter
CHECK_FEATURE_NAME ?= demo
CHECK_FEATURE_NAMESPACE ?= demo
CHECK_METRIC_NAMESPACE ?= demo_exporter
CHECK_DEFAULT_PORT ?= 9888
CHECK_FEATURE_CONFIG_FILE ?= $(CHECK_PROJECT_NAME).yml

DRIFT_ALLOW_DIRTY := $(if $(filter 1 true yes,$(ALLOW_DIRTY)),--allow-dirty,)
DRIFT_FILE_ARGS := $(foreach file,$(FILE),--file "$(file)")
DRIFT_SKIP_FILE_ARGS := $(foreach file,$(SKIP_FILE),--skip-file "$(file)")
DRIFT_SYMBOL_DIFF := $(if $(filter 1 true yes,$(SYMBOL_DIFF)),--symbol-diff,)
DRIFT_ALL_FILES := $(if $(filter 1 true yes,$(ALL_FILES)),--all-files,)
DOCKER_SMOKE_METRIC_ARG := $(if $(filter undefined,$(origin DOCKER_SMOKE_METRIC)),,--docker-smoke-metric "$(DOCKER_SMOKE_METRIC)")
DOCKER_SMOKE_RUN_OPTIONS_ARG := $(if $(filter undefined,$(origin DOCKER_SMOKE_RUN_OPTIONS)),,--docker-smoke-run-options "$(DOCKER_SMOKE_RUN_OPTIONS)")
DOCKER_SMOKE_EXPORTER_ARGS_ARG := $(if $(filter undefined,$(origin DOCKER_SMOKE_EXPORTER_ARGS)),,--docker-smoke-exporter-args "$(DOCKER_SMOKE_EXPORTER_ARGS)")
DOCKER_SMOKE_EXTRA_METRICS_ARG := $(if $(filter undefined,$(origin DOCKER_SMOKE_EXTRA_METRICS)),,--docker-smoke-extra-metrics "$(DOCKER_SMOKE_EXTRA_METRICS)")

.PHONY: help scripts-check tools-check symbol-diff-check render-check check check-local check-pinned template-mod-tidy template-deps-update new-exporter drift-check drift-check-all drift-sync drift-list-files clean require-project-name require-target-dir

help: ## Show available make targets.
	@printf "\033[33mUsage:\033[0m\n"
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "};{printf "\033[36m%-24s\033[0m %s\n", $$1, $$2}'

scripts-check: ## Check scaffold shell scripts for syntax errors.
	@sh -n scripts/render.sh
	@bash -n scripts/scaffold-drift.sh
	@bash scripts/scaffold-drift_test.sh

tools-check: ## Check local tools required by generated Go checks.
	@command -v "$(GO)" >/dev/null 2>&1 || { echo "GO command not found: $(GO)" >&2; exit 127; }
	@command -v "$(GOFMT)" >/dev/null 2>&1 || { echo "GOFMT command not found: $(GOFMT)" >&2; exit 127; }

symbol-diff-check: tools-check ## Check the Go symbol-level diff helper.
	@$(GO) run ./scripts/go-symbol-diff.go -- template/internal/__FEATURE_NAME__/feature_snapshotter_ext.go template/internal/__FEATURE_NAME__/feature_snapshotter_ext.go >/dev/null

render-check: scripts-check symbol-diff-check ## Render a demo exporter against the local framework checkout and run its Go-only checks.
	@set -e; \
	tmp="$$(mktemp -d)"; \
	trap 'rm -rf "$$tmp"' EXIT; \
	scripts/render.sh \
		--project-name "$(CHECK_PROJECT_NAME)" \
		--module "$(CHECK_GO_MODULE)" \
		--description "$(CHECK_PROJECT_DESC)" \
		--feature-name "$(CHECK_FEATURE_NAME)" \
		--feature-namespace "$(CHECK_FEATURE_NAMESPACE)" \
		--namespace "$(CHECK_METRIC_NAMESPACE)" \
		--port "$(CHECK_DEFAULT_PORT)" \
		--feature-config-file "$(CHECK_FEATURE_CONFIG_FILE)" \
		--target-dir "$$tmp"; \
	if grep -R -n -E '__[A-Z0-9_]+__' "$$tmp"; then \
		echo "rendered exporter still contains scaffold placeholders" >&2; \
		exit 1; \
	fi; \
	scripts/scaffold-drift.sh --target-dir "$$tmp"; \
	cd "$$tmp"; \
	if [ -n "$(FRAMEWORK_REPLACE)" ]; then \
		printf "\033[31mFramework source: local replace -> %s\033[0m\n" "$(FRAMEWORK_REPLACE)"; \
		$(GO) mod edit -replace "$(FRAMEWORK_MODULE)=$(FRAMEWORK_REPLACE)"; \
	else \
		printf "\033[31mFramework source: pinned dependency from scaffold/template/go.mod\033[0m\n"; \
	fi; \
	$(GO) mod tidy; \
	$(GO) list -m "$(FRAMEWORK_MODULE)"; \
	modcheck="$$tmp/.modcheck"; \
	mkdir -p "$$modcheck"; \
	cp go.mod "$$modcheck/go.mod"; \
	cp go.sum "$$modcheck/go.sum"; \
	$(GO) mod tidy; \
	diff -u "$$modcheck/go.mod" go.mod; \
	diff -u "$$modcheck/go.sum" go.sum; \
	$(MAKE) go-check GO="$(GO)" GOFMT="$(GOFMT)"

check: render-check ## Run scaffold self-checks against the local framework checkout.

check-local: ## Run scaffold checks against the local framework checkout.
	@$(MAKE) --no-print-directory check FRAMEWORK_REPLACE="$(abspath ..)" GO="$(GO)" GOFMT="$(GOFMT)"

check-pinned: ## Run scaffold checks against template/go.mod's pinned framework dependency.
	@$(MAKE) --no-print-directory check FRAMEWORK_REPLACE= GO="$(GO)" GOFMT="$(GOFMT)"

template-mod-tidy: ## Run go mod tidy for the scaffold template module.
	cd template && $(GO) mod tidy

template-deps-update: ## Update scaffold template dependencies and tidy module files.
	cd template && $(GO) get -u ./...
	cd template && $(GO) mod tidy

new-exporter: require-project-name require-target-dir ## Render a new exporter. Set PROJECT_NAME and optionally TARGET_DIR/GO_MODULE/PROJECT_DESC/FEATURE_NAME/FEATURE_NAMESPACE/METRIC_NAMESPACE/DEFAULT_PORT/DOCKER_SMOKE_*.
	@scripts/render.sh \
		--project-name "$(PROJECT_NAME)" \
		$(if $(GO_MODULE),--module "$(GO_MODULE)",) \
		$(if $(PROJECT_DESC),--description "$(PROJECT_DESC)",) \
		$(if $(FEATURE_NAME),--feature-name "$(FEATURE_NAME)",) \
		$(if $(FEATURE_NAMESPACE),--feature-namespace "$(FEATURE_NAMESPACE)",) \
		$(if $(METRIC_NAMESPACE),--namespace "$(METRIC_NAMESPACE)",) \
		$(if $(DEFAULT_PORT),--port "$(DEFAULT_PORT)",) \
		$(if $(FEATURE_CONFIG_FILE),--feature-config-file "$(FEATURE_CONFIG_FILE)",) \
		$(DOCKER_SMOKE_METRIC_ARG) \
		$(DOCKER_SMOKE_RUN_OPTIONS_ARG) \
		$(DOCKER_SMOKE_EXPORTER_ARGS_ARG) \
		$(DOCKER_SMOKE_EXTRA_METRICS_ARG) \
		--target-dir "$(TARGET_DIR)"

drift-check: require-target-dir ## Check scaffold-managed files. Set TARGET_DIR, optionally FILE, SKIP_FILE, ALL_FILES=1, and SYMBOL_DIFF=1.
	@scripts/scaffold-drift.sh \
		--target-dir "$(TARGET_DIR)" \
		$(if $(PROJECT_NAME),--project-name "$(PROJECT_NAME)",) \
		$(if $(GO_MODULE),--module "$(GO_MODULE)",) \
		$(if $(PROJECT_DESC),--description "$(PROJECT_DESC)",) \
		$(if $(FEATURE_NAME),--feature-name "$(FEATURE_NAME)",) \
		$(if $(FEATURE_NAMESPACE),--feature-namespace "$(FEATURE_NAMESPACE)",) \
		$(if $(METRIC_NAMESPACE),--namespace "$(METRIC_NAMESPACE)",) \
		$(if $(DEFAULT_PORT),--port "$(DEFAULT_PORT)",) \
		$(if $(FEATURE_CONFIG_FILE),--feature-config-file "$(FEATURE_CONFIG_FILE)",) \
		$(DOCKER_SMOKE_METRIC_ARG) \
		$(DOCKER_SMOKE_RUN_OPTIONS_ARG) \
		$(DOCKER_SMOKE_EXPORTER_ARGS_ARG) \
		$(DOCKER_SMOKE_EXTRA_METRICS_ARG) \
		$(DRIFT_SYMBOL_DIFF) \
		$(DRIFT_ALL_FILES) \
		$(DRIFT_SKIP_FILE_ARGS) \
		$(DRIFT_FILE_ARGS)

drift-check-all: require-target-dir ## Check every rendered scaffold file in an exporter. Set TARGET_DIR; use SYMBOL_DIFF=1 for Go symbol diffs.
	@$(MAKE) --no-print-directory drift-check TARGET_DIR="$(TARGET_DIR)" ALL_FILES=1 SYMBOL_DIFF="$(SYMBOL_DIFF)" GO="$(GO)" GOFMT="$(GOFMT)"

drift-sync: require-target-dir ## Sync scaffold-managed files into an existing exporter. Set TARGET_DIR; use SKIP_FILE and ALLOW_DIRTY=1 as needed.
	@scripts/scaffold-drift.sh \
		--target-dir "$(TARGET_DIR)" \
		--sync \
		$(DRIFT_ALLOW_DIRTY) \
		$(if $(PROJECT_NAME),--project-name "$(PROJECT_NAME)",) \
		$(if $(GO_MODULE),--module "$(GO_MODULE)",) \
		$(if $(PROJECT_DESC),--description "$(PROJECT_DESC)",) \
		$(if $(FEATURE_NAME),--feature-name "$(FEATURE_NAME)",) \
		$(if $(FEATURE_NAMESPACE),--feature-namespace "$(FEATURE_NAMESPACE)",) \
		$(if $(METRIC_NAMESPACE),--namespace "$(METRIC_NAMESPACE)",) \
		$(if $(DEFAULT_PORT),--port "$(DEFAULT_PORT)",) \
		$(if $(FEATURE_CONFIG_FILE),--feature-config-file "$(FEATURE_CONFIG_FILE)",) \
		$(DOCKER_SMOKE_METRIC_ARG) \
		$(DOCKER_SMOKE_RUN_OPTIONS_ARG) \
		$(DOCKER_SMOKE_EXPORTER_ARGS_ARG) \
		$(DOCKER_SMOKE_EXTRA_METRICS_ARG) \
		$(DRIFT_SKIP_FILE_ARGS) \
		$(DRIFT_FILE_ARGS)

drift-list-files: ## List the default scaffold-managed files used by drift-check/drift-sync.
	@scripts/scaffold-drift.sh --list-files

clean: ## Remove local rendered exporters and temporary artifacts.
	@rm -rf rendered tmp
	@$(MAKE) -C template clean

require-project-name:
	@test -n "$(PROJECT_NAME)" || { echo "PROJECT_NAME is required" >&2; exit 2; }

require-target-dir:
	@test -n "$(TARGET_DIR)" || { echo "TARGET_DIR is required" >&2; exit 2; }
