package files

import (
	"errors"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/exportertest"
)

func TestFileScrapeMetricsDescribeAndCollect(t *testing.T) {
	t.Parallel()

	now := time.Unix(1_700_000_000, 0)
	readErrors := atomic.Uint64{}
	parseErrors := atomic.Uint64{}
	mtimeDesc := prometheus.NewDesc("file_mtime_seconds", "File mtime.", nil, nil)
	readErrorsDesc := prometheus.NewDesc("file_read_errors_total", "File read errors.", nil, nil)
	parseErrorsDesc := prometheus.NewDesc("file_parse_errors_total", "File parse errors.", nil, nil)
	durationDesc := prometheus.NewDesc("file_scrape_duration_seconds", "File scrape duration.", nil, nil)

	collector := fileScrapeTestCollector{
		metrics: FileScrapeMetrics{
			Path:                 "/tmp/input",
			MTimeDesc:            mtimeDesc,
			ReadErrorsTotalDesc:  readErrorsDesc,
			ParseErrorsTotalDesc: parseErrorsDesc,
			ScrapeDurationDesc:   durationDesc,
			ReadErrorsTotal:      &readErrors,
			ParseErrorsTotal:     &parseErrors,
			Now: func() time.Time {
				return now
			},
			FileModificationSeconds: func(path string) float64 {
				if path != "/tmp/input" {
					t.Fatalf("path = %q, want /tmp/input", path)
				}
				return 123
			},
		},
		collect: func(metrics FileScrapeMetrics, _ chan<- prometheus.Metric) {
			now = now.Add(250 * time.Millisecond)
			metrics.AddReadError()
			metrics.AddParseError()
		},
	}

	families := exportertest.RegisterAndGather(t, collector)
	exportertest.AssertMetricValue(t, families, "file_mtime_seconds", nil, 123)
	exportertest.AssertMetricValue(t, families, "file_read_errors_total", nil, 1)
	exportertest.AssertMetricValue(t, families, "file_parse_errors_total", nil, 1)
	exportertest.AssertMetricValue(t, families, "file_scrape_duration_seconds", nil, 0.25)
}

func TestFileScrapeMetricsAllowsOptionalCounters(t *testing.T) {
	t.Parallel()

	mtimeDesc := prometheus.NewDesc("optional_file_mtime_seconds", "File mtime.", nil, nil)
	collector := fileScrapeTestCollector{
		metrics: FileScrapeMetrics{
			Path:      "/tmp/input",
			MTimeDesc: mtimeDesc,
			FileModificationSeconds: func(string) float64 {
				return 456
			},
		},
		collect: func(metrics FileScrapeMetrics, _ chan<- prometheus.Metric) {
			metrics.AddReadError()
			metrics.AddParseError()
		},
	}

	families := exportertest.RegisterAndGather(t, collector)
	exportertest.AssertMetricValue(t, families, "optional_file_mtime_seconds", nil, 456)
}

func TestFileScrapeMetricsCollectResultWithLabels(t *testing.T) {
	t.Parallel()

	mtimeDesc := prometheus.NewDesc("labeled_file_mtime_seconds", "File mtime.", []string{"path"}, nil)
	upDesc := prometheus.NewDesc("labeled_file_up", "File up.", []string{"path"}, nil)
	validDesc := prometheus.NewDesc("labeled_file_valid", "File valid.", []string{"path"}, nil)
	readErrorsDesc := prometheus.NewDesc("labeled_file_read_errors_total", "File read errors.", []string{"path"}, nil)
	parseErrorsDesc := prometheus.NewDesc("labeled_file_parse_errors_total", "File parse errors.", []string{"path"}, nil)
	durationDesc := prometheus.NewDesc("labeled_file_scrape_duration_seconds", "File scrape duration.", []string{"path"}, nil)
	collector := fileScrapeTestCollector{
		skipBegin: true,
		metrics: FileScrapeMetrics{
			LabelValues:          []string{"/tmp/input"},
			MTimeDesc:            mtimeDesc,
			UpDesc:               upDesc,
			ValidDesc:            validDesc,
			ReadErrorsTotalDesc:  readErrorsDesc,
			ParseErrorsTotalDesc: parseErrorsDesc,
			ScrapeDurationDesc:   durationDesc,
		},
		collect: func(metrics FileScrapeMetrics, ch chan<- prometheus.Metric) {
			metrics.CollectResult(ch, FileScrapeResult{
				Up:                    true,
				MTimeSeconds:          123,
				ReadErrorsTotal:       2,
				ParseErrorsTotal:      3,
				ScrapeDurationSeconds: 0.25,
			})
			metrics.CollectValid(ch, false)
		},
	}

	families := exportertest.RegisterAndGather(t, collector)
	labels := map[string]string{"path": "/tmp/input"}
	exportertest.AssertMetricValue(t, families, "labeled_file_mtime_seconds", labels, 123)
	exportertest.AssertMetricValue(t, families, "labeled_file_up", labels, 1)
	exportertest.AssertMetricValue(t, families, "labeled_file_valid", labels, 0)
	exportertest.AssertMetricValue(t, families, "labeled_file_read_errors_total", labels, 2)
	exportertest.AssertMetricValue(t, families, "labeled_file_parse_errors_total", labels, 3)
	exportertest.AssertMetricValue(t, families, "labeled_file_scrape_duration_seconds", labels, 0.25)
}

func TestFileScrapeMetricsUsesDefaultHooks(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/input.txt"
	if err := os.WriteFile(path, []byte("payload"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	mtimeDesc := prometheus.NewDesc("default_file_mtime_seconds", "File mtime.", nil, nil)
	durationDesc := prometheus.NewDesc("default_file_scrape_duration_seconds", "File scrape duration.", nil, nil)
	collector := fileScrapeTestCollector{
		metrics: FileScrapeMetrics{
			Path:               path,
			MTimeDesc:          mtimeDesc,
			ScrapeDurationDesc: durationDesc,
		},
	}

	families := exportertest.RegisterAndGather(t, collector)
	mtime, ok := exportertest.MetricValue(families, "default_file_mtime_seconds", nil)
	if !ok {
		t.Fatal("default_file_mtime_seconds metric not found")
	}
	if mtime <= 0 {
		t.Fatalf("default_file_mtime_seconds = %v, want positive value", mtime)
	}
	duration, ok := exportertest.MetricValue(families, "default_file_scrape_duration_seconds", nil)
	if !ok {
		t.Fatal("default_file_scrape_duration_seconds metric not found")
	}
	if duration < 0 {
		t.Fatalf("default_file_scrape_duration_seconds = %v, want non-negative value", duration)
	}
}

func TestFileScraperScrape(t *testing.T) {
	t.Parallel()

	readErr := errors.New("read failed")
	parseErr := errors.New("parse failed")

	for _, tc := range []struct {
		name            string
		readFile        FileReadFunc
		parse           func([]byte) error
		wantUp          bool
		wantErr         error
		wantReadErrors  uint64
		wantParseErrors uint64
	}{
		{
			name: "success",
			readFile: func(path string) ([]byte, error) {
				return []byte("payload"), nil
			},
			parse: func(content []byte) error {
				return nil
			},
			wantUp: true,
		},
		{
			name: "read error",
			readFile: func(string) ([]byte, error) {
				return nil, readErr
			},
			parse: func([]byte) error {
				return nil
			},
			wantErr:        readErr,
			wantReadErrors: 1,
		},
		{
			name: "parse error",
			readFile: func(string) ([]byte, error) {
				return []byte("payload"), nil
			},
			parse: func([]byte) error {
				return parseErr
			},
			wantErr:         parseErr,
			wantParseErrors: 1,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			now := time.Unix(1_700_000_000, 0)
			readErrors := atomic.Uint64{}
			parseErrors := atomic.Uint64{}
			scraper := FileScraper{
				ReadErrorsTotal:  &readErrors,
				ParseErrorsTotal: &parseErrors,
				Now: func() time.Time {
					current := now
					now = now.Add(125 * time.Millisecond)
					return current
				},
				ReadFile: tc.readFile,
				FileModificationSeconds: func(path string) float64 {
					if path != "/tmp/input" {
						t.Fatalf("mtime path = %q, want /tmp/input", path)
					}
					return 123
				},
			}

			result := scraper.Scrape("/tmp/input", tc.parse)
			if result.Path != "/tmp/input" {
				t.Fatalf("Path = %q, want /tmp/input", result.Path)
			}
			if result.Up != tc.wantUp {
				t.Fatalf("Up = %v, want %v", result.Up, tc.wantUp)
			}
			if !errors.Is(result.Err, tc.wantErr) {
				t.Fatalf("Err = %v, want %v", result.Err, tc.wantErr)
			}
			if result.MTimeSeconds != 123 {
				t.Fatalf("MTimeSeconds = %v, want 123", result.MTimeSeconds)
			}
			if result.ReadErrorsTotal != tc.wantReadErrors {
				t.Fatalf("ReadErrorsTotal = %d, want %d", result.ReadErrorsTotal, tc.wantReadErrors)
			}
			if result.ParseErrorsTotal != tc.wantParseErrors {
				t.Fatalf("ParseErrorsTotal = %d, want %d", result.ParseErrorsTotal, tc.wantParseErrors)
			}
			if result.ScrapeDurationSeconds != 0.125 {
				t.Fatalf("ScrapeDurationSeconds = %v, want 0.125", result.ScrapeDurationSeconds)
			}
		})
	}
}

func TestFileScraperScrapeUsesDefaultHooks(t *testing.T) {
	t.Parallel()

	path := t.TempDir() + "/input.txt"
	if err := os.WriteFile(path, []byte("payload"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	result := (FileScraper{}).Scrape(path, nil)
	if !result.Up {
		t.Fatalf("Up = false, want true; err = %v", result.Err)
	}
	if result.MTimeSeconds <= 0 {
		t.Fatalf("MTimeSeconds = %v, want positive value", result.MTimeSeconds)
	}
	if result.ReadErrorsTotal != 0 {
		t.Fatalf("ReadErrorsTotal = %d, want 0", result.ReadErrorsTotal)
	}
	if result.ParseErrorsTotal != 0 {
		t.Fatalf("ParseErrorsTotal = %d, want 0", result.ParseErrorsTotal)
	}
	if result.ScrapeDurationSeconds < 0 {
		t.Fatalf("ScrapeDurationSeconds = %v, want non-negative value", result.ScrapeDurationSeconds)
	}
}

type fileScrapeTestCollector struct {
	metrics   FileScrapeMetrics
	collect   func(FileScrapeMetrics, chan<- prometheus.Metric)
	skipBegin bool
}

func (c fileScrapeTestCollector) Describe(ch chan<- *prometheus.Desc) {
	c.metrics.Describe(ch)
}

func (c fileScrapeTestCollector) Collect(ch chan<- prometheus.Metric) {
	if !c.skipBegin {
		finish := c.metrics.Begin(ch)
		defer finish()
	}
	if c.collect != nil {
		c.collect(c.metrics, ch)
	}
}
