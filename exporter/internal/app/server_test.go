package app

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/exporter-toolkit/web"
	featurepkg "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/internal/feature"
)

func TestOptionsNormalizedSetsDefaults(t *testing.T) {
	t.Parallel()

	opts := Options{}.normalized()

	if opts.Name != defaultExporterName {
		t.Fatalf("Name = %q, want %q", opts.Name, defaultExporterName)
	}
	if opts.Namespace != defaultExporterName {
		t.Fatalf("Namespace = %q, want %q", opts.Namespace, defaultExporterName)
	}
	if opts.Description != defaultDescription {
		t.Fatalf("Description = %q, want %q", opts.Description, defaultDescription)
	}
	if opts.MetricsPath != defaultTelemetryPath {
		t.Fatalf("MetricsPath = %q, want %q", opts.MetricsPath, defaultTelemetryPath)
	}
}

func TestOptionsNormalizedUsesNameAsNamespace(t *testing.T) {
	t.Parallel()

	opts := Options{Name: "custom_exporter"}.normalized()
	if opts.Namespace != "custom_exporter" {
		t.Fatalf("Namespace = %q, want %q", opts.Namespace, "custom_exporter")
	}
}

func TestNewServerUsesProvidedRegistry(t *testing.T) {
	t.Parallel()

	registry := prometheus.NewRegistry()
	if err := featurepkg.RegisterCollectors(registry, newConstCollector("server_example_metric", "Server example metric", 5)); err != nil {
		t.Fatalf("RegisterCollectors() error = %v", err)
	}

	srv := NewServer(Options{
		Name:        "server_exporter",
		Description: "Server exporter",
		MetricsPath: "/custom-metrics",
	}, registry)

	req := httptest.NewRequest(http.MethodGet, "/custom-metrics", nil)
	rec := httptest.NewRecorder()
	srv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /custom-metrics status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "server_example_metric 5") {
		t.Fatalf("GET /custom-metrics body missing custom metric: %s", rec.Body.String())
	}
}

func TestNewServerCheckedRejectsInvalidMetricsPath(t *testing.T) {
	t.Parallel()

	srv, err := NewServerChecked(Options{MetricsPath: "metrics"}, prometheus.NewRegistry())
	if err == nil {
		t.Fatal("NewServerChecked() error = nil, want invalid metrics path error")
	}
	if srv != nil {
		t.Fatal("NewServerChecked() server != nil, want nil")
	}
	if !strings.Contains(err.Error(), `invalid metrics path "metrics"`) {
		t.Fatalf("NewServerChecked() error = %q, want metrics path context", err.Error())
	}
}

func TestRunInvokesListenAndServeWithConfiguredHandler(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "run-context")
	toolkitFlags := &web.FlagConfig{}
	feature := featurepkg.CollectorFeature{
		Name: "run",
		CollectorsFunc: func(ctx FeatureContext) ([]prometheus.Collector, error) {
			if ctx.ExporterName != "run_exporter" {
				t.Fatalf("FeatureContext.ExporterName = %q, want %q", ctx.ExporterName, "run_exporter")
			}
			if ctx.Namespace != "run_exporter" {
				t.Fatalf("FeatureContext.Namespace = %q, want %q", ctx.Namespace, "run_exporter")
			}
			if ctx.Logger == nil {
				t.Fatal("FeatureContext.Logger = nil, want logger")
			}
			if ctx.Context.Value(contextKey{}) != "run-context" {
				t.Fatal("FeatureContext.Context missing run context value")
			}
			return []prometheus.Collector{
				newConstCollector("run_feature_value", "Run feature value", 13),
			}, nil
		},
	}

	called := false
	stubListenAndServe(t, func(srv *http.Server, flags *web.FlagConfig, logger *slog.Logger) error {
		called = true
		if flags != toolkitFlags {
			t.Fatalf("ToolkitFlags = %p, want %p", flags, toolkitFlags)
		}
		if logger == nil {
			t.Fatal("logger = nil, want logger")
		}

		req := httptest.NewRequest(http.MethodGet, "/custom-metrics", nil)
		rec := httptest.NewRecorder()
		srv.Handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("GET /custom-metrics status = %d, want %d", rec.Code, http.StatusOK)
		}
		if !strings.Contains(rec.Body.String(), "run_feature_value 13") {
			t.Fatalf("GET /custom-metrics body missing feature metric: %s", rec.Body.String())
		}
		return nil
	})

	err := RunContext(ctx, Options{
		Name:         "run_exporter",
		Namespace:    "run_exporter",
		Description:  "Run exporter",
		MetricsPath:  "/custom-metrics",
		ToolkitFlags: toolkitFlags,
		EnablePprof:  true,
		Features:     []Feature{feature},
	}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if !called {
		t.Fatal("listenAndServe was not called")
	}
}

func TestRunWrapsRegistryErrorsBeforeServing(t *testing.T) {
	wantErr := errors.New("feature failed")
	feature := featurepkg.CollectorFeature{
		Name: "broken",
		RegisterCollectorsFunc: func(ctx FeatureContext, registry *prometheus.Registry) error {
			return wantErr
		},
	}

	stubListenAndServe(t, func(srv *http.Server, flags *web.FlagConfig, logger *slog.Logger) error {
		t.Fatal("listenAndServe was called despite registry error")
		return nil
	})

	err := Run(Options{Namespace: "broken_exporter", Features: []Feature{feature}}, nil)
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want wrapped %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), "create registry: register feature") {
		t.Fatalf("Run() error = %q, want registry context", err.Error())
	}
}

func TestRunContextShutsDownServerWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})

	stubListenAndServe(t, func(srv *http.Server, flags *web.FlagConfig, logger *slog.Logger) error {
		shutdown := make(chan struct{})
		srv.RegisterOnShutdown(func() {
			close(shutdown)
		})
		close(started)
		<-shutdown
		return http.ErrServerClosed
	})

	errCh := make(chan error, 1)
	go func() {
		errCh <- RunContext(ctx, Options{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	}()

	<-started
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("RunContext() error = %v, want nil after context cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("RunContext() did not return after context cancellation")
	}
}

func TestRunRejectsInvalidMetricsPathBeforeServing(t *testing.T) {
	stubListenAndServe(t, func(srv *http.Server, flags *web.FlagConfig, logger *slog.Logger) error {
		t.Fatal("listenAndServe was called despite invalid metrics path")
		return nil
	})

	err := Run(Options{MetricsPath: "metrics"}, nil)
	if err == nil {
		t.Fatal("Run() error = nil, want invalid metrics path error")
	}
	if !strings.Contains(err.Error(), `invalid metrics path "metrics"`) {
		t.Fatalf("Run() error = %q, want invalid metrics path context", err.Error())
	}
}

func TestMustRunPanicsWhenRunFails(t *testing.T) {
	wantErr := errors.New("listen failed")
	stubListenAndServe(t, func(srv *http.Server, flags *web.FlagConfig, logger *slog.Logger) error {
		return wantErr
	})

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("MustRun() did not panic")
		}
		gotErr, ok := recovered.(error)
		if !ok {
			t.Fatalf("MustRun() panic = %T(%v), want error", recovered, recovered)
		}
		if !errors.Is(gotErr, wantErr) {
			t.Fatalf("MustRun() panic = %v, want %v", gotErr, wantErr)
		}
	}()

	MustRun(Options{}, nil)
}

func stubListenAndServe(t *testing.T, fn func(*http.Server, *web.FlagConfig, *slog.Logger) error) {
	t.Helper()

	original := listenAndServe
	listenAndServe = fn
	t.Cleanup(func() {
		listenAndServe = original
	})
}
