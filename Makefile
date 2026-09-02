GO ?= go
GOFMT ?= gofmt
STATICCHECK_VERSION ?= v0.8.1
STATICCHECK ?= $(GO) run honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)
STATICCHECK_GOFLAGS ?= -buildvcs=false
GOVULNCHECK_VERSION ?= v1.7.0
GOVULNCHECK ?= $(GO) run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
GOLANGCI_LINT_VERSION ?= v2.13.1
GOLANGCI_LINT ?= $(GO) run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
COVERAGE_PROFILE ?= coverage.out
COVERAGE_REPORT ?= coverage.txt
COVERAGE_THRESHOLD ?= 90.0
SMOKE_VERSION ?= v9.8.7
SMOKE_BRANCH ?= smoke-branch
SMOKE_REVISION ?= abc123def
SMOKE_BUILD_USER ?= smoke-test
SMOKE_BUILD_DATE ?= 2026-05-17T00:00:00Z
SCAFFOLD_MAKEFILE ?= scaffold/Makefile

.PHONY: help fmt fmt-check vet staticcheck govulncheck golangci-lint test test-race coverage coverage-check smoke check clean
.PHONY: mod-tidy deps-update framework-mod-tidy framework-deps-update
.PHONY: public-api-check public-api-update %-public-api-check %-public-api-update
.PHONY: scaffold-help scaffold-scripts-check scaffold-tools-check scaffold-symbol-diff-check scaffold-render-check
.PHONY: scaffold-check scaffold-check-local scaffold-check-pinned scaffold-template-mod-tidy scaffold-template-deps-update
.PHONY: scaffold-new-exporter scaffold-drift-check scaffold-drift-check-all scaffold-drift-sync scaffold-drift-list-files scaffold-clean
.PHONY: FORCE

help: ## Show available make targets.
	@printf "\033[33mUsage:\033[0m\n"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "};{printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
	@if [ -f "$(SCAFFOLD_MAKEFILE)" ]; then \
		printf "\n\033[33mScaffold targets:\033[0m\n"; \
		awk 'BEGIN {FS = ":.*?## "}; /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-30s\033[0m %s\n", "scaffold-" $$1, $$2}' "$(SCAFFOLD_MAKEFILE)" | sort; \
	fi

fmt: ## Format Go files.
	$(GOFMT) -w $$($(GO) list -f '{{range .GoFiles}}{{$$.Dir}}/{{.}} {{end}}{{range .TestGoFiles}}{{$$.Dir}}/{{.}} {{end}}' ./...)

fmt-check: ## Check Go formatting.
	@test -z "$$($(GOFMT) -l $$($(GO) list -f '{{range .GoFiles}}{{$$.Dir}}/{{.}} {{end}}{{range .TestGoFiles}}{{$$.Dir}}/{{.}} {{end}}' ./... | tr '\n' ' '))"

vet: ## Run go vet.
	$(GO) vet ./...

staticcheck: ## Run staticcheck.
	GOFLAGS="$(strip $(GOFLAGS) $(STATICCHECK_GOFLAGS))" $(STATICCHECK) ./...

govulncheck: ## Run Go vulnerability analysis.
	$(GOVULNCHECK) ./...

golangci-lint: ## Run golangci-lint.
	$(GOLANGCI_LINT) run ./...

test: ## Run Go tests.
	$(GO) test ./...

test-race: ## Run Go tests with the race detector.
	@packages="$$($(GO) list -f '{{if .TestGoFiles}}{{.ImportPath}}{{end}}' ./...)"; \
	if [ -z "$$packages" ]; then \
		echo "no test packages"; \
		exit 0; \
	fi; \
	$(GO) test -race $$packages

framework-mod-tidy: ## Run go mod tidy for the framework module.
	$(GO) mod tidy

framework-deps-update: ## Update framework module dependencies and tidy module files.
	$(GO) get -u ./...
	$(GO) mod tidy

mod-tidy: framework-mod-tidy scaffold-template-mod-tidy ## Run go mod tidy for framework and scaffold template modules.

deps-update: framework-deps-update scaffold-template-deps-update ## Update dependencies for framework and scaffold template modules.

exporter-public-api-check: ## Check exporter public API surface.
	$(GO) test ./exporter

featurekit-public-api-check: ## Check featurekit public API surface.
	$(GO) test ./exporter/featurekit

exportertest-public-api-check: ## Check exportertest public API surface.
	$(GO) test ./exporter/exportertest/...

exporter-public-api-update: ## Update exporter public API golden file.
	$(GO) test ./exporter -update-public-api

featurekit-public-api-update: ## Update featurekit public API golden file.
	$(GO) test ./exporter/featurekit -update-public-api

exportertest-public-api-update: ## Update exportertest public API golden file.
	$(GO) test ./exporter/exportertest/... -update-public-api

public-api-update: exporter-public-api-update featurekit-public-api-update exportertest-public-api-update ## Update public API golden files.

public-api-check: exporter-public-api-check featurekit-public-api-check exportertest-public-api-check ## Check public API golden files.

coverage: ## Run tests with coverage and write coverage reports.
	$(GO) test ./... -covermode=atomic -coverprofile=$(COVERAGE_PROFILE)
	$(GO) tool cover -func=$(COVERAGE_PROFILE) | tee $(COVERAGE_REPORT)

coverage-check: coverage ## Enforce the coverage threshold.
	@coverage="$$(awk '/^total:/ {gsub(/%/, "", $$3); print $$3}' $(COVERAGE_REPORT))"; \
	awk -v coverage="$$coverage" -v threshold="$(COVERAGE_THRESHOLD)" 'BEGIN { \
		if (coverage + 0 < threshold + 0) { \
			printf "coverage %.1f%% is below %.1f%%\n", coverage, threshold; \
			exit 1; \
		} \
		printf "coverage %.1f%% meets threshold %.1f%%\n", coverage, threshold; \
	}'

smoke: ## Build and smoke-test the local binary.
	RUN_BINARY_SMOKE=1 GO="$(GO)" $(GO) test ./smoke -run TestBinarySmoke -count=1

check: fmt-check vet staticcheck golangci-lint govulncheck coverage-check smoke test-race ## Run the standard maintenance check.

clean: ## Remove generated local artifacts.
	rm -f $(COVERAGE_PROFILE) $(COVERAGE_REPORT)
	@$(MAKE) -C scaffold clean

scaffold-help: ## Show scaffold make targets.
	@$(MAKE) -C scaffold help

scaffold-scripts-check: ## Check scaffold shell scripts.
	@$(MAKE) -C scaffold scripts-check GO="$(GO)" GOFMT="$(GOFMT)"

scaffold-tools-check: ## Check scaffold local tools.
	@$(MAKE) -C scaffold tools-check GO="$(GO)" GOFMT="$(GOFMT)"

scaffold-symbol-diff-check: ## Check scaffold symbol diff helper.
	@$(MAKE) -C scaffold symbol-diff-check GO="$(GO)" GOFMT="$(GOFMT)"

scaffold-render-check: ## Render demo exporter and run scaffold checks.
	@$(MAKE) -C scaffold render-check GO="$(GO)" GOFMT="$(GOFMT)"

scaffold-check: ## Run scaffold self-checks.
	@$(MAKE) -C scaffold check GO="$(GO)" GOFMT="$(GOFMT)"

scaffold-check-local: ## Check rendered scaffold against local framework checkout.
	@$(MAKE) -C scaffold check-local GO="$(GO)" GOFMT="$(GOFMT)"

scaffold-check-pinned: ## Check rendered scaffold against pinned framework dependency.
	@$(MAKE) -C scaffold check-pinned GO="$(GO)" GOFMT="$(GOFMT)"

scaffold-template-mod-tidy: ## Run go mod tidy for the scaffold template module.
	@$(MAKE) -C scaffold template-mod-tidy GO="$(GO)" GOFMT="$(GOFMT)"

scaffold-template-deps-update: ## Update scaffold template dependencies.
	@$(MAKE) -C scaffold template-deps-update GO="$(GO)" GOFMT="$(GOFMT)"

scaffold-new-exporter: ## Render a new exporter through scaffold.
	@$(MAKE) -C scaffold new-exporter GO="$(GO)" GOFMT="$(GOFMT)"

scaffold-drift-check: ## Check scaffold-managed files in an exporter.
	@$(MAKE) -C scaffold drift-check GO="$(GO)" GOFMT="$(GOFMT)"

scaffold-drift-check-all: ## Check every rendered scaffold file in an exporter.
	@$(MAKE) -C scaffold drift-check-all GO="$(GO)" GOFMT="$(GOFMT)"

scaffold-drift-sync: ## Sync scaffold-managed files into an exporter.
	@$(MAKE) -C scaffold drift-sync GO="$(GO)" GOFMT="$(GOFMT)"

scaffold-drift-list-files: ## List scaffold-managed files.
	@$(MAKE) -C scaffold drift-list-files GO="$(GO)" GOFMT="$(GOFMT)"

scaffold-clean: ## Remove scaffold local generated artifacts.
	@$(MAKE) -C scaffold clean GO="$(GO)" GOFMT="$(GOFMT)"

scaffold-%: FORCE
	@$(MAKE) -C scaffold $* GO="$(GO)" GOFMT="$(GOFMT)"

FORCE:
