package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

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

func Main(cfg Config) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := RunCLIContext(ctx, cfg, os.Args[1:]); err != nil {
		return err
	}
	return nil
}

func MainFromProject(features ...Feature) error {
	cfg := ConfigFromProject(features...)
	cfg.Name = ExecutableName(os.Args, cfg.Name)
	if err := Main(cfg); err != nil {
		return err
	}
	return nil
}

// MainForProject runs a concrete exporter with explicit project metadata.
func MainForProject(projectName, description string, features ...Feature) error {
	cfg := ConfigForProject(projectName, features...)
	cfg.Name = ExecutableName(os.Args, cfg.Name)
	cfg.Description = description
	if err := Main(cfg); err != nil {
		return err
	}
	return nil
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
	return RunCLIContext(context.Background(), cfg, args)
}

func RunCLIContext(ctx context.Context, cfg Config, args []string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	HydrateVersionMetadata()

	parsed, err := parseCLIConfig(cfg, args)
	if err != nil {
		return err
	}

	logger := promslog.New(parsed.promslogCfg)
	logStartup(logger, parsed.options.Name, parsed.runtimeConfig)

	return RunContext(ctx, parsed.options, logger)
}

func parseCLIConfig(cfg Config, args []string) (cliConfig, error) {
	cfg = cfg.normalized()
	if err := ValidateListenAddress(cfg.DefaultListenAddress); err != nil {
		return cliConfig{}, fmt.Errorf("invalid default listen address %q: %w", cfg.DefaultListenAddress, err)
	}
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

	app.Version(version.Print(cfg.Name))
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

func logStartup(logger *slog.Logger, exporterName string, runtimeConfig []any) {
	logger.Info("Starting "+exporterName, "version", version.Info())
	logger.Info("Build context", "build_context", version.BuildContext())
	logger.Info("Runtime config", runtimeConfig...)
}
