package smoketest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestConfigDefaultsAndMetricWants(t *testing.T) {
	t.Parallel()

	config := Config{
		ProjectName:     "demo-exporter",
		BuildInfoMetric: "demo_build_info",
		WantMetrics:     []string{"demo_value 1"},
	}
	config.setDefaults(t)

	if config.RunEnv != "RUN_BINARY_SMOKE" {
		t.Fatalf("RunEnv = %q, want RUN_BINARY_SMOKE", config.RunEnv)
	}
	if config.CommandDir != "./cmd" {
		t.Fatalf("CommandDir = %q, want ./cmd", config.CommandDir)
	}
	if config.TelemetryPath != "/metrics" {
		t.Fatalf("TelemetryPath = %q, want /metrics", config.TelemetryPath)
	}
	if config.HealthPath != "/healthz" {
		t.Fatalf("HealthPath = %q, want /healthz", config.HealthPath)
	}

	wants := strings.Join(config.metricWants(), "\n")
	for _, want := range []string{
		"demo_build_info",
		`version="` + Version + `"`,
		`branch="` + Branch + `"`,
		`revision="` + Revision + `"`,
		"demo_value 1",
	} {
		if !strings.Contains(wants, want) {
			t.Fatalf("metricWants() missing %q in %q", want, wants)
		}
	}
}

func TestConfigRejectsInvalidTelemetryPath(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"metrics",
		" /metrics",
		"/",
		"/met rics",
		"/metrics?format=openmetrics",
		"/metrics#section",
		"/prometheus//metrics",
	} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			if err := validateSmokePath("TelemetryPath", path); err == nil || !strings.Contains(err.Error(), "TelemetryPath") {
				t.Fatalf("validateSmokePath() error = %v, want TelemetryPath error", err)
			}
		})
	}
}

func TestConfigRejectsInvalidHealthPath(t *testing.T) {
	t.Parallel()

	for _, path := range []string{
		"healthz",
		" /healthz",
		"/",
		"/health z",
		"/healthz?ready=true",
		"/healthz#ready",
		"/healthz//ready",
	} {
		t.Run(path, func(t *testing.T) {
			t.Parallel()

			if err := validateSmokePath("HealthPath", path); err == nil || !strings.Contains(err.Error(), "HealthPath") {
				t.Fatalf("validateSmokePath() error = %v, want HealthPath error", err)
			}
		})
	}
}

func TestSafeBufferSerializesWritesAndReads(t *testing.T) {
	t.Parallel()

	var buffer safeBuffer
	if n, err := buffer.Write([]byte("hello")); err != nil || n != len("hello") {
		t.Fatalf("Write() = %d, %v; want %d, nil", n, err, len("hello"))
	}
	if n, err := buffer.Write([]byte(" world")); err != nil || n != len(" world") {
		t.Fatalf("Write() = %d, %v; want %d, nil", n, err, len(" world"))
	}
	if got := buffer.String(); got != "hello world" {
		t.Fatalf("String() = %q, want hello world", got)
	}
}

func TestRunBinaryWithoutServerSmoke(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake shell go command is Unix-only")
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module smoke.test\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	fakeGo := filepath.Join(t.TempDir(), "go")
	if err := os.WriteFile(fakeGo, []byte(fakeGoScript()), 0o755); err != nil {
		t.Fatalf("write fake go: %v", err)
	}

	workingDir := filepath.Join(root, "smoke")
	if err := os.Mkdir(workingDir, 0o755); err != nil {
		t.Fatalf("make working directory: %v", err)
	}

	t.Setenv("GO", fakeGo)
	t.Setenv("RUN_BINARY_SMOKE", "1")
	chdir(t, workingDir)

	RunBinary(t, Config{
		ProjectName:         "demo-exporter",
		ForbiddenUsageNames: []string{"demo_exporter"},
		RenamedExecutable:   "renamed-demo-exporter",
	})
}

func TestRunBinaryWithPrebuiltBinary(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fake shell binary is Unix-only")
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module smoke.test\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	binary := filepath.Join(root, "demo-exporter")
	if err := os.WriteFile(binary, []byte(fakeBinaryScript()), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	workingDir := filepath.Join(root, "smoke")
	if err := os.Mkdir(workingDir, 0o755); err != nil {
		t.Fatalf("make working directory: %v", err)
	}

	t.Setenv("GO", filepath.Join(root, "missing-go"))
	t.Setenv("RUN_BINARY_SMOKE", "1")
	chdir(t, workingDir)

	RunBinary(t, Config{
		ProjectName:         "demo-exporter",
		BinaryPath:          "demo-exporter",
		ForbiddenUsageNames: []string{"demo_exporter"},
		RenamedExecutable:   "renamed-demo-exporter",
	})
}

func TestRunBinaryWithServerSmoke(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module smoke.test\n\ngo 1.27.0\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	cmdDir := filepath.Join(root, "cmd")
	if err := os.Mkdir(cmdDir, 0o755); err != nil {
		t.Fatalf("make cmd directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cmdDir, "main.go"), []byte(fixtureExporterMain()), 0o644); err != nil {
		t.Fatalf("write fixture main.go: %v", err)
	}

	workingDir := filepath.Join(root, "smoke")
	if err := os.Mkdir(workingDir, 0o755); err != nil {
		t.Fatalf("make working directory: %v", err)
	}
	chdir(t, workingDir)
	t.Setenv("RUN_BINARY_SMOKE", "1")

	RunBinary(t, Config{
		ProjectName:         "demo-exporter",
		BuildInfoMetric:     "demo_exporter_build_info",
		ForbiddenUsageNames: []string{"demo_exporter"},
		RenamedExecutable:   "renamed-demo-exporter",
		ServerArgs: func(_ *testing.T, _ string) []string {
			return []string{"--demo.refresh-interval=100ms"}
		},
		WantMetrics: []string{
			"demo_last_collection_success 1",
		},
		RejectMetrics: []string{
			"demo_last_collection_success 0",
		},
	})
}

func TestWaitForMetricsWaitsUntilAllWantedTextAppears(t *testing.T) {
	t.Parallel()

	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch r.URL.Path {
		case "/metrics":
			if requests == 1 {
				_, _ = w.Write([]byte("demo_value 0\n"))
				return
			}
			_, _ = w.Write([]byte("demo_value 1\ndemo_other 2\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	config := Config{
		ProjectName:     "demo-exporter",
		TelemetryPath:   "/metrics",
		StartupTimeout:  time.Second,
		HTTPTimeout:     time.Second,
		ServerTestName:  "server",
		HealthPath:      "/healthz",
		RunEnv:          "RUN_BINARY_SMOKE",
		CommandDir:      "./cmd",
		BuildInfoMetric: "demo_build_info",
	}
	waitCh := make(chan error)
	var stdout safeBuffer
	var stderr safeBuffer
	metrics := waitForMetrics(t, server.Client(), server.URL, config, waitCh, &stdout, &stderr, []string{
		"demo_value 1",
		"demo_other 2",
	})
	if !strings.Contains(metrics, "demo_value 1") {
		t.Fatalf("waitForMetrics() = %q, want demo_value 1", metrics)
	}
}

func TestContainsAll(t *testing.T) {
	t.Parallel()

	if !containsAll("first second", []string{"first", "second"}) {
		t.Fatal("containsAll() = false, want true")
	}
	if containsAll("first", []string{"first", "second"}) {
		t.Fatal("containsAll() = true, want false")
	}
}

func fakeGoScript() string {
	return fmt.Sprintf(`#!/bin/sh
out=""
while [ "$#" -gt 0 ]; do
	case "$1" in
		-o)
			shift
			out="$1"
			;;
	esac
	shift
done

cat > "$out" <<'EOF'
#!/bin/sh
case "$1" in
	--version)
		echo "%[1]s %[2]s %[3]s"
		exit 0
		;;
	--help)
		echo "usage: $(basename "$0") [<flags>]"
		exit 0
		;;
	--web.telemetry-path=metrics)
		echo 'invalid --web.telemetry-path "metrics"'
		exit 1
		;;
esac
echo "unexpected args: $*" >&2
exit 1
EOF
chmod +x "$out"
`, Version, Branch, Revision)
}

func fakeBinaryScript() string {
	return fmt.Sprintf(`#!/bin/sh
case "$1" in
	--version)
		echo "%[1]s %[2]s %[3]s"
		exit 0
		;;
	--help)
		echo "usage: $(basename "$0") [<flags>]"
		exit 0
		;;
	--web.telemetry-path=metrics)
		echo 'invalid --web.telemetry-path "metrics"'
		exit 1
		;;
esac
echo "unexpected args: $*" >&2
exit 1
`, Version, Branch, Revision)
}

func fixtureExporterMain() string {
	return fmt.Sprintf(`package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	listenAddress := "127.0.0.1:0"
	telemetryPath := "/metrics"
	for _, arg := range os.Args[1:] {
		switch {
		case arg == "--version":
			fmt.Println("%[1]s %[2]s %[3]s")
			return
		case arg == "--help":
			fmt.Printf("usage: %%s [<flags>]\n", filepath.Base(os.Args[0]))
			return
		case arg == "--web.telemetry-path=metrics":
			fmt.Println("invalid --web.telemetry-path \"metrics\"")
			os.Exit(1)
		case strings.HasPrefix(arg, "--web.listen-address="):
			listenAddress = strings.TrimPrefix(arg, "--web.listen-address=")
		case strings.HasPrefix(arg, "--web.telemetry-path="):
			telemetryPath = strings.TrimPrefix(arg, "--web.telemetry-path=")
		}
	}

	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok\n"))
	})
	http.HandleFunc(telemetryPath, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("demo_exporter_build_info{version=\"%[1]s\",branch=\"%[2]s\",revision=\"%[3]s\"} 1\n"))
		_, _ = w.Write([]byte("demo_last_collection_success 1\n"))
	})
	log.Fatal(http.ListenAndServe(listenAddress, nil))
}
`, Version, Branch, Revision)
}

func chdir(t *testing.T, dir string) {
	t.Helper()

	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir to fixture: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(currentDir); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	})
}
