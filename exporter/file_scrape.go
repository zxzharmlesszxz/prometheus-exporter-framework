package exporter

import (
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Uint64Counter interface {
	Add(uint64) uint64
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
	MTimeDesc               *prometheus.Desc
	ReadErrorsTotalDesc     *prometheus.Desc
	ParseErrorsTotalDesc    *prometheus.Desc
	ScrapeDurationDesc      *prometheus.Desc
	ReadErrorsTotal         Uint64Counter
	ParseErrorsTotal        Uint64Counter
	Now                     func() time.Time
	FileModificationSeconds func(string) float64
}

func (m FileScrapeMetrics) Describe(ch chan<- *prometheus.Desc) {
	if m.MTimeDesc != nil {
		ch <- m.MTimeDesc
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

func (m FileScrapeMetrics) Begin(ch chan<- prometheus.Metric) func() {
	start := m.now()
	if m.MTimeDesc != nil {
		ch <- prometheus.MustNewConstMetric(m.MTimeDesc, prometheus.GaugeValue, m.fileModificationSeconds(m.Path))
	}

	return func() {
		if m.ScrapeDurationDesc != nil {
			ch <- prometheus.MustNewConstMetric(m.ScrapeDurationDesc, prometheus.GaugeValue, m.since(start).Seconds())
		}
		if m.ReadErrorsTotalDesc != nil && m.ReadErrorsTotal != nil {
			ch <- prometheus.MustNewConstMetric(m.ReadErrorsTotalDesc, prometheus.CounterValue, float64(m.ReadErrorsTotal.Load()))
		}
		if m.ParseErrorsTotalDesc != nil && m.ParseErrorsTotal != nil {
			ch <- prometheus.MustNewConstMetric(m.ParseErrorsTotalDesc, prometheus.CounterValue, float64(m.ParseErrorsTotal.Load()))
		}
	}
}

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
	if parse != nil {
		if err := parse(content); err != nil {
			addCounter(s.ParseErrorsTotal)
			result.Err = err
			return result
		}
	}
	result.Up = true
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
	if m.Now != nil {
		return m.Now().Sub(start)
	}
	return time.Since(start)
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
	if s.Now != nil {
		return s.Now().Sub(start)
	}
	return time.Since(start)
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
