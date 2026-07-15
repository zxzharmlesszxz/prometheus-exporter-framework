package exporter

import (
	"context"
	"fmt"
	"os"

	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/internal/app"
)

type Config = app.Config

func ConfigFromProject(features ...Feature) Config {
	return app.ConfigFromProject(features...)
}

func ConfigForProject(projectName string, features ...Feature) Config {
	return app.ConfigForProject(projectName, features...)
}

func ExporterNameFromProject(projectName string) string {
	return app.ExporterNameFromProject(projectName)
}

func DescriptionFromProject(projectName string) string {
	return app.DescriptionFromProject(projectName)
}

func Main(cfg Config) {
	if err := app.Main(cfg); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func MainFromProject(features ...Feature) {
	if err := app.MainFromProject(features...); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func MainForProject(projectName, description string, features ...Feature) {
	if err := app.MainForProject(projectName, description, features...); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// MainErr runs the exporter and returns startup/runtime errors instead of
// printing them and exiting the process.
func MainErr(cfg Config) error { return app.Main(cfg) }

// MainFromProjectErr runs the exporter with project-derived metadata and
// returns startup/runtime errors instead of exiting the process.
func MainFromProjectErr(features ...Feature) error { return app.MainFromProject(features...) }

// MainForProjectErr runs the exporter with explicit project metadata and
// returns startup/runtime errors instead of exiting the process.
func MainForProjectErr(projectName, description string, features ...Feature) error {
	return app.MainForProject(projectName, description, features...)
}

func RunCLIFromProject(args []string, features ...Feature) error {
	return app.RunCLIFromProject(args, features...)
}

func RunCLI(cfg Config, args []string) error {
	return app.RunCLI(cfg, args)
}

func RunCLIContext(ctx context.Context, cfg Config, args []string) error {
	return app.RunCLIContext(ctx, cfg, args)
}
