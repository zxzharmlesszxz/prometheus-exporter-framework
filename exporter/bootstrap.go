package exporter

import "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/internal/app"

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
	app.Main(cfg)
}

func MainFromProject(features ...Feature) {
	app.MainFromProject(features...)
}

func MainForProject(projectName, description string, features ...Feature) {
	app.MainForProject(projectName, description, features...)
}

func RunCLIFromProject(args []string, features ...Feature) error {
	return app.RunCLIFromProject(args, features...)
}

func RunCLI(cfg Config, args []string) error {
	return app.RunCLI(cfg, args)
}
