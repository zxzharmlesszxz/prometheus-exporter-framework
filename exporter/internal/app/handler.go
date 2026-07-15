package app

import (
	"fmt"
	"net/http"
	"net/http/pprof"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
)

type HandlerOptions struct {
	Name        string
	Description string
	MetricsPath string
	Registry    *prometheus.Registry
	EnablePprof bool
}

func NewHandler(opts HandlerOptions) http.Handler {
	handler, err := NewHandlerChecked(opts)
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		})
	}
	return handler
}

func NewHandlerChecked(opts HandlerOptions) (http.Handler, error) {
	opts = opts.normalized()
	if err := validateMetricsPath(opts.MetricsPath); err != nil {
		return nil, fmt.Errorf("invalid metrics path %q: %w", opts.MetricsPath, err)
	}
	return newHandler(opts), nil
}

func (opts HandlerOptions) normalized() HandlerOptions {
	if opts.Name == "" {
		opts.Name = defaultLandingName
	}
	if opts.Description == "" {
		opts.Description = defaultDescription
	}
	if opts.MetricsPath == "" {
		opts.MetricsPath = defaultTelemetryPath
	}
	if opts.Registry == nil {
		opts.Registry = prometheus.NewRegistry()
	}
	return opts
}

func newHandler(opts HandlerOptions) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(opts.MetricsPath, promhttp.HandlerFor(opts.Registry, promhttp.HandlerOpts{}))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})

	registerPprofHandlers(mux, opts.EnablePprof)
	if opts.MetricsPath != "" && opts.MetricsPath != "/" {
		mux.Handle("/", landingPage(opts))
	}
	return mux
}

func registerPprofHandlers(mux *http.ServeMux, enabled bool) {
	if enabled {
		mux.HandleFunc("/debug/pprof/", pprof.Index)
		mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		mux.Handle("/debug/pprof/allocs", pprof.Handler("allocs"))
		mux.Handle("/debug/pprof/block", pprof.Handler("block"))
		mux.Handle("/debug/pprof/goroutine", pprof.Handler("goroutine"))
		mux.Handle("/debug/pprof/heap", pprof.Handler("heap"))
		mux.Handle("/debug/pprof/mutex", pprof.Handler("mutex"))
		mux.Handle("/debug/pprof/threadcreate", pprof.Handler("threadcreate"))
		return
	}

	mux.HandleFunc("/debug/pprof/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
}

func landingPage(opts HandlerOptions) http.Handler {
	profiling := defaultProfilingValue
	if opts.EnablePprof {
		profiling = "true"
	}

	landingPage, err := web.NewLandingPage(web.LandingConfig{
		Name:        opts.Name,
		Description: opts.Description,
		Version:     version.Info(),
		Profiling:   profiling,
		Links: []web.LandingLinks{
			{Address: opts.MetricsPath, Text: "Metrics"},
			{Address: "/healthz", Text: "Health"},
		},
	})
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		})
	}
	return landingPage
}
