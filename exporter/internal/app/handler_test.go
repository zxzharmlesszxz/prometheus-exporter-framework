package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	featurepkg "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/internal/feature"
)

func TestHandlerServesMetrics(t *testing.T) {
	t.Parallel()

	registry := prometheus.NewRegistry()
	if err := featurepkg.RegisterCollectors(registry, newConstCollector("template_example_metric", "Example metric", 7)); err != nil {
		t.Fatalf("RegisterCollectors() error = %v", err)
	}

	handler := NewHandler(HandlerOptions{
		Name:        "exporter_framework",
		Description: "Exporter framework",
		MetricsPath: "/metrics",
		Registry:    registry,
		EnablePprof: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /metrics status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "template_example_metric 7") {
		t.Fatalf("GET /metrics body missing custom metric: %s", rec.Body.String())
	}
}

func TestHandlerServesMetricsAtRoot(t *testing.T) {
	t.Parallel()

	registry := prometheus.NewRegistry()
	if err := featurepkg.RegisterCollectors(registry, newConstCollector("root_example_metric", "Root example metric", 11)); err != nil {
		t.Fatalf("RegisterCollectors() error = %v", err)
	}

	handler := NewHandler(HandlerOptions{
		MetricsPath: "/",
		Registry:    registry,
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), "root_example_metric 11") {
		t.Fatalf("GET / body missing custom metric: %s", rec.Body.String())
	}
}

func TestNewHandlerCheckedRejectsInvalidMetricsPath(t *testing.T) {
	t.Parallel()

	handler, err := NewHandlerChecked(HandlerOptions{MetricsPath: "metrics"})
	if err == nil {
		t.Fatal("NewHandlerChecked() error = nil, want invalid metrics path error")
	}
	if handler != nil {
		t.Fatal("NewHandlerChecked() handler != nil, want nil")
	}
	if !strings.Contains(err.Error(), `invalid metrics path "metrics"`) {
		t.Fatalf("NewHandlerChecked() error = %q, want metrics path context", err.Error())
	}
}

func TestNewHandlerReturnsErrorHandlerForInvalidMetricsPath(t *testing.T) {
	t.Parallel()

	handler := NewHandler(HandlerOptions{MetricsPath: "/healthz"})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("GET / status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), `invalid metrics path "/healthz"`) {
		t.Fatalf("GET / body = %q, want invalid metrics path error", rec.Body.String())
	}
}

func TestHandlerServesHealthz(t *testing.T) {
	t.Parallel()

	handler := NewHandler(HandlerOptions{Registry: prometheus.NewRegistry()})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /healthz status = %d, want %d", rec.Code, http.StatusOK)
	}
	if rec.Body.String() != "ok\n" {
		t.Fatalf("GET /healthz body = %q, want %q", rec.Body.String(), "ok\n")
	}
	if got, want := rec.Header().Get("Content-Type"), "text/plain; charset=utf-8"; got != want {
		t.Fatalf("GET /healthz Content-Type = %q, want %q", got, want)
	}
}

func TestHandlerServesLandingPage(t *testing.T) {
	t.Parallel()

	handler := NewHandler(HandlerOptions{
		Name:        "exporter_framework",
		Description: "Reusable Prometheus exporter framework",
		MetricsPath: "/metrics",
		Registry:    prometheus.NewRegistry(),
		EnablePprof: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET / status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Reusable Prometheus exporter framework",
		"/metrics",
		"/healthz",
		"/debug/pprof/heap",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("GET / body missing %q: %s", want, body)
		}
	}
}

func TestHandlerDisablesPprofWhenConfigured(t *testing.T) {
	t.Parallel()

	handler := NewHandler(HandlerOptions{
		MetricsPath: "/metrics",
		Registry:    prometheus.NewRegistry(),
		EnablePprof: false,
	})

	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/heap?debug=1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /debug/pprof/heap status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestHandlerEnablesPprofWhenConfigured(t *testing.T) {
	t.Parallel()

	handler := NewHandler(HandlerOptions{
		MetricsPath: "/metrics",
		Registry:    prometheus.NewRegistry(),
		EnablePprof: true,
	})

	req := httptest.NewRequest(http.MethodGet, "/debug/pprof/heap?debug=1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /debug/pprof/heap status = %d, want %d", rec.Code, http.StatusOK)
	}
}
