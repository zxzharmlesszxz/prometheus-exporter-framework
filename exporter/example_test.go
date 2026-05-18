package exporter_test

import (
	"fmt"
	"io"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	"github.com/zxzharmlesszxz/prometheus-template-exporter/exporter"
)

func ExampleConfigForProject() {
	cfg := exporter.ConfigForProject("git.example.net/platform/prometheus-demo-exporter")

	fmt.Println(cfg.Name)
	fmt.Println(cfg.Namespace)
	fmt.Println(cfg.Description)

	// Output:
	// demo_exporter
	// demo_exporter
	// Prometheus Demo Exporter
}

func ExampleCollectorFeature() {
	feature := exporter.CollectorFeature{
		Name: "demo",
		CollectorsFunc: func(ctx exporter.FeatureContext) ([]prometheus.Collector, error) {
			return []prometheus.Collector{
				prometheus.NewGaugeFunc(
					prometheus.GaugeOpts{
						Name: ctx.Namespace + "_demo_value",
						Help: "Demo value.",
					},
					func() float64 { return 1 },
				),
			}, nil
		},
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	registry, err := exporter.NewRegistry("demo_exporter", logger, feature)
	if err != nil {
		fmt.Println(err)
		return
	}

	families, err := registry.Gather()
	fmt.Println(err == nil)
	fmt.Println(hasMetricFamily(families, "demo_exporter_demo_value"))

	// Output:
	// true
	// true
}

func hasMetricFamily(families []*dto.MetricFamily, name string) bool {
	for _, family := range families {
		if family.GetName() == name {
			return true
		}
	}
	return false
}
