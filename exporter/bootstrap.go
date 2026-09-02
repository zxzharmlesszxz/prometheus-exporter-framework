package exporter

import (
	"context"
	"fmt"
	"os"

	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/internal/app"
)

type Config = app.Config

func Main(cfg Config) {
	if err := app.Main(cfg); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// MainErr runs the exporter and returns startup/runtime errors instead of
// printing them and exiting the process.
func MainErr(cfg Config) error { return app.Main(cfg) }

func RunCLI(cfg Config, args []string) error {
	return app.RunCLI(cfg, args)
}

func RunCLIContext(ctx context.Context, cfg Config, args []string) error {
	return app.RunCLIContext(ctx, cfg, args)
}
