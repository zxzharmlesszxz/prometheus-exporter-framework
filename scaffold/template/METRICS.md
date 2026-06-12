# Metrics

## Example Metric

`__FEATURE_NAME___example_value`

Example business metric emitted by the generated skeleton.
Replace this metric with domain-specific metrics.

## Exporter Collection Health

`__METRIC_NAMESPACE___last_collection_success`

Whether the last refresh succeeded.
When cached data exists, the exporter can continue to expose the last successful business metrics while this metric is `0`.

`__METRIC_NAMESPACE___last_collection_timestamp_seconds`

Unix timestamp of the last refresh attempt.
The value is `0` before the first collection attempt.

`__METRIC_NAMESPACE___last_successful_collection_timestamp_seconds`

Unix timestamp of the last successful refresh.
The value is `0` until the first successful refresh.
