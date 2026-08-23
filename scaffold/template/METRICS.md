# Metrics

## Example Metric

`__FEATURE_NAMESPACE___example_value`

Example business metric emitted by the generated skeleton.
Replace this metric with domain-specific metrics.

Domain metrics use the feature namespace. It defaults to `__FEATURE_NAMESPACE__`
in the generated project and is intentionally separate from the exporter
framework namespace below.

## Source Health Pattern

File-backed exporters should expose the same source-health metric shape for
each domain input source. For a source named `<source>`, use:

`__FEATURE_NAMESPACE___<source>_up`

Whether the source was readable during the last collection.

`__FEATURE_NAMESPACE___<source>_valid`

Whether the source produced valid domain data during the last collection.
Concrete exporters define the domain-specific validity rule.
Parse or domain-validity failures should leave
`__FEATURE_NAMESPACE___<source>_up` at `1` when the source was readable and
report invalid data through `__FEATURE_NAMESPACE___<source>_valid = 0`.

`__FEATURE_NAMESPACE___<source>_mtime_seconds`

Unix timestamp, including fractional seconds, of the source file modification
time.

`__FEATURE_NAMESPACE___<source>_scrape_duration_seconds`

Duration in seconds of the last source scrape.

`__FEATURE_NAMESPACE___<source>_read_errors_total`

Counter with the total number of source read errors. This is cumulative, not
the number of read errors in the latest scrape.

`__FEATURE_NAMESPACE___<source>_parse_errors_total`

Counter with the total number of source parse or validity errors. This is
cumulative, not the number of parse/validity errors in the latest scrape.

Use framework helpers to avoid hand-copying descriptor names:

```go
var sourceMetricIDs = featurekit.FileScrapeMetricIDsFor("<source>")

var featureMetricSpecs = append(
	[]featurekit.FeatureMetricSpec{
		// domain metrics
	},
	featurekit.FileScrapeMetricSpecs("<source>", []string{"source"})...,
)
```

In `CollectFeatureMetrics`, bind those descriptors to
`exporter.FileScrapeMetrics`, call `CollectResult`, then call `CollectValid`
with the exporter-defined validity boolean.
Read/parse error metrics are counters. Use a separate counter pair for each
logical source label when the exporter reports per-source totals; reusing one
counter pair intentionally reports aggregate totals.

## Exporter Collection Health

`__METRIC_NAMESPACE___collection_duration_seconds`

Histogram of framework collection refresh duration in seconds.
This measures the exporter background refresh loop, not Prometheus scrape time.
Use Prometheus-side `scrape_*` series for target scrape health and scrape
timing instead of duplicating those metrics in the exporter.

`__METRIC_NAMESPACE___last_collection_success`

Whether the last refresh succeeded.
The generated skeleton skips domain metrics when the latest snapshot failed.
Concrete exporters may choose different stale-data semantics in
`CollectFeatureMetrics`, but this framework metric always reports latest
refresh status.

`__METRIC_NAMESPACE___last_collection_timestamp_seconds`

Unix timestamp, including fractional seconds, of the last refresh attempt.
The value is `0` before the first collection attempt.

`__METRIC_NAMESPACE___last_successful_collection_timestamp_seconds`

Unix timestamp, including fractional seconds, of the last successful refresh.
The value is `0` until the first successful refresh.
