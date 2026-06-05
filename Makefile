GO ?= go
GOFMT ?= gofmt
DOCKER ?= docker
STATICCHECK_VERSION ?= v0.7.0
STATICCHECK ?= $(GO) run honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK_VERSION)
STATICCHECK_GOFLAGS ?= -buildvcs=false
COVERAGE_PROFILE ?= coverage.out
COVERAGE_REPORT ?= coverage.txt
COVERAGE_THRESHOLD ?= 90.0
DOCKER_IMAGE ?= prometheus-exporter-framework:smoke
DOCKER_HTTP_IMAGE ?= busybox:1.36
SMOKE_VERSION ?= v9.8.7
SMOKE_BRANCH ?= smoke-branch
SMOKE_REVISION ?= abc123def
SMOKE_BUILD_USER ?= smoke-test
SMOKE_BUILD_DATE ?= 2026-05-17T00:00:00Z

.PHONY: help fmt fmt-check vet staticcheck test test-race coverage coverage-check smoke docker-build docker-smoke-image docker-smoke check clean public-api-check public-api-update
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

public-api-check:
	$(GO) test ./exporter

public-api-update:
	$(GO) test ./exporter -update-public-api

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

docker-build: ## Build the Docker image used by docker-smoke.
	$(DOCKER) build \
		--build-arg VERSION=$(SMOKE_VERSION) \
		--build-arg BRANCH=$(SMOKE_BRANCH) \
		--build-arg REVISION=$(SMOKE_REVISION) \
		--build-arg BUILD_USER=$(SMOKE_BUILD_USER) \
		--build-arg BUILD_DATE=$(SMOKE_BUILD_DATE) \
		-t $(DOCKER_IMAGE) \
		.

docker-smoke-image: ## Smoke-test an already built Docker image.
	@version_output="$$( $(DOCKER) run --rm $(DOCKER_IMAGE) --version 2>&1 )"; \
	echo "$$version_output"; \
	echo "$$version_output" | grep -F "$(SMOKE_VERSION)" >/dev/null; \
	echo "$$version_output" | grep -F "$(SMOKE_BRANCH)" >/dev/null; \
	echo "$$version_output" | grep -F "$(SMOKE_REVISION)" >/dev/null
	@cid="$$( $(DOCKER) run -d --rm $(DOCKER_IMAGE) --log.level=error --web.listen-address=:9900 )"; \
	trap '$(DOCKER) rm -f '"'"'$$cid'"'"' >/dev/null 2>&1 || true' EXIT; \
	i=0; \
	while [ "$$i" -lt 60 ]; do \
		i=$$((i + 1)); \
		if $(DOCKER) run --rm --network container:$$cid $(DOCKER_HTTP_IMAGE) wget -qO- http://127.0.0.1:9900/healthz 2>/dev/null | grep -qx 'ok'; then \
			break; \
		fi; \
		if [ "$$i" -eq 60 ]; then \
			$(DOCKER) logs "$$cid"; \
			exit 1; \
		fi; \
		sleep 1; \
	done; \
	metrics="$$( $(DOCKER) run --rm --network container:$$cid $(DOCKER_HTTP_IMAGE) wget -qO- http://127.0.0.1:9900/metrics )"; \
	echo "$$metrics" | grep -F "exporter_framework_build_info" >/dev/null; \
	echo "$$metrics" | grep -F 'version="$(SMOKE_VERSION)"' >/dev/null; \
	echo "$$metrics" | grep -F 'branch="$(SMOKE_BRANCH)"' >/dev/null; \
	echo "$$metrics" | grep -F 'revision="$(SMOKE_REVISION)"' >/dev/null

docker-smoke: docker-build docker-smoke-image ## Build and smoke-test the Docker image.

check: fmt-check vet staticcheck coverage-check smoke test-race public-api-check ## Run the standard maintenance check.

clean: ## Remove generated local artifacts.
	rm -f $(COVERAGE_PROFILE) $(COVERAGE_REPORT)
