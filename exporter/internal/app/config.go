package app

import (
	"errors"
	"strings"
)

const (
	defaultExporterName   = "exporter_framework"
	defaultDescription    = "Prometheus exporter framework"
	defaultListenAddress  = ":9900"
	defaultTelemetryPath  = "/metrics"
	defaultLandingName    = "exporter_framework"
	defaultProfilingValue = "false"
)

var (
	errMetricNamespaceEmpty      = errors.New("metric namespace is empty")
	errMetricNamespaceWhitespace = errors.New("metric namespace must not contain leading or trailing whitespace")
	errMetricNamespaceInvalid    = errors.New("metric namespace must contain only letters, digits, or underscores and must not start with a digit")
)

type Config struct {
	Name                 string
	Namespace            string
	Description          string
	DefaultListenAddress string
	DefaultMetricsPath   string
	Features             []Feature
}

func (c Config) normalized() Config {
	if c.Name == "" {
		c.Name = defaultExporterName
	}
	if c.Namespace == "" {
		c.Namespace = c.Name
	}
	if c.Description == "" {
		c.Description = defaultDescription
	}
	if c.DefaultListenAddress == "" {
		c.DefaultListenAddress = defaultListenAddress
	}
	if c.DefaultMetricsPath == "" {
		c.DefaultMetricsPath = defaultTelemetryPath
	}
	return c
}

func validateMetricNamespace(value string) error {
	if strings.TrimSpace(value) == "" {
		return errMetricNamespaceEmpty
	}
	if strings.TrimSpace(value) != value {
		return errMetricNamespaceWhitespace
	}
	for i, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r == '_':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return errMetricNamespaceInvalid
		}
	}
	return nil
}
