package app

import (
	"errors"
	"path"
	"runtime/debug"
	"strings"
	"unicode"
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

func ConfigFromProject(features ...Feature) Config {
	return ConfigForProject(moduleProjectName(), features...)
}

func ConfigForProject(projectName string, features ...Feature) Config {
	name := ExporterNameFromProject(projectName)
	return Config{
		Name:                 name,
		Namespace:            name,
		Description:          DescriptionFromProject(projectName),
		DefaultListenAddress: defaultListenAddressFromFeatures(features),
		Features:             features,
	}
}

func ExporterNameFromProject(projectName string) string {
	base := projectDomainName(projectName)
	name := sanitizeMetricNamespace(base)
	if name == "" {
		return defaultExporterName
	}
	if name == "exporter_framework" {
		return name
	}
	if !strings.HasSuffix(name, "_exporter") {
		name += "_exporter"
	}
	return name
}

func DescriptionFromProject(projectName string) string {
	base := projectBase(projectName)
	if base == "" {
		return defaultDescription
	}

	parts := strings.FieldsFunc(base, func(r rune) bool {
		return r == '-' || r == '_' || r == '.'
	})
	if len(parts) == 0 {
		return defaultDescription
	}

	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = titleProjectPart(part)
	}
	return strings.Join(parts, " ")
}

func titleProjectPart(part string) string {
	if acronym, ok := projectAcronyms[strings.ToLower(part)]; ok {
		return acronym
	}

	runes := []rune(part)
	if len(runes) == 0 {
		return ""
	}
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

var projectAcronyms = map[string]string{
	"api":   "API",
	"ca":    "CA",
	"cli":   "CLI",
	"cpu":   "CPU",
	"dns":   "DNS",
	"http":  "HTTP",
	"https": "HTTPS",
	"id":    "ID",
	"ip":    "IP",
	"json":  "JSON",
	"pkg":   "Package",
	"sql":   "SQL",
	"ssh":   "SSH",
	"ssl":   "SSL",
	"tcp":   "TCP",
	"tls":   "TLS",
	"udp":   "UDP",
	"uri":   "URI",
	"url":   "URL",
	"xml":   "XML",
	"yaml":  "YAML",
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

func moduleProjectName() string {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	return buildInfo.Main.Path
}

func projectDomainName(projectName string) string {
	base := projectBase(projectName)
	base = strings.TrimPrefix(base, "prometheus-")
	base = strings.TrimSuffix(base, "-exporter")
	base = strings.TrimSuffix(base, "_exporter")
	return base
}

func projectBase(projectName string) string {
	base := strings.TrimSpace(projectName)
	if base == "" {
		return ""
	}
	return path.Base(base)
}

func sanitizeMetricNamespace(value string) string {
	var builder strings.Builder
	lastUnderscore := false

	for _, r := range strings.ToLower(value) {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastUnderscore = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				builder.WriteByte('_')
				lastUnderscore = true
			}
		}
	}

	result := strings.Trim(builder.String(), "_")
	if result == "" {
		return ""
	}
	if result[0] >= '0' && result[0] <= '9' {
		return "_" + result
	}
	return result
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

func defaultListenAddressFromFeatures(features []Feature) string {
	for _, feature := range features {
		provider, ok := feature.(DefaultListenAddressProvider)
		if !ok {
			continue
		}
		listenAddress := strings.TrimSpace(provider.DefaultListenAddress())
		if listenAddress != "" && ValidateListenAddress(listenAddress) == nil {
			return listenAddress
		}
	}
	return defaultListenAddress
}
