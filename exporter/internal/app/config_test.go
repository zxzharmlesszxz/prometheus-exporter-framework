package app

import (
	"errors"
	"testing"

	featurepkg "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/internal/feature"
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

func TestConfigForProject(t *testing.T) {
	t.Parallel()

	feature := featurepkg.CollectorFeature{Name: "demo", DefaultListenAddressValue: ":9999"}
	cfg := ConfigForProject("git.example.net/platform/prometheus-demo-exporter", feature)

	if cfg.Name != "demo_exporter" {
		t.Fatalf("Name = %q, want %q", cfg.Name, "demo_exporter")
	}
	if cfg.Namespace != "demo_exporter" {
		t.Fatalf("Namespace = %q, want %q", cfg.Namespace, "demo_exporter")
	}
	if cfg.Description != "Prometheus Demo Exporter" {
		t.Fatalf("Description = %q, want %q", cfg.Description, "Prometheus Demo Exporter")
	}
	if cfg.DefaultListenAddress != ":9999" {
		t.Fatalf("DefaultListenAddress = %q, want %q", cfg.DefaultListenAddress, ":9999")
	}
	if len(cfg.Features) != 1 {
		t.Fatalf("Features len = %d, want 1", len(cfg.Features))
	}
}

func TestConfigForProjectFallsBackToDefaultListenAddress(t *testing.T) {
	t.Parallel()

	cfg := ConfigForProject("prometheus-demo-exporter")
	if cfg.DefaultListenAddress != defaultListenAddress {
		t.Fatalf("DefaultListenAddress = %q, want %q", cfg.DefaultListenAddress, defaultListenAddress)
	}
}

func TestConfigForProjectSkipsBlankFeatureListenAddress(t *testing.T) {
	t.Parallel()

	blank := featurepkg.CollectorFeature{Name: "blank", DefaultListenAddressValue: "  "}
	nonBlank := featurepkg.CollectorFeature{Name: "non_blank", DefaultListenAddressValue: ":9777"}
	cfg := ConfigForProject("prometheus-non_blank-exporter", blank, nonBlank)

	if cfg.DefaultListenAddress != ":9777" {
		t.Fatalf("DefaultListenAddress = %q, want %q", cfg.DefaultListenAddress, ":9777")
	}
}

func TestConfigForProjectSkipsInvalidFeatureListenAddress(t *testing.T) {
	t.Parallel()

	invalid := featurepkg.CollectorFeature{Name: "invalid", DefaultListenAddressValue: "9888"}
	valid := featurepkg.CollectorFeature{Name: "valid", DefaultListenAddressValue: "127.0.0.1:9777"}
	cfg := ConfigForProject("prometheus-valid-exporter", invalid, valid)

	if cfg.DefaultListenAddress != "127.0.0.1:9777" {
		t.Fatalf("DefaultListenAddress = %q, want 127.0.0.1:9777", cfg.DefaultListenAddress)
	}
}

func TestExporterNameFromProject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		project string
		want    string
	}{
		{project: "prometheus-exporter-framework", want: "exporter_framework"},
		{project: "example.com/team/prometheus-puppetfile-exporter", want: "puppetfile_exporter"},
		{project: "custom-exporter", want: "custom_exporter"},
		{project: "123-custom", want: "_123_custom_exporter"},
		{project: "", want: "exporter_framework"},
	}

	for _, tc := range tests {
		t.Run(tc.project, func(t *testing.T) {
			t.Parallel()

			if got := ExporterNameFromProject(tc.project); got != tc.want {
				t.Fatalf("ExporterNameFromProject(%q) = %q, want %q", tc.project, got, tc.want)
			}
		})
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

func TestDescriptionFromProject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		project string
		want    string
	}{
		{project: "", want: defaultDescription},
		{project: "  ", want: defaultDescription},
		{project: "example.com/team/custom-exporter", want: "Custom Exporter"},
		{project: "example.com/team/prometheus-pkg-exporter", want: "Prometheus Package Exporter"},
		{project: "example.com/team/prometheus-ssl-exporter", want: "Prometheus SSL Exporter"},
		{project: "example.com/team/prometheus-tls-api-exporter", want: "Prometheus TLS API Exporter"},
		{project: "multi_part.name", want: "Multi Part Name"},
		{project: "example.com/team/прометей-exporter", want: "Прометей Exporter"},
	}

	for _, tc := range tests {
		t.Run(tc.project, func(t *testing.T) {
			t.Parallel()

			if got := DescriptionFromProject(tc.project); got != tc.want {
				t.Fatalf("DescriptionFromProject(%q) = %q, want %q", tc.project, got, tc.want)
			}
		})
	}
}
