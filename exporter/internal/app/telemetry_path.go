package app

import (
	"fmt"
	"net/url"
	"strings"
)

func validateMetricsPath(metricsPath string) error {
	if metricsPath == "" {
		return fmt.Errorf("must not be empty")
	}
	if strings.TrimSpace(metricsPath) != metricsPath {
		return fmt.Errorf("must not contain leading or trailing whitespace")
	}
	if !strings.HasPrefix(metricsPath, "/") {
		return fmt.Errorf("must start with /")
	}
	if strings.ContainsAny(metricsPath, " \t\r\n") {
		return fmt.Errorf("must not contain whitespace")
	}
	if strings.ContainsAny(metricsPath, "?#") {
		return fmt.Errorf("must not include query strings or fragments")
	}
	if strings.ContainsAny(metricsPath, "{}") {
		return fmt.Errorf("must be a literal URL path, not a ServeMux pattern")
	}
	if strings.Contains(metricsPath, "//") {
		return fmt.Errorf("must not contain double slashes")
	}

	parsed, err := url.ParseRequestURI(metricsPath)
	if err != nil {
		return fmt.Errorf("must be a valid URL path: %w", err)
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("must not include query strings or fragments")
	}

	if metricsPath == "/healthz" || strings.HasPrefix(metricsPath, "/healthz/") {
		return fmt.Errorf("conflicts with reserved health endpoint /healthz")
	}
	if metricsPath == "/debug/pprof" || strings.HasPrefix(metricsPath, "/debug/pprof/") {
		return fmt.Errorf("conflicts with reserved pprof endpoint /debug/pprof/")
	}
	return nil
}
