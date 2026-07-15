package main

import (
	"fmt"
	"os"

	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter"
)

func main() {
	if err := exporter.MainFromProjectErr(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
