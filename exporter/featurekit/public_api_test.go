package featurekit_test

import (
	"bytes"
	"flag"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var updatePublicAPI = flag.Bool("update-public-api", false, "update exporter/featurekit public API golden file")

func TestPublicAPISurface(t *testing.T) {
	const goldenPath = "testdata/public_api.txt"

	got := collectPublicAPI(t, ".")
	gotBytes := []byte(strings.Join(got, "\n") + "\n")

	if *updatePublicAPI {
		if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
			t.Fatalf("create testdata dir: %v", err)
		}
		if err := os.WriteFile(goldenPath, gotBytes, 0o644); err != nil {
			t.Fatalf("update golden file: %v", err)
		}
	}

	wantBytes, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden file: %v", err)
	}

	if !bytes.Equal(gotBytes, wantBytes) {
		t.Fatalf("public API surface changed; run `go test ./exporter/featurekit -update-public-api` if this change is intentional\n\nwant:\n%s\n\ngot:\n%s", wantBytes, gotBytes)
	}
}

func collectPublicAPI(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read exporter/featurekit dir: %v", err)
	}

	fset := token.NewFileSet()
	var symbols []string

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		path := filepath.Join(dir, name)
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}

		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				if decl.Recv == nil && decl.Name.IsExported() {
					symbols = append(symbols, "func "+decl.Name.Name)
				}

			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					switch spec := spec.(type) {
					case *ast.TypeSpec:
						if spec.Name.IsExported() {
							symbols = append(symbols, "type "+spec.Name.Name)
						}

					case *ast.ValueSpec:
						for _, ident := range spec.Names {
							if ident.IsExported() {
								symbols = append(symbols, decl.Tok.String()+" "+ident.Name)
							}
						}
					}
				}
			}
		}
	}

	sort.Strings(symbols)

	return symbols
}
