package app

import (
	"errors"
	"testing"
)

func TestConfigNormalizedSetsDefaults(t *testing.T) {
	t.Parallel()

	cfg := Config{}.normalized()

	if cfg.Name != defaultExporterName {
		t.Fatalf("Name = %q, want %q", cfg.Name, defaultExporterName)
	}
	if cfg.Namespace != defaultExporterName {
		t.Fatalf("Namespace = %q, want %q", cfg.Namespace, defaultExporterName)
	}
	if cfg.Description != defaultDescription {
		t.Fatalf("Description = %q, want %q", cfg.Description, defaultDescription)
	}
	if cfg.DefaultListenAddress != defaultListenAddress {
		t.Fatalf("DefaultListenAddress = %q, want %q", cfg.DefaultListenAddress, defaultListenAddress)
	}
	if cfg.DefaultMetricsPath != defaultTelemetryPath {
		t.Fatalf("DefaultMetricsPath = %q, want %q", cfg.DefaultMetricsPath, defaultTelemetryPath)
	}
}

func TestConfigNormalizedUsesNameAsNamespace(t *testing.T) {
	t.Parallel()

	cfg := Config{Name: "custom_exporter"}.normalized()
	if cfg.Namespace != "custom_exporter" {
		t.Fatalf("Namespace = %q, want %q", cfg.Namespace, "custom_exporter")
	}
}

func TestValidateMetricNamespace(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		namespace string
		wantErr   error
	}{
		{name: "simple", namespace: "demo_exporter"},
		{name: "leading underscore", namespace: "_123_demo_exporter"},
		{name: "mixed case", namespace: "Demo_Exporter"},
		{name: "empty", namespace: "", wantErr: errMetricNamespaceEmpty},
		{name: "blank", namespace: "  ", wantErr: errMetricNamespaceEmpty},
		{name: "trimmed", namespace: " demo_exporter", wantErr: errMetricNamespaceWhitespace},
		{name: "hyphen", namespace: "demo-exporter", wantErr: errMetricNamespaceInvalid},
		{name: "starts with digit", namespace: "123_demo_exporter", wantErr: errMetricNamespaceInvalid},
		{name: "colon", namespace: "demo:exporter", wantErr: errMetricNamespaceInvalid},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := validateMetricNamespace(tc.namespace)
			if tc.wantErr == nil && err != nil {
				t.Fatalf("validateMetricNamespace(%q) error = %v, want nil", tc.namespace, err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("validateMetricNamespace(%q) error = %v, want %v", tc.namespace, err, tc.wantErr)
			}
		})
	}
}
