package main

import (
	"fmt"
	"os"

	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
)

func main() {
	if err := exporter.MainErr(exporter.Config{
		Name:        "exporter_framework",
		Namespace:   "exporter_framework",
		Description: "Prometheus exporter framework",
	}); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
