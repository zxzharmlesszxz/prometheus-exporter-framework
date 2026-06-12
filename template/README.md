# __PROJECT_NAME__

`__PROJECT_NAME__` exposes __FEATURE_NAME__ state as Prometheus metrics.

It is built as a thin exporter on top of `prometheus-exporter-framework`.

## Local Run

```bash
make build
./dist/__PROJECT_NAME__ \
  --web.listen-address=:__DEFAULT_PORT__ \
  --__FEATURE_NAME__.config-file=examples/__FEATURE_CONFIG_FILE__
```

Useful flags:

```bash
--__FEATURE_NAME__.config-file
--__FEATURE_NAME__.refresh-interval
--web.listen-address
--web.telemetry-path
--web.enable-pprof
--log.level
--log.format
```

By default, the exporter listens on `:__DEFAULT_PORT__` and refreshes data every `1m`.
If no `--__FEATURE_NAME__.config-file` value is provided, `/etc/prometheus/prometheus-__FEATURE_NAME__-exporter.yml` is loaded when it exists; if it is missing, defaults and flags are used.
The generated `examples/__FEATURE_CONFIG_FILE__` file is an empty but valid feature config for the skeleton exporter.
Make, Compose, and smoke defaults use `FEATURE_CONFIG_FILE`, which defaults to `__FEATURE_CONFIG_FILE__`, and pass that path explicitly with `--__FEATURE_NAME__.config-file=...`.
Runtime config can always be overridden with another `--__FEATURE_NAME__.config-file=...` value.
Data refresh runs through the framework snapshot collector in a background worker; scrapes return the last collected snapshot.

## Metrics

Example output:

```code
__FEATURE_NAME___example_value 1
__METRIC_NAMESPACE___last_collection_success 1
__METRIC_NAMESPACE___last_collection_timestamp_seconds 1742812800
__METRIC_NAMESPACE___last_successful_collection_timestamp_seconds 1742812800
```

The full metric contract lives in [`METRICS.md`](METRICS.md).

## Docker Compose

The repository includes [`docker-compose.yml`](docker-compose.yml) for local testing.
The Prometheus scrape config is embedded in Compose, while alerting rules live
under [`examples/prometheus`](examples/prometheus).
It starts:

- `exporter`
- `prometheus`
- `grafana`

```bash
make compose
```

Endpoints:

- `http://localhost:__DEFAULT_PORT__`
- `http://localhost:__DEFAULT_PORT__/metrics`
- `http://localhost:__DEFAULT_PORT__/healthz`
- `http://localhost:9090`
- `http://localhost:3000`

## Grafana

Docker Compose provisions Grafana with:

- Prometheus datasource `DS_PROMETHEUS`
- dashboards from [`examples/grafana`](examples/grafana)
- default login `admin` / `admin`

Open `http://localhost:3000` after `make compose`.

For a direct Docker build, run:

```bash
make docker-build
```

## Tests

```bash
make go-check
```

The repository includes the same maintenance target layout used by the concrete exporter repos:

```bash
make help
make go-check
make check
make docker-smoke
make full-check
```

`make go-check` runs Go-only checks. `make check` also validates the Prometheus and Docker Compose examples, so it requires Docker.

## Scaffold-Owned Go Files

Go files named `scaffold_*.go` are generated contract glue and should stay
identical to the scaffold output. Add exporter-specific behavior in adjacent
non-scaffold files such as `feature_config_ext.go`, `feature_metrics_ext.go`,
`feature_snapshotter_ext.go`, `feature_smoke_ext.go`,
`feature_test_suite_ext_test.go`, `metrics.go`, and the feature check package.
The generated starter `Snapshot` struct lives in `snapshot_types.go`, and the
starter snapshot structure lives in `internal/__FEATURE_NAME__check`; both are
feature-owned after generation so concrete exporters can replace the alias with
an aggregate snapshot and split domain checks into multiple packages.
Standard feature tests run through framework `exporter/exportertest/featuretest`
via `scaffold_feature_test_suite_test.go`; add feature-specific test cases by
registering them in `feature_test_suite_ext_test.go`.

Build local release artifacts:

```bash
make build VERSION=v0.1.0
make release VERSION=v0.1.0
make release-smoke VERSION=v0.1.0
```

Build and push a Docker image:

```bash
make docker-build VERSION=v0.1.0 DOCKER_IMAGE=__PROJECT_NAME__:v0.1.0
make docker-push DOCKER_IMAGE=__PROJECT_NAME__:v0.1.0
make docker-buildx-push VERSION=v0.1.0 DOCKER_IMAGE=registry.example.com/__PROJECT_NAME__:v0.1.0
```

## Architecture

The high-level design is documented in [`ARCHITECTURE.md`](ARCHITECTURE.md).

## License

This project is licensed under the MIT License. See [`LICENSE`](LICENSE).
