package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	if err := os.Chdir("../.."); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestGenerateIncludesReferenceSections(t *testing.T) {
	content, err := generate()
	if err != nil {
		t.Fatalf("generate() error = %v", err)
	}
	text := string(content)

	for _, want := range []string{
		"# Generated Reference",
		"## Root Make Targets",
		"- `docs-check`: Check generated reference documentation.",
		"## Scaffold Make Targets",
		"- `scaffold-drift-sync`: Sync scaffold-managed files into an existing exporter.",
		"## Scaffold Managed Files",
		"- `Dockerfile`",
		"## Demo Scaffold Metadata",
		"project-name: prometheus-demo-exporter",
		"## Public API Surfaces",
		"func NewTTLCache func[K comparable, V any](ttl time.Duration) *TTLCache[K, V]",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("generated docs do not contain %q", want)
		}
	}
}

func TestMakeTargetsAddsPrefixAndSorts(t *testing.T) {
	targets := makeTargets("scaffold/Makefile", "scaffold-")
	if len(targets) == 0 {
		t.Fatal("expected scaffold make targets")
	}
	for i, target := range targets {
		if !strings.HasPrefix(target.Name, "scaffold-") {
			t.Fatalf("target %q does not have scaffold prefix", target.Name)
		}
		if i > 0 && targets[i-1].Name > target.Name {
			t.Fatalf("targets are not sorted: %q before %q", targets[i-1].Name, target.Name)
		}
	}
}

func TestMakeVariableExpandsReferences(t *testing.T) {
	vars := map[string]string{
		"CHECK_PROJECT_NAME":        "prometheus-demo-exporter",
		"CHECK_FEATURE_CONFIG_FILE": "$(CHECK_PROJECT_NAME).yml",
	}

	got := makeVariable(vars, "CHECK_FEATURE_CONFIG_FILE")
	if got != "prometheus-demo-exporter.yml" {
		t.Fatalf("makeVariable() = %q, want prometheus-demo-exporter.yml", got)
	}
}

func TestDocsCheckComparison(t *testing.T) {
	content, err := generate()
	if err != nil {
		t.Fatalf("generate() error = %v", err)
	}

	path := filepath.Join(t.TempDir(), "reference.md")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatalf("write temp docs: %v", err)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read temp docs: %v", err)
	}
	if !bytes.Equal(current, content) {
		t.Fatal("written docs differ from generated docs")
	}
}

func TestRunGeneratesAndChecksDocs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "reference.md")

	if code := run([]string{"-output", path}); code != 0 {
		t.Fatalf("run generate exit code = %d, want 0", code)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("generated docs stat: %v", err)
	}
	if code := run([]string{"-output", path, "-check"}); code != 0 {
		t.Fatalf("run check exit code = %d, want 0", code)
	}
}

func TestRunCheckFailsForStaleDocs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reference.md")
	if err := os.WriteFile(path, []byte("stale\n"), 0o644); err != nil {
		t.Fatalf("write stale docs: %v", err)
	}

	if code := run([]string{"-output", path, "-check"}); code != 1 {
		t.Fatalf("run stale check exit code = %d, want 1", code)
	}
}

func TestRunCheckFailsForMissingDocs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.md")

	if code := run([]string{"-output", path, "-check"}); code != 1 {
		t.Fatalf("run missing check exit code = %d, want 1", code)
	}
}

func TestRunGenerateFailsWhenOutputIsDirectory(t *testing.T) {
	if code := run([]string{"-output", t.TempDir()}); code != 1 {
		t.Fatalf("run directory output exit code = %d, want 1", code)
	}
}

func TestRunGenerateFailsWhenOutputParentIsFile(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(parent, []byte("file\n"), 0o644); err != nil {
		t.Fatalf("write parent file: %v", err)
	}

	if code := run([]string{"-output", filepath.Join(parent, "reference.md")}); code != 1 {
		t.Fatalf("run file parent output exit code = %d, want 1", code)
	}
}

func TestRunFailsForBadFlags(t *testing.T) {
	if code := run([]string{"-bad-flag"}); code != 2 {
		t.Fatalf("run bad flag exit code = %d, want 2", code)
	}
}

func TestRunFailsWhenGenerationFails(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "Makefile"), "help: ## Show help.\n")
	writeTestFile(t, filepath.Join(dir, "scaffold", "Makefile"), strings.Join([]string{
		"new-exporter: ## Render exporter.",
		"CHECK_PROJECT_NAME ?= prometheus-demo-exporter",
		"CHECK_GO_MODULE ?= github.com/example/prometheus-demo-exporter",
		"CHECK_PROJECT_DESC ?= Prometheus Demo Exporter",
		"CHECK_FEATURE_NAME ?= demo",
		"CHECK_METRIC_NAMESPACE ?= demo_exporter",
		"CHECK_DEFAULT_PORT ?= 9888",
		"CHECK_FEATURE_CONFIG_FILE ?= $(CHECK_PROJECT_NAME).yml",
		"",
	}, "\n"))
	writeTestFile(t, filepath.Join(dir, "scaffold", "scripts", "scaffold-drift.sh"), strings.Join([]string{
		"default_files=(",
		`"Makefile"`,
		")",
		"obsolete_files=(",
		`"old.yml"`,
		")",
		"",
	}, "\n"))
	writeTestFile(t, filepath.Join(dir, "scaffold", "template", "go.mod"), "module github.com/example/prometheus-demo-exporter\n\nrequire (\n\tgithub.com/zxzharmlesszxz/prometheus-exporter-framework v0.0.0\n)\n")

	restore := chdir(t, dir)
	defer restore()

	if code := run([]string{"-output", filepath.Join(dir, "reference.md")}); code != 1 {
		t.Fatalf("run generation failure exit code = %d, want 1", code)
	}
}

func TestWritePublicAPIAddsMissingTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "exporter", "testdata", "public_api.txt"), "func Example")
	restore := chdir(t, dir)
	defer restore()

	var b strings.Builder
	if err := writePublicAPI(&b); err != nil {
		t.Fatalf("writePublicAPI() error = %v", err)
	}

	if !strings.Contains(b.String(), "func Example\n```") {
		t.Fatalf("public API block does not add trailing newline:\n%s", b.String())
	}
}

func TestExpandMakeVariablesLeavesUnknownReferences(t *testing.T) {
	got := expandMakeVariables("$(KNOWN)-$(UNKNOWN)", map[string]string{
		"KNOWN": "value",
	})
	if got != "value-$(UNKNOWN)" {
		t.Fatalf("expandMakeVariables() = %q, want value-$(UNKNOWN)", got)
	}
}

func chdir(t *testing.T, dir string) func() {
	t.Helper()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("get current dir: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	return func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatalf("restore dir %s: %v", previous, err)
		}
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
