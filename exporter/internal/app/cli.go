package app

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/common/promslog"
	promflag "github.com/prometheus/common/promslog/flag"
	"github.com/prometheus/common/version"
	webflag "github.com/prometheus/exporter-toolkit/web/kingpinflag"
)

type cliConfig struct {
	options       Options
	promslogCfg   *promslog.Config
	runtimeConfig []any
}

func Main(cfg Config) {
	if err := RunCLI(cfg, os.Args[1:]); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func MainFromProject(features ...Feature) {
	cfg := ConfigFromProject(features...)
	cfg.Name = ExecutableName(os.Args, cfg.Name)
	Main(cfg)
}

// MainForProject runs a concrete exporter with explicit project metadata.
func MainForProject(projectName, description string, features ...Feature) {
	cfg := ConfigForProject(projectName, features...)
	cfg.Name = ExecutableName(os.Args, cfg.Name)
	cfg.Description = description
	Main(cfg)
}

func ExecutableName(args []string, fallback string) string {
	if len(args) == 0 {
		return fallback
	}

	name := filepath.Base(args[0])

	if name == "." || strings.TrimSpace(name) == "" {
		return fallback
	}

	return name
}

func RunCLIFromProject(args []string, features ...Feature) error {
	return RunCLI(ConfigFromProject(features...), args)
}

func RunCLI(cfg Config, args []string) error {
	cfg = cfg.normalized()
	HydrateVersionMetadata()

	parsed, err := parseCLIConfig(cfg, args)
	if err != nil {
		return err
	}

	logger := promslog.New(parsed.promslogCfg)
	logStartup(logger, cfg, parsed.runtimeConfig)

	return Run(parsed.options, logger)
}

func parseCLIConfig(cfg Config, args []string) (cliConfig, error) {
	cfg = cfg.normalized()
	app := kingpin.New(cfg.Name, cfg.Description)
	promslogCfg := &promslog.Config{}
	promflag.AddFlags(app, promslogCfg)

	toolkitFlags := webflag.AddFlags(app, cfg.DefaultListenAddress)
	metricsPath := app.Flag(
		"web.telemetry-path", "Path under which to expose metrics",
	).Default(cfg.DefaultMetricsPath).String()
	enablePprof := app.Flag(
		"web.enable-pprof", "Expose pprof endpoints and links on the landing page",
	).Default("false").Bool()

	for _, feature := range cfg.Features {
		if feature != nil {
			feature.RegisterFlags(app)
		}
	}

	app.Version(version.Print(cfg.Namespace))
	app.HelpFlag.Short('h')
	if _, err := app.Parse(args); err != nil {
		return cliConfig{}, err
	}
	if err := validateMetricsPath(*metricsPath); err != nil {
		return cliConfig{}, fmt.Errorf("invalid --web.telemetry-path %q: %w", *metricsPath, err)
	}

	opts := Options{
		Name:         cfg.Name,
		Namespace:    cfg.Namespace,
		Description:  cfg.Description,
		MetricsPath:  *metricsPath,
		ToolkitFlags: toolkitFlags,
		EnablePprof:  *enablePprof,
		Features:     cfg.Features,
	}

	return cliConfig{
		options:       opts,
		promslogCfg:   promslogCfg,
		runtimeConfig: runtimeConfigForOptions(opts),
	}, nil
}

func runtimeConfigForOptions(opts Options) []any {
	runtimeConfig := []any{
		"metrics_path", opts.MetricsPath,
		"pprof_enabled", opts.EnablePprof,
	}
	for _, feature := range opts.Features {
		reporter, ok := feature.(RuntimeConfigReporter)
		if !ok {
			continue
		}
		runtimeConfig = append(runtimeConfig, reporter.RuntimeConfig()...)
	}
	return runtimeConfig
}

func logStartup(logger *slog.Logger, cfg Config, runtimeConfig []any) {
	logger.Info("Starting "+cfg.Name, "version", version.Info())
	logger.Info("Build context", "build_context", version.BuildContext())
	logger.Info("Runtime config", runtimeConfig...)
}
