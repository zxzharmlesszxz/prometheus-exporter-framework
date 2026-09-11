package grafana_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

type prometheusRuleFile struct {
	Groups []struct {
		Rules []struct {
			Alert       string            `yaml:"alert"`
			Expr        string            `yaml:"expr"`
			For         string            `yaml:"for"`
			Labels      map[string]string `yaml:"labels"`
			Annotations map[string]string `yaml:"annotations"`
		} `yaml:"rules"`
	} `yaml:"groups"`
}

type grafanaRuleFile struct {
	Groups []struct {
		Rules []struct {
			Title       string            `yaml:"title"`
			Condition   string            `yaml:"condition"`
			For         string            `yaml:"for"`
			Labels      map[string]string `yaml:"labels"`
			Annotations map[string]string `yaml:"annotations"`
			Data        []struct {
				RefID         string `yaml:"refId"`
				DatasourceUID string `yaml:"datasourceUid"`
				Model         struct {
					Expr       string `yaml:"expr"`
					Expression string `yaml:"expression"`
				} `yaml:"model"`
			} `yaml:"data"`
		} `yaml:"rules"`
	} `yaml:"groups"`
}

func TestGrafanaAlertsMirrorPrometheusMetadata(t *testing.T) {
	const prometheusRulesPath = "../prometheus/__PROJECT_NAME__.yml"
	const grafanaRulesPath = "alerting/__PROJECT_NAME__.yml"

	if _, err := os.Stat(grafanaRulesPath); errors.Is(err, os.ErrNotExist) {
		t.Skipf("no Grafana alerting rules found at examples/grafana/%s", grafanaRulesPath)
	} else if err != nil {
		t.Fatal(err)
	}

	var prometheus prometheusRuleFile
	readYAML(t, prometheusRulesPath, &prometheus)
	var grafana grafanaRuleFile
	readYAML(t, grafanaRulesPath, &grafana)

	want := make(map[string]struct {
		Expr        string
		For         string
		Severity    string
		Summary     string
		Description string
	})
	for _, group := range prometheus.Groups {
		for _, rule := range group.Rules {
			want[rule.Alert] = struct {
				Expr        string
				For         string
				Severity    string
				Summary     string
				Description string
			}{rule.Expr, rule.For, rule.Labels["severity"], rule.Annotations["summary"], rule.Annotations["description"]}
		}
	}

	seen := make(map[string]bool)
	for _, group := range grafana.Groups {
		for _, rule := range group.Rules {
			expected, ok := want[rule.Title]
			if !ok {
				t.Errorf("Grafana rule %q has no Prometheus counterpart", rule.Title)
				continue
			}
			seen[rule.Title] = true
			if rule.For != expected.For || rule.Labels["severity"] != expected.Severity || rule.Annotations["summary"] != expected.Summary {
				t.Errorf("Grafana rule %q metadata differs from Prometheus", rule.Title)
			}
			wantDescription := strings.Replace(expected.Description, "$value", "$values.A.Value", 1)
			if rule.Annotations["description"] != wantDescription {
				t.Errorf("Grafana rule %q description differs from Prometheus", rule.Title)
			}
			if rule.Labels["service"] != "__PROJECT_NAME__" || rule.Labels["rule_source"] != "grafana" {
				t.Errorf("Grafana rule %q lacks routing labels", rule.Title)
			}
			if rule.Condition != "B" || len(rule.Data) != 2 || rule.Data[0].RefID != "A" || rule.Data[0].DatasourceUID != "DS_PROMETHEUS" || rule.Data[0].Model.Expr == "" || rule.Data[1].RefID != "B" || rule.Data[1].DatasourceUID != "__expr__" || rule.Data[1].Model.Expression == "" {
				t.Errorf("Grafana rule %q has invalid query wiring", rule.Title)
			}
			if !equivalentExpression(expected.Expr, rule.Data[0].Model.Expr, rule.Data[1].Model.Expression) {
				t.Errorf("Grafana rule %q expression differs from Prometheus", rule.Title)
			}
		}
	}

	for name := range want {
		if !seen[name] {
			t.Errorf("Prometheus rule %q has no Grafana counterpart", name)
		}
	}
}

func equivalentExpression(prometheusExpr, query, condition string) bool {
	if normalizeExpression(prometheusExpr) == normalizeExpression(query) {
		return normalizeExpression(condition) == normalizeExpression("$A > 0")
	}
	expanded := strings.Replace(condition, "$A", query, 1)
	return normalizeExpression(prometheusExpr) == normalizeExpression(expanded)
}

func normalizeExpression(value string) string {
	return strings.Join(strings.Fields(value), " ")
}

func readYAML(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(data, target); err != nil {
		t.Fatal(err)
	}
}
