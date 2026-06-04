package exporter

import (
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/internal/app"
)

func NewRegistry(namespace string, logger *slog.Logger, features ...Feature) (*prometheus.Registry, error) {
	return app.NewRegistry(namespace, logger, features...)
}

type Options = app.Options

func Run(opts Options, logger *slog.Logger) error {
	return app.Run(opts, logger)
}

func MustRun(opts Options, logger *slog.Logger) {
	app.MustRun(opts, logger)
}

func NewServer(opts Options, registry *prometheus.Registry) *http.Server {
	return app.NewServer(opts, registry)
}

func NewServerChecked(opts Options, registry *prometheus.Registry) (*http.Server, error) {
	return app.NewServerChecked(opts, registry)
}
