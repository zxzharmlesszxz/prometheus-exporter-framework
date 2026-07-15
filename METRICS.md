# Metrics

The framework exports common Prometheus metrics and provides helpers for
generated exporters. Concrete exporters own their business metrics.

## Build Info

### `<metric_namespace>_build_info`

- Type: gauge
- Value: always `1`
- Labels: provided by `github.com/prometheus/common/version`
- Notes:
  - metric name is based on the exporter metric namespace
  - for the default framework binary the metric is
    `exporter_framework_build_info`

## Snapshot Collection Metrics

Exporters using the framework snapshot collector expose these metrics in the
exporter metric namespace.

### `<metric_namespace>_last_collection_success`

- Type: gauge
- Value:
  - `1` if the last feature data collection succeeded
  - `0` if the last feature data collection failed
- Labels: none
- Notes: reports framework refresh-loop status, not Prometheus scrape status

### `<metric_namespace>_last_collection_timestamp_seconds`

- Type: gauge
- Value: Unix timestamp, including fractional seconds, of the last feature data
  collection attempt
- Labels: none
- Notes: updates on both successful and failed collection attempts

### `<metric_namespace>_last_successful_collection_timestamp_seconds`

- Type: gauge
- Value: Unix timestamp, including fractional seconds, of the last successful
  feature data collection
- Labels: none
- Notes: remains unchanged while collection attempts fail

### `<metric_namespace>_collection_duration_seconds`

- Type: histogram
- Value: feature data collection duration in seconds
- Labels: none
- Notes:
  - measures exporter refresh-loop work
  - distinct from Prometheus `scrape_duration_seconds`
  - emitted for both background refreshes and scrape-triggered refreshes
  - Prometheus scrape health and scrape timing should come from Prometheus-side
    `scrape_*` series for the exporter target, not from duplicated
    exporter-owned metrics

## Source Health Pattern

The framework does not know domain sources, but it provides
`featurekit.FileScrapeMetricSpecs` and `exporter.FileScrapeMetrics` so
file-backed exporters can expose a consistent source-health shape.

For a feature namespace `<feature_namespace>` and source name `<source>`, use:

### `<feature_namespace>_<source>_up`

- Type: gauge
- Value:
  - `1` if the source was readable during the last collection
  - `0` if the source could not be read
- Labels: exporter-defined, commonly `source` or `path`

### `<feature_namespace>_<source>_valid`

- Type: gauge
- Value:
  - `1` if the source produced valid domain data during the last collection
  - `0` if the source data was invalid or incomplete
- Labels: exporter-defined, commonly `source` or `path`
- Notes:
  - the concrete exporter defines validity semantics
  - parse or domain-validity failures should leave `*_up = 1` when the source
    was readable and report invalid data through `*_valid = 0`

### `<feature_namespace>_<source>_mtime_seconds`

- Type: gauge
- Value: Unix timestamp, including fractional seconds, of the source file
  modification time, or `0` when unavailable
- Labels: exporter-defined, commonly `source` or `path`

### `<feature_namespace>_<source>_scrape_duration_seconds`

- Type: gauge
- Value: duration in seconds of the last source scrape/read/parse operation
- Labels: exporter-defined, commonly `source` or `path`

### `<feature_namespace>_<source>_read_errors_total`

- Type: counter
- Value: cumulative total number of source read errors
- Labels: exporter-defined, commonly `source` or `path`

### `<feature_namespace>_<source>_parse_errors_total`

- Type: counter
- Value: cumulative total number of source parse or validity errors
- Labels: exporter-defined, commonly `source` or `path`

## Go Runtime Metrics

The framework registers the standard Prometheus Go collector.
Metric names include:

- `go_gc_duration_seconds`
- `go_goroutines`
- `go_memstats_*`

## Process Metrics

The framework registers the standard Prometheus process collector.
Metric names include:

- `process_cpu_seconds_total`
- `process_open_fds`
- `process_resident_memory_bytes`

## Prometheus Scrape Metrics

Prometheus adds scrape-side metrics for each target. They are not emitted by the
exporter process itself.

Common examples:

- `up`
- `scrape_duration_seconds`
- `scrape_samples_scraped`
- `scrape_samples_post_metric_relabeling`
- `scrape_series_added`

## Health Endpoint

`/healthz` is not a Prometheus metric.
It returns `200 OK` while the process is serving requests.

## Business Metrics

This framework does not define business metrics.
Business metric contracts must be documented by each concrete exporter.
