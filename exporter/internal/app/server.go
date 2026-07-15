package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/exporter-toolkit/web"
)

const defaultShutdownTimeout = 5 * time.Second

type Options struct {
	Name         string
	Namespace    string
	Description  string
	MetricsPath  string
	ToolkitFlags *web.FlagConfig
	EnablePprof  bool
	Features     []Feature
}

var listenAndServe = web.ListenAndServe

func (o Options) normalized() Options {
	if o.Name == "" {
		o.Name = defaultExporterName
	}
	if o.Namespace == "" {
		o.Namespace = o.Name
	}
	if o.Description == "" {
		o.Description = defaultDescription
	}
	if o.MetricsPath == "" {
		o.MetricsPath = defaultTelemetryPath
	}
	return o
}

func Run(opts Options, logger *slog.Logger) error {
	return RunContext(context.Background(), opts, logger)
}

func RunContext(ctx context.Context, opts Options, logger *slog.Logger) error {
	if ctx == nil {
		ctx = context.Background()
	}
	opts = opts.normalized()
	if err := validateMetricsPath(opts.MetricsPath); err != nil {
		return fmt.Errorf("invalid metrics path %q: %w", opts.MetricsPath, err)
	}
	if logger == nil {
		logger = slog.Default()
	}

	registry, err := NewRegistryContext(ctx, opts.Namespace, logger.With("component", "collector"), opts.Features...)
	if err != nil {
		return fmt.Errorf("create registry: %w", err)
	}

	logger = logger.With("component", "server")

	handler := NewHandler(HandlerOptions{
		Name:        opts.Name,
		Description: opts.Description,
		MetricsPath: opts.MetricsPath,
		Registry:    registry,
		EnablePprof: opts.EnablePprof,
	})

	srv := &http.Server{Handler: handler}
	return listenAndServeContext(ctx, srv, opts.ToolkitFlags, logger)
}

func listenAndServeContext(ctx context.Context, srv *http.Server, flags *web.FlagConfig, logger *slog.Logger) error {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultShutdownTimeout)
			defer cancel()
			if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
				logger.Warn("shutting down server", "err", err)
			}
		case <-done:
		}
	}()

	err := listenAndServe(srv, flags, logger)
	close(done)
	if errors.Is(err, http.ErrServerClosed) && ctx.Err() != nil {
		return nil
	}
	return err
}

func MustRun(opts Options, logger *slog.Logger) {
	if err := Run(opts, logger); err != nil {
		panic(err)
	}
}

func NewServer(opts Options, registry *prometheus.Registry) *http.Server {
	opts = opts.normalized()
	return &http.Server{
		Handler: NewHandler(HandlerOptions{
			Name:        opts.Name,
			Description: opts.Description,
			MetricsPath: opts.MetricsPath,
			Registry:    registry,
			EnablePprof: opts.EnablePprof,
		}),
	}
}

func NewServerChecked(opts Options, registry *prometheus.Registry) (*http.Server, error) {
	opts = opts.normalized()
	handler, err := NewHandlerChecked(HandlerOptions{
		Name:        opts.Name,
		Description: opts.Description,
		MetricsPath: opts.MetricsPath,
		Registry:    registry,
		EnablePprof: opts.EnablePprof,
	})
	if err != nil {
		return nil, err
	}
	return &http.Server{Handler: handler}, nil
}
