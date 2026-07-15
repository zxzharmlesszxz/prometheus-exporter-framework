package app

import "testing"

func TestValidateMetricsPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		metricsPath string
		wantErr     bool
	}{
		{name: "default path", metricsPath: "/metrics"},
		{name: "root path", metricsPath: "/"},
		{name: "nested path", metricsPath: "/prometheus/metrics"},
		{name: "trailing slash", metricsPath: "/metrics/"},
		{name: "escaped path", metricsPath: "/metrics/%7Bname%7D"},
		{name: "empty", metricsPath: "", wantErr: true},
		{name: "missing slash", metricsPath: "metrics", wantErr: true},
		{name: "leading whitespace", metricsPath: " /metrics", wantErr: true},
		{name: "embedded whitespace", metricsPath: "/met rics", wantErr: true},
		{name: "query string", metricsPath: "/metrics?format=openmetrics", wantErr: true},
		{name: "fragment", metricsPath: "/metrics#section", wantErr: true},
		{name: "double slash", metricsPath: "//metrics", wantErr: true},
		{name: "servemux wildcard", metricsPath: "/metrics/{name}", wantErr: true},
		{name: "bad escape", metricsPath: "/metrics/%zz", wantErr: true},
		{name: "health endpoint", metricsPath: "/healthz", wantErr: true},
		{name: "health subtree", metricsPath: "/healthz/metrics", wantErr: true},
		{name: "pprof endpoint", metricsPath: "/debug/pprof", wantErr: true},
		{name: "pprof subtree", metricsPath: "/debug/pprof/heap", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateMetricsPath(tc.metricsPath)
			if tc.wantErr && err == nil {
				t.Fatalf("validateMetricsPath(%q) error = nil, want error", tc.metricsPath)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("validateMetricsPath(%q) error = %v, want nil", tc.metricsPath, err)
			}
		})
	}
}
