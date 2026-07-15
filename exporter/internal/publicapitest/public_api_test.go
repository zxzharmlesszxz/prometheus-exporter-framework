package publicapitest

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCollectPublicAPI(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "api.go", `package demo

import "time"

const ExportedConst = "value"
var ExportedVar = 1

func ExportedFunc() {}
func unexportedFunc() {}

type ExportedStruct struct {
	Name string `+"`json:\"name\"`"+`
	count int
}

func (ExportedStruct) Method() {}
func (ExportedStruct) hidden() {}

type Box[T any] struct {
	Value T
}

func (*Box[T]) PointerMethod() {}

type ExportedInt int
type ExportedFuncType func(string) error

type EmbeddedStruct struct {
	*ExportedStruct
	time.Time
}

type ExportedInterface interface {
	Read() string
	hidden()
}

type EmbeddedInterface interface {
	time.Time
}

type hiddenStruct struct {
	Name string
}
`)

	got := Collect(t, Options{Dir: dir, ReadDirLabel: "demo"})
	want := []string{
		"const ExportedConst",
		"field Box.Value T",
		"field EmbeddedStruct.ExportedStruct *ExportedStruct",
		"field EmbeddedStruct.Time time.Time",
		"field ExportedStruct.Name string `json:\"name\"`",
		"func ExportedFunc func()",
		"interface EmbeddedInterface.Time time.Time",
		"interface ExportedInterface.Read func() string",
		"method (*Box[T]) PointerMethod func()",
		"method (ExportedStruct) Method func()",
		"type Box[T any]",
		"type EmbeddedInterface",
		"type EmbeddedStruct",
		"type ExportedFuncType func(string) error",
		"type ExportedInt int",
		"type ExportedInterface",
		"type ExportedStruct",
		"var ExportedVar",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Collect() mismatch\nwant:\n%s\n\ngot:\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
	}
}

func TestCollectAliasTargetMembers(t *testing.T) {
	dir := t.TempDir()
	exporterDir := filepath.Join(dir, "exporter")
	internalDir := filepath.Join(exporterDir, "internal", "app")
	writeFile(t, exporterDir, "api.go", `package exporter

import (
	app "example.test/framework/exporter/internal/app"
	other "example.test/other"
)

type Config = app.Config
type Runner = app.Runner
type GenericAlias = app.Generic[int]
type Renamed = app.Original
type External = other.External
`)
	writeFile(t, internalDir, "app.go", `package app

type Config struct {
	Name string
	private string
}

func (Config) Validate() error { return nil }
func (Config) privateMethod() {}

type Original struct{}

func (*Original) PointerValidate() error { return nil }

type Runner interface {
	Run() error
	private()
}

type Generic[T any] struct {
	Value T
}
`)

	got := Collect(t, Options{
		Dir:                 exporterDir,
		ReadDirLabel:        "exporter",
		ModulePath:          "example.test/framework",
		AliasInternalPrefix: "example.test/framework/exporter/internal/",
	})
	want := []string{
		"alias-field Config.Name string",
		"alias-field GenericAlias.Value T",
		"alias-interface Runner.Run func() error",
		"alias-method (*Renamed) PointerValidate func() error",
		"alias-method (Config) Validate func() error",
		"type Config = app.Config",
		"type External = other.External",
		"type GenericAlias = app.Generic[int]",
		"type Renamed = app.Original",
		"type Runner = app.Runner",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Collect(alias) mismatch\nwant:\n%s\n\ngot:\n%s", strings.Join(want, "\n"), strings.Join(got, "\n"))
	}
}

func TestCheckUpdatesGoldenFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "api.go", `package demo

func ExportedFunc() {}
`)
	goldenPath := filepath.Join(dir, "testdata", "public_api.txt")

	Check(t, Options{
		Dir:        dir,
		GoldenPath: goldenPath,
		Update:     true,
	})

	got, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if string(got) != "func ExportedFunc func()\n" {
		t.Fatalf("golden = %q, want exported function", got)
	}

	Check(t, Options{
		Dir:        dir,
		GoldenPath: goldenPath,
	})
}

func TestCheckReportsGoldenMismatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "api.go", `package demo

func ExportedFunc() {}
`)
	goldenPath := filepath.Join(dir, "public_api.txt")
	if err := os.WriteFile(goldenPath, []byte("func Other\n"), 0o644); err != nil {
		t.Fatalf("write golden: %v", err)
	}

	tb := &fakeTB{}
	Check(tb, Options{
		Dir:           dir,
		GoldenPath:    goldenPath,
		UpdateCommand: "go test ./demo -update-public-api",
	})
	if !tb.failed {
		t.Fatal("Check() did not fail on golden mismatch")
	}
	if !strings.Contains(tb.message, "go test ./demo -update-public-api") {
		t.Fatalf("failure message = %q, want update command", tb.message)
	}
}

type fakeTB struct {
	failed  bool
	message string
}

func (tb *fakeTB) Helper() {}

func (tb *fakeTB) Fatalf(format string, args ...any) {
	tb.failed = true
	tb.message = fmt.Sprintf(format, args...)
}

func writeFile(t *testing.T, dir string, name string, contents string) {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create dir: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
