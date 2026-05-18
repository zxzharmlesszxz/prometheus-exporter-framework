# Metrics

The template exports common Prometheus metrics only.
Concrete features own their business metrics.

## Common Metrics

### `*_build_info`

- Type: gauge
- Value: always `1`
- Labels: provided by `github.com/prometheus/common/version`
- Notes:
  - metric name is based on `Config.Namespace`
  - for the default binary the metric is `template_exporter_build_info`

### Go Runtime Metrics

The template registers the standard Prometheus Go collector.
Metric names include:

- `go_gc_duration_seconds`
- `go_goroutines`
- `go_memstats_*`

### Process Metrics

The template registers the standard Prometheus process collector.
Metric names include:

- `process_cpu_seconds_total`
- `process_open_fds`
- `process_resident_memory_bytes`

## Health Endpoint

`/healthz` is not a Prometheus metric.
It returns `200 OK` while the process is serving requests.

Feature-specific source health should be exposed by the feature itself, for example:

- `exporter_up`

## Business Metrics

This template does not define business metrics.
Business metric contracts must be documented by the concrete exporter feature.
