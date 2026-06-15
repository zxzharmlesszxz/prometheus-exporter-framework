package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunSymbolDiff(t *testing.T) {
	t.Parallel()

	left := writeGoFile(t, `package demo

const shared = "same"
const leftOnly = "left"

func Changed() string {
	return "left"
}
`)
	right := writeGoFile(t, `package demo

const shared = "same"
const rightOnly = "right"

func Changed() string {
	return "right"
}
`)

	var stdout, stderr bytes.Buffer
	code := runSymbolDiff([]string{"--left-label", "rendered", "--right-label", "target", left, right}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("runSymbolDiff() code = %d, stderr = %q", code, stderr.String())
	}
	output := stdout.String()
	wantFragments := []string{
		"SYMBOL DIFF func Changed\n",
		"--- rendered func Changed\n",
		"+++ target func Changed\n",
		"SYMBOL EXTRA target const leftOnly\n",
		"SYMBOL MISSING target const rightOnly\n",
		"SYMBOL OK const shared\n",
	}
	for _, fragment := range wantFragments {
		if !strings.Contains(output, fragment) {
			t.Fatalf("runSymbolDiff() output missing %q in:\n%s", fragment, output)
		}
	}
}

func TestRunSymbolDiffErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		wantStderr string
	}{
		{
			name:       "bad args",
			args:       nil,
			wantStderr: "usage: go-symbol-diff",
		},
		{
			name:       "bad flag",
			args:       []string{"--unknown"},
			wantStderr: "flag provided but not defined",
		},
		{
			name:       "bad left file",
			args:       []string{filepath.Join(t.TempDir(), "missing.go"), writeGoFile(t, "package demo\n")},
			wantStderr: "parse ",
		},
		{
			name:       "bad right file",
			args:       []string{writeGoFile(t, "package demo\n"), filepath.Join(t.TempDir(), "missing.go")},
			wantStderr: "parse ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer
			code := runSymbolDiff(tt.args, &stdout, &stderr)
			if code != 2 {
				t.Fatalf("runSymbolDiff() code = %d, want 2", code)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Fatalf("runSymbolDiff() stderr = %q, want fragment %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestSymbolsExtractsTopLevelDeclarations(t *testing.T) {
	t.Parallel()

	path := writeGoFile(t, `package demo

const answer = 42

var (
	first = "one"
	second = "two"
)

type box[T any] struct {
	value T
}

func NewBox[T any](value T) box[T] {
	return box[T]{value: value}
}

func (b box[T]) Value() T {
	return b.value
}

func (b *box[T]) Set(value T) {
	b.value = value
}
`)

	got, err := symbols(path)
	if err != nil {
		t.Fatalf("symbols() error = %v", err)
	}

	wantKeys := []string{
		"const answer",
		"func NewBox",
		"method *box.Set",
		"method box.Value",
		"type box",
		"var first",
		"var second",
	}
	for _, key := range wantKeys {
		if _, ok := got[key]; !ok {
			t.Fatalf("symbols() missing %q; got keys %v", key, keys(got))
		}
	}
	if got["type box"].order >= got["func NewBox"].order {
		t.Fatalf("type box order = %d, func NewBox order = %d; want type first", got["type box"].order, got["func NewBox"].order)
	}
	if !strings.Contains(got["method *box.Set"].text, "func (b *box[T]) Set(value T)") {
		t.Fatalf("method text = %q, want formatted receiver", got["method *box.Set"].text)
	}
}

func TestSymbolsReturnsParseError(t *testing.T) {
	t.Parallel()

	_, err := symbols(writeGoFile(t, "package demo\nfunc broken("))
	if err == nil {
		t.Fatal("symbols() error = nil, want parse error")
	}
}

func TestReceiverName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		expr string
		want string
	}{
		{name: "ident", expr: "box", want: "box"},
		{name: "pointer", expr: "*box", want: "*box"},
		{name: "index", expr: "box[T]", want: "box"},
		{name: "index list", expr: "box[K, V]", want: "box"},
		{name: "selector fallback", expr: "pkg.Box", want: "pkg.Box"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			expr, err := parser.ParseExpr(tt.expr)
			if err != nil {
				t.Fatalf("ParseExpr(%q) error = %v", tt.expr, err)
			}
			if got := receiverName(expr); got != tt.want {
				t.Fatalf("receiverName(%q) = %q, want %q", tt.expr, got, tt.want)
			}
		})
	}
}

func TestFormattingHelpers(t *testing.T) {
	t.Parallel()

	fileSet := token.NewFileSet()
	expr, err := parser.ParseExpr("map[string]int{\"a\": 1}")
	if err != nil {
		t.Fatalf("ParseExpr() error = %v", err)
	}
	if got := exprText(expr); !strings.Contains(got, `"a": 1`) {
		t.Fatalf("exprText() = %q, want formatted expression", got)
	}
	if got := nodeText(fileSet, expr); !strings.Contains(got, `"a": 1`) {
		t.Fatalf("nodeText() = %q, want formatted expression", got)
	}
	spec := &ast.TypeSpec{Name: ast.NewIdent("demo"), Type: ast.NewIdent("string")}
	if got := genSpecText(fileSet, "type", spec); got != "type demo string" {
		t.Fatalf("genSpecText() = %q, want type demo string", got)
	}
	if got := indent("one\ntwo\n"); got != "    one\n    two" {
		t.Fatalf("indent() = %q, want indented lines", got)
	}
}

func TestPrintLineDiff(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	printLineDiff(&output, "left", "same\nold\nleft-only", "right", "same\nnew")

	wantFragments := []string{
		"--- left\n",
		"+++ right\n",
		"  same\n",
		"- old\n",
		"+ new\n",
		"- left-only\n",
	}
	for _, fragment := range wantFragments {
		if !strings.Contains(output.String(), fragment) {
			t.Fatalf("printLineDiff() output missing %q in:\n%s", fragment, output.String())
		}
	}
}

func writeGoFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "input.go")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	return path
}

func keys(values map[string]symbol) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	return out
}
