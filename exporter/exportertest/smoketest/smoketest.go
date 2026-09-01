package smoketest

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	Version   = "v9.8.7"
	Branch    = "smoke-branch"
	Revision  = "abc123def"
	BuildUser = "smoke-test"
	BuildDate = "2026-05-17T00:00:00Z"
)

const serverStartAttempts = 5

type ServerArgsFunc func(t *testing.T, root string) []string

type Config struct {
	ProjectName         string
	BinaryPath          string
	BuildInfoMetric     string
	ForbiddenUsageNames []string
	RenamedExecutable   string
	ServerTestName      string
	ServerArgs          ServerArgsFunc
	WantMetrics         []string
	RejectMetrics       []string
	RunEnv              string
	CommandDir          string
	TelemetryPath       string
	HealthPath          string
	StartupTimeout      time.Duration
	HTTPTimeout         time.Duration
}

func RunBinary(t *testing.T, config Config) {
	t.Helper()

	config.setDefaults(t)
	if os.Getenv(config.RunEnv) != "1" {
		t.Skipf("set %s=1 to run binary smoke test", config.RunEnv)
	}

	root := repoRoot(t)
	binary := testBinary(t, root, config)

	t.Run("prints injected version metadata", func(t *testing.T) {
		output := runBinary(t, binary, "--version")
		for _, want := range []string{Version, Branch, Revision} {
			if !strings.Contains(output, want) {
				t.Fatalf("--version output missing %q: %s", want, output)
			}
		}
	})

	t.Run("prints concrete binary name in help usage", func(t *testing.T) {
		output := runBinary(t, binary, "--help")
		want := "usage: " + config.ProjectName + " [<flags>]"
		if !strings.Contains(output, want) {
			t.Fatalf("--help output missing concrete usage name %q: %s", want, output)
		}
		for _, name := range config.ForbiddenUsageNames {
			forbidden := "usage: " + name + " [<flags>]"
			if strings.Contains(output, forbidden) {
				t.Fatalf("--help output uses forbidden usage name %q: %s", name, output)
			}
		}
	})

	if config.RenamedExecutable != "" {
		t.Run("uses executable file name in help usage", func(t *testing.T) {
			renamedName := config.RenamedExecutable
			if runtime.GOOS == "windows" && !strings.HasSuffix(renamedName, ".exe") {
				renamedName += ".exe"
			}
			renamed := copyBinary(t, binary, renamedName)

			output := runBinary(t, renamed, "--help")
			want := "usage: " + renamedName + " [<flags>]"
			if !strings.Contains(output, want) {
				t.Fatalf("--help output missing executable file name usage %q: %s", want, output)
			}
		})
	}

	t.Run("rejects invalid telemetry path", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, binary, "--web.telemetry-path=metrics")
		output, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("invalid telemetry path exited successfully, output: %s", output)
		}
		if !strings.Contains(string(output), `invalid --web.telemetry-path "metrics"`) {
			t.Fatalf("invalid telemetry path output missing validation error: %s", output)
		}
	})

	wants := config.metricWants()
	if len(wants) > 0 || len(config.RejectMetrics) > 0 || config.ServerArgs != nil {
		t.Run(config.ServerTestName, func(t *testing.T) {
			runServerSmoke(t, root, binary, config, wants)
		})
	}
}

func (c *Config) setDefaults(t *testing.T) {
	t.Helper()

	if c.ProjectName == "" {
		t.Fatal("smoketest.Config.ProjectName is required")
	}
	if c.RunEnv == "" {
		c.RunEnv = "RUN_BINARY_SMOKE"
	}
	if c.CommandDir == "" {
		c.CommandDir = "./cmd"
	}
	if c.TelemetryPath == "" {
		c.TelemetryPath = "/metrics"
	}
	if c.HealthPath == "" {
		c.HealthPath = "/healthz"
	}
	if c.ServerTestName == "" {
		c.ServerTestName = "serves health and metrics"
	}
	if err := validateSmokePath("TelemetryPath", c.TelemetryPath); err != nil {
		t.Fatal(err)
	}
	if err := validateSmokePath("HealthPath", c.HealthPath); err != nil {
		t.Fatal(err)
	}
	if c.StartupTimeout == 0 {
		c.StartupTimeout = 10 * time.Second
	}
	if c.HTTPTimeout == 0 {
		c.HTTPTimeout = 500 * time.Millisecond
	}
}

func validateSmokePath(name string, value string) error {
	if strings.TrimSpace(value) != value {
		return fmt.Errorf("smoketest.Config.%s = %q, want path without leading or trailing whitespace", name, value)
	}
	if !strings.HasPrefix(value, "/") {
		return fmt.Errorf("smoketest.Config.%s = %q, want absolute path starting with /", name, value)
	}
	if value == "/" {
		return fmt.Errorf("smoketest.Config.%s = %q, want non-root path", name, value)
	}
	if strings.ContainsAny(value, " \t\r\n") {
		return fmt.Errorf("smoketest.Config.%s = %q, want path without whitespace", name, value)
	}
	if strings.ContainsAny(value, "?#") {
		return fmt.Errorf("smoketest.Config.%s = %q, want path without query string or fragment", name, value)
	}
	if strings.Contains(value, "//") {
		return fmt.Errorf("smoketest.Config.%s = %q, want path without double slashes", name, value)
	}
	return nil
}

func (c *Config) metricWants() []string {
	var wants []string
	if c.BuildInfoMetric != "" {
		wants = append(wants,
			c.BuildInfoMetric,
			`version="`+Version+`"`,
			`branch="`+Branch+`"`,
			`revision="`+Revision+`"`,
		)
	}
	wants = append(wants, c.WantMetrics...)
	return wants
}

func runServerSmoke(t *testing.T, root string, binary string, config Config, wants []string) {
	t.Helper()

	var lastStartupExit string
	for attempt := 1; attempt <= serverStartAttempts; attempt++ {
		retry, startupExit := runServerSmokeAttempt(t, root, binary, config, wants)
		if !retry {
			return
		}
		lastStartupExit = startupExit
	}
	t.Fatalf("server exited during startup after %d attempts\n%s", serverStartAttempts, lastStartupExit)
}

func runServerSmokeAttempt(t *testing.T, root string, binary string, config Config, wants []string) (bool, string) {
	t.Helper()

	addr := freeAddress(t)
	baseURL := "http://" + addr
	ctx, cancel := context.WithCancel(context.Background())

	args := []string{
		"--log.level=error",
		"--web.listen-address=" + addr,
		"--web.telemetry-path=" + config.TelemetryPath,
	}
	if config.ServerArgs != nil {
		args = append(args, config.ServerArgs(t, root)...)
	}

	var stdout safeBuffer
	var stderr safeBuffer
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatalf("start binary: %v", err)
	}
	waitCh := make(chan error, 1)
	go func() {
		waitCh <- cmd.Wait()
		close(waitCh)
	}()

	select {
	case err := <-waitCh:
		cancel()
		return true, fmt.Sprintf(
			"attempt address: %s\nexit: %v\nstdout:\n%s\nstderr:\n%s",
			addr,
			err,
			stdout.String(),
			stderr.String(),
		)
	case <-time.After(50 * time.Millisecond):
	}

	t.Cleanup(func() {
		cancel()
		select {
		case <-waitCh:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			<-waitCh
		}
	})

	client := &http.Client{Timeout: config.HTTPTimeout}
	waitForHealth(t, client, baseURL, config, waitCh, &stdout, &stderr)

	health := httpGet(t, client, baseURL+config.HealthPath, http.StatusOK)
	if health != "ok\n" {
		t.Fatalf("GET %s body = %q, want %q", config.HealthPath, health, "ok\n")
	}

	metrics := waitForMetrics(t, client, baseURL, config, waitCh, &stdout, &stderr, wants)
	for _, reject := range config.RejectMetrics {
		if strings.Contains(metrics, reject) {
			t.Fatalf("GET %s body contains rejected metric %q:\n%s", config.TelemetryPath, reject, metrics)
		}
	}
	return false, ""
}

func repoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repository root from working directory")
		}
		dir = parent
	}
}

func buildBinary(t *testing.T, root string, config Config) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), config.ProjectName)
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}

	ldflags := strings.Join([]string{
		"-s",
		"-w",
		"-X github.com/prometheus/common/version.Version=" + Version,
		"-X github.com/prometheus/common/version.Branch=" + Branch,
		"-X github.com/prometheus/common/version.Revision=" + Revision,
		"-X github.com/prometheus/common/version.BuildUser=" + BuildUser,
		"-X github.com/prometheus/common/version.BuildDate=" + BuildDate,
	}, " ")

	cmd := exec.Command(goCommand(), "build", "-trimpath", "-ldflags", ldflags, "-o", binary, config.CommandDir)
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, output)
	}
	return binary
}

func testBinary(t *testing.T, root string, config Config) string {
	t.Helper()

	if config.BinaryPath == "" {
		return buildBinary(t, root, config)
	}

	binary := config.BinaryPath
	if !filepath.IsAbs(binary) {
		binary = filepath.Join(root, binary)
	}
	if _, err := os.Stat(binary); err != nil {
		t.Fatalf("smoke binary %s: %v", binary, err)
	}
	return binary
}

func runBinary(t *testing.T, binary string, args ...string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", binary, strings.Join(args, " "), err, output)
	}
	return string(output)
}

func copyBinary(t *testing.T, source string, name string) string {
	t.Helper()

	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read binary %s: %v", source, err)
	}

	target := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(target, data, 0o755); err != nil {
		t.Fatalf("write copied binary %s: %v", target, err)
	}
	return target
}

func freeAddress(t *testing.T) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate free address: %v", err)
	}
	defer func() {
		_ = listener.Close()
	}()
	return listener.Addr().String()
}

func waitForHealth(
	t *testing.T,
	client *http.Client,
	baseURL string,
	config Config,
	waitCh <-chan error,
	stdout *safeBuffer,
	stderr *safeBuffer,
) {
	t.Helper()

	deadline := time.Now().Add(config.StartupTimeout)
	for time.Now().Before(deadline) {
		failIfExited(t, waitCh, "health check", stdout, stderr)

		resp, err := client.Get(baseURL + config.HealthPath)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for %s\nstdout:\n%s\nstderr:\n%s", config.HealthPath, stdout.String(), stderr.String())
}

func waitForMetrics(
	t *testing.T,
	client *http.Client,
	baseURL string,
	config Config,
	waitCh <-chan error,
	stdout *safeBuffer,
	stderr *safeBuffer,
	wants []string,
) string {
	t.Helper()

	deadline := time.Now().Add(config.StartupTimeout)
	var metrics string
	for time.Now().Before(deadline) {
		failIfExited(t, waitCh, "metrics check", stdout, stderr)

		metrics = httpGet(t, client, baseURL+config.TelemetryPath, http.StatusOK)
		if containsAll(metrics, wants) {
			return metrics
		}
		time.Sleep(50 * time.Millisecond)
	}

	t.Fatalf("timed out waiting for metrics %v\nlast metrics:\n%s\nstdout:\n%s\nstderr:\n%s", wants, metrics, stdout.String(), stderr.String())
	return ""
}

func failIfExited(t *testing.T, waitCh <-chan error, stage string, stdout *safeBuffer, stderr *safeBuffer) {
	t.Helper()

	select {
	case err, ok := <-waitCh:
		if ok {
			t.Fatalf("server exited before %s passed: %v\nstdout:\n%s\nstderr:\n%s", stage, err, stdout.String(), stderr.String())
		}
		t.Fatalf("server exited before %s passed\nstdout:\n%s\nstderr:\n%s", stage, stdout.String(), stderr.String())
	default:
	}
}

func httpGet(t *testing.T, client *http.Client, url string, wantStatus int) string {
	t.Helper()

	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read GET %s response body: %v", url, err)
	}
	if resp.StatusCode != wantStatus {
		t.Fatalf("GET %s status = %d, want %d; body: %s", url, resp.StatusCode, wantStatus, body)
	}
	return string(body)
}

func containsAll(text string, wants []string) bool {
	for _, want := range wants {
		if !strings.Contains(text, want) {
			return false
		}
	}
	return true
}

func goCommand() string {
	if goEnv := os.Getenv("GO"); goEnv != "" {
		return goEnv
	}
	return "go"
}

type safeBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.String()
}
