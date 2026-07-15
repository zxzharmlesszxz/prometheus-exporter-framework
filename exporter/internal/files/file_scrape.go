package files

import (
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Uint64Counter is the minimal atomic counter contract used by file scrape
// helpers. *atomic.Uint64 satisfies this interface.
type Uint64Counter interface {
	// Add must atomically add delta and return the new value.
	Add(uint64) uint64
	// Load must atomically return the current cumulative value.
	Load() uint64
}

type FileReadFunc func(path string) ([]byte, error)

type FileScrapeResult struct {
	Path                  string
	Up                    bool
	MTimeSeconds          float64
	ReadErrorsTotal       uint64
	ParseErrorsTotal      uint64
	ScrapeDurationSeconds float64
	Err                   error
}

type FileScraper struct {
	ReadErrorsTotal         Uint64Counter
	ParseErrorsTotal        Uint64Counter
	Now                     func() time.Time
	ReadFile                FileReadFunc
	FileModificationSeconds func(string) float64
}

type FileScrapeMetrics struct {
	Path                    string
	LabelValues             []string
	MTimeDesc               *prometheus.Desc
	UpDesc                  *prometheus.Desc
	ValidDesc               *prometheus.Desc
	ReadErrorsTotalDesc     *prometheus.Desc
	ParseErrorsTotalDesc    *prometheus.Desc
	ScrapeDurationDesc      *prometheus.Desc
	ReadErrorsTotal         Uint64Counter
	ParseErrorsTotal        Uint64Counter
	Now                     func() time.Time
	FileModificationSeconds func(string) float64
}

// Describe emits all configured file/source scrape descriptors.
func (m FileScrapeMetrics) Describe(ch chan<- *prometheus.Desc) {
	if m.MTimeDesc != nil {
		ch <- m.MTimeDesc
	}
	if m.UpDesc != nil {
		ch <- m.UpDesc
	}
	if m.ValidDesc != nil {
		ch <- m.ValidDesc
	}
	if m.ReadErrorsTotalDesc != nil {
		ch <- m.ReadErrorsTotalDesc
	}
	if m.ParseErrorsTotalDesc != nil {
		ch <- m.ParseErrorsTotalDesc
	}
	if m.ScrapeDurationDesc != nil {
		ch <- m.ScrapeDurationDesc
	}
}

// Begin starts scrape-time metric collection for callers that perform source
// work directly inside Collect. It emits mtime immediately and returns a finish
// callback that emits scrape duration and shared cumulative counters.
//
// Do not combine Begin with CollectResult for the same descriptors in one
// Collect call: both modes emit mtime and duration metrics.
func (m FileScrapeMetrics) Begin(ch chan<- prometheus.Metric) func() {
	start := m.now()
	if m.MTimeDesc != nil {
		ch <- prometheus.MustNewConstMetric(m.MTimeDesc, prometheus.GaugeValue, m.fileModificationSeconds(m.Path), m.LabelValues...)
	}

	return func() {
		if m.ScrapeDurationDesc != nil {
			ch <- prometheus.MustNewConstMetric(m.ScrapeDurationDesc, prometheus.GaugeValue, m.since(start).Seconds(), m.LabelValues...)
		}
		if m.ReadErrorsTotalDesc != nil && m.ReadErrorsTotal != nil {
			ch <- prometheus.MustNewConstMetric(m.ReadErrorsTotalDesc, prometheus.CounterValue, float64(m.ReadErrorsTotal.Load()), m.LabelValues...)
		}
		if m.ParseErrorsTotalDesc != nil && m.ParseErrorsTotal != nil {
			ch <- prometheus.MustNewConstMetric(m.ParseErrorsTotalDesc, prometheus.CounterValue, float64(m.ParseErrorsTotal.Load()), m.LabelValues...)
		}
	}
}

// CollectResult emits metrics from a precomputed FileScrapeResult, usually one
// stored in a snapshot. Use this mode instead of Begin when source work already
// happened before Collect.
func (m FileScrapeMetrics) CollectResult(ch chan<- prometheus.Metric, result FileScrapeResult) {
	if m.MTimeDesc != nil {
		ch <- prometheus.MustNewConstMetric(m.MTimeDesc, prometheus.GaugeValue, result.MTimeSeconds, m.LabelValues...)
	}
	if m.UpDesc != nil {
		ch <- prometheus.MustNewConstMetric(m.UpDesc, prometheus.GaugeValue, boolFloat(result.Up), m.LabelValues...)
	}
	if m.ScrapeDurationDesc != nil {
		ch <- prometheus.MustNewConstMetric(m.ScrapeDurationDesc, prometheus.GaugeValue, result.ScrapeDurationSeconds, m.LabelValues...)
	}
	if m.ReadErrorsTotalDesc != nil {
		ch <- prometheus.MustNewConstMetric(m.ReadErrorsTotalDesc, prometheus.CounterValue, float64(result.ReadErrorsTotal), m.LabelValues...)
	}
	if m.ParseErrorsTotalDesc != nil {
		ch <- prometheus.MustNewConstMetric(m.ParseErrorsTotalDesc, prometheus.CounterValue, float64(result.ParseErrorsTotal), m.LabelValues...)
	}
}

// CollectValid emits the domain-defined validity gauge for the source.
func (m FileScrapeMetrics) CollectValid(ch chan<- prometheus.Metric, valid bool) {
	if m.ValidDesc != nil {
		ch <- prometheus.MustNewConstMetric(m.ValidDesc, prometheus.GaugeValue, boolFloat(valid), m.LabelValues...)
	}
}

// Scrape reads path, runs parse, and returns source-health bookkeeping. The
// error counters are cumulative counters from the FileScraper, not per-call
// error counts. Use separate counters per source when per-source totals are
// required.
func (s FileScraper) Scrape(path string, parse func([]byte) error) (result FileScrapeResult) {
	start := s.now()
	result = FileScrapeResult{
		Path:         path,
		MTimeSeconds: s.fileModificationSeconds(path),
	}
	defer func() {
		result.ReadErrorsTotal = loadCounter(s.ReadErrorsTotal)
		result.ParseErrorsTotal = loadCounter(s.ParseErrorsTotal)
		result.ScrapeDurationSeconds = s.since(start).Seconds()
	}()

	content, err := s.readFile(path)
	if err != nil {
		addCounter(s.ReadErrorsTotal)
		result.Err = err
		return result
	}
	result.Up = true
	if parse != nil {
		if err := parse(content); err != nil {
			addCounter(s.ParseErrorsTotal)
			result.Err = err
			return result
		}
	}
	return result
}

func (m FileScrapeMetrics) AddReadError() {
	if m.ReadErrorsTotal != nil {
		m.ReadErrorsTotal.Add(1)
	}
}

func (m FileScrapeMetrics) AddParseError() {
	if m.ParseErrorsTotal != nil {
		m.ParseErrorsTotal.Add(1)
	}
}

func (m FileScrapeMetrics) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

func (m FileScrapeMetrics) since(start time.Time) time.Duration {
	var duration time.Duration
	if m.Now != nil {
		duration = m.Now().Sub(start)
	} else {
		duration = time.Since(start)
	}
	if duration < 0 {
		return 0
	}
	return duration
}

func (m FileScrapeMetrics) fileModificationSeconds(path string) float64 {
	if m.FileModificationSeconds != nil {
		return m.FileModificationSeconds(path)
	}
	return FileMTimeSeconds(path)
}

func (s FileScraper) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s FileScraper) since(start time.Time) time.Duration {
	var duration time.Duration
	if s.Now != nil {
		duration = s.Now().Sub(start)
	} else {
		duration = time.Since(start)
	}
	if duration < 0 {
		return 0
	}
	return duration
}

func (s FileScraper) readFile(path string) ([]byte, error) {
	if s.ReadFile != nil {
		return s.ReadFile(path)
	}
	return os.ReadFile(path)
}

func (s FileScraper) fileModificationSeconds(path string) float64 {
	if s.FileModificationSeconds != nil {
		return s.FileModificationSeconds(path)
	}
	return FileMTimeSeconds(path)
}

func addCounter(counter Uint64Counter) {
	if counter != nil {
		counter.Add(1)
	}
}

func loadCounter(counter Uint64Counter) uint64 {
	if counter == nil {
		return 0
	}
	return counter.Load()
}

func boolFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
