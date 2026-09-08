# Rendered exporter contract. Keep this file scaffold-owned.
override SCAFFOLD_RENDERED := false
override GO_MODULE := __GO_MODULE__
override FRAMEWORK_MODULE := github.com/zxzharmlesszxz/prometheus-exporter-framework
override COVERAGE_PROFILE := coverage.out
override COVERAGE_REPORT := coverage.txt
override PROJECT_NAME := __PROJECT_NAME__
override PROJECT_DESC := __PROJECT_DESC__
override FEATURE_NAME := __FEATURE_NAME__
override FEATURE_NAMESPACE := __FEATURE_NAMESPACE__
override METRIC_NAMESPACE := __METRIC_NAMESPACE__
override DEFAULT_PORT := :__DEFAULT_PORT__
override FEATURE_CONFIG_FILE := __FEATURE_CONFIG_FILE__
override FEATURE_CONFIG_PATH := examples/$(FEATURE_CONFIG_FILE)
override FEATURE_CONFIG_CONTAINER_PATH := /etc/prometheus/$(FEATURE_CONFIG_FILE)
override MAIN_PACKAGE := ./cmd
override DIST_DIR := dist
define exporter_ldflags
-s -w \
 -X 'github.com/prometheus/common/version.Version=$(1)' \
 -X 'github.com/prometheus/common/version.Branch=$(2)' \
 -X 'github.com/prometheus/common/version.Revision=$(3)' \
 -X 'github.com/prometheus/common/version.BuildUser=$(4)' \
 -X 'github.com/prometheus/common/version.BuildDate=$(5)' \
 -X '$(FRAMEWORK_MODULE)/exporter.injectedExporterName=$(PROJECT_NAME)' \
 -X '$(FRAMEWORK_MODULE)/exporter.injectedExporterDescription=$(PROJECT_DESC)' \
 -X '$(FRAMEWORK_MODULE)/exporter.injectedFeatureName=$(FEATURE_NAME)' \
 -X '$(FRAMEWORK_MODULE)/exporter.injectedMetricNamespace=$(METRIC_NAMESPACE)' \
 -X '$(FRAMEWORK_MODULE)/exporter.injectedListenAddress=$(DEFAULT_PORT)' \
 -X '$(GO_MODULE)/internal/$(FEATURE_NAME).DefaultFeatureConfigFileName=$(FEATURE_CONFIG_FILE)'
endef
override LDFLAGS = $(call exporter_ldflags,$(VERSION),$(BRANCH),$(REVISION),$(BUILD_USER),$(BUILD_DATE))
override DOCKER_PROJECT_NAME := $(PROJECT_NAME)
override DOCKER_ENTRYPOINT_NAME := $(PROJECT_NAME)
override DOCKER_SMOKE_METRIC := __DOCKER_SMOKE_METRIC__
override DOCKER_SMOKE_RUN_OPTIONS := __DOCKER_SMOKE_RUN_OPTIONS__
override DOCKER_SMOKE_EXPORTER_ARGS := __DOCKER_SMOKE_EXPORTER_ARGS__
override DOCKER_SMOKE_EXTRA_METRICS :=__DOCKER_SMOKE_EXTRA_METRICS__
override DOCKER_SMOKE_PORT := 9900
override SMOKE_LDFLAGS = $(call exporter_ldflags,$(SMOKE_VERSION),$(SMOKE_BRANCH),$(SMOKE_REVISION),$(SMOKE_BUILD_USER),$(SMOKE_BUILD_DATE))
