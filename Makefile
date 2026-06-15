GO ?= go
GOFMT ?= gofmt
STATICCHECK_VERSION ?= v0.7.0
STATICCHECK ?= $(GO) run honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)
STATICCHECK_GOFLAGS ?= -buildvcs=false
COVERAGE_PROFILE ?= coverage.out
COVERAGE_REPORT ?= coverage.txt
COVERAGE_THRESHOLD ?= 90.0
SMOKE_VERSION ?= v9.8.7
SMOKE_BRANCH ?= smoke-branch
SMOKE_REVISION ?= abc123def
SMOKE_BUILD_USER ?= smoke-test
SMOKE_BUILD_DATE ?= 2026-05-17T00:00:00Z

.PHONY: help fmt fmt-check vet staticcheck test test-race coverage coverage-check smoke check clean
.PHONY: public-api-check public-api-update %-public-api-check %-public-api-update

help: ## Show available make targets.
	@printf "\033[33mUsage:\033[0m\n"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "};{printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

fmt: ## Format Go files.
	$(GOFMT) -w $$($(GO) list -f '{{range .GoFiles}}{{$$.Dir}}/{{.}} {{end}}{{range .TestGoFiles}}{{$$.Dir}}/{{.}} {{end}}' ./...)

fmt-check: ## Check Go formatting.
	@test -z "$$($(GOFMT) -l $$($(GO) list -f '{{range .GoFiles}}{{$$.Dir}}/{{.}} {{end}}{{range .TestGoFiles}}{{$$.Dir}}/{{.}} {{end}}' ./... | tr '\n' ' '))"

vet: ## Run go vet.
	$(GO) vet ./...

staticcheck: ## Run staticcheck.
	GOFLAGS="$(strip $(GOFLAGS) $(STATICCHECK_GOFLAGS))" $(STATICCHECK) ./...

test: ## Run Go tests.
	$(GO) test ./...

test-race: ## Run Go tests with the race detector.
	$(GO) test -race ./...

exporter-public-api-check: ## Check exporter public API surface.
	$(GO) test ./exporter

featurekit-public-api-check: ## Check featurekit public API surface.
	$(GO) test ./exporter/featurekit

exporter-public-api-update: ## Update exporter public API golden file.
	$(GO) test ./exporter -update-public-api

featurekit-public-api-update: ## Update featurekit public API golden file.
	$(GO) test ./exporter/featurekit -update-public-api

public-api-update: exporter-public-api-update featurekit-public-api-update ## Update public API golden files.

public-api-check: exporter-public-api-check featurekit-public-api-check ## Check public API golden files.

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

check: fmt-check vet staticcheck coverage-check smoke test-race public-api-check ## Run the standard maintenance check.

clean: ## Remove generated local artifacts.
	rm -f $(COVERAGE_PROFILE) $(COVERAGE_REPORT)

.PHONY: scaffold-compatibility
scaffold-compatibility: ## Render a demo exporter from local scaffold and run its Go-only checks.
	@set -e; \
	tmp="$$(mktemp -d)"; \
	trap 'rm -rf "$$tmp"' EXIT; \
	target="$$tmp/prometheus-demo-exporter"; \
	$(MAKE) -C scaffold new-exporter \
		PROJECT_NAME=prometheus-demo-exporter \
		GO_MODULE=github.com/example/prometheus-demo-exporter \
		PROJECT_DESC="Prometheus Demo Exporter" \
		FEATURE_NAME=demo \
		METRIC_NAMESPACE=demo_exporter \
		DEFAULT_PORT=9888 \
		TARGET_DIR="$$target"; \
	cd "$$target"; \
	$(GO) mod edit -replace github.com/zxzharmlesszxz/prometheus-exporter-framework="$(CURDIR)"; \
	$(GO) mod tidy; \
	$(MAKE) go-check GO="$(GO)" GOFMT="$(GOFMT)"
