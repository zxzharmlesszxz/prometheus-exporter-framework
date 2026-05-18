package exporter

import "testing"

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

	feature := CollectorFeature{Name: "demo", DefaultListenAddressValue: ":9999"}
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

	blank := CollectorFeature{Name: "blank", DefaultListenAddressValue: "  "}
	nonBlank := CollectorFeature{Name: "non_blank", DefaultListenAddressValue: ":9777"}
	cfg := ConfigForProject("prometheus-non_blank-exporter", blank, nonBlank)

	if cfg.DefaultListenAddress != ":9777" {
		t.Fatalf("DefaultListenAddress = %q, want %q", cfg.DefaultListenAddress, ":9777")
	}
}

func TestExporterNameFromProject(t *testing.T) {
	t.Parallel()

	tests := []struct {
		project string
		want    string
	}{
		{project: "prometheus-template-exporter", want: "template_exporter"},
		{project: "example.com/team/prometheus-puppetfile-exporter", want: "puppetfile_exporter"},
		{project: "custom-exporter", want: "custom_exporter"},
		{project: "123-custom", want: "_123_custom_exporter"},
		{project: "", want: "template_exporter"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.project, func(t *testing.T) {
			t.Parallel()

			if got := ExporterNameFromProject(tc.project); got != tc.want {
				t.Fatalf("ExporterNameFromProject(%q) = %q, want %q", tc.project, got, tc.want)
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
		{project: "multi_part.name", want: "Multi Part Name"},
		{project: "example.com/team/прометей-exporter", want: "Прометей Exporter"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.project, func(t *testing.T) {
			t.Parallel()

			if got := DescriptionFromProject(tc.project); got != tc.want {
				t.Fatalf("DescriptionFromProject(%q) = %q, want %q", tc.project, got, tc.want)
			}
		})
	}
}
