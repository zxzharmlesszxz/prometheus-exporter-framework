package exporter

import "github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/internal/files"

func FileMTimeSeconds(path string) float64 {
	return files.FileMTimeSeconds(path)
}

type Uint64Counter = files.Uint64Counter
type FileReadFunc = files.FileReadFunc
type FileScrapeResult = files.FileScrapeResult
type FileScraper = files.FileScraper
type FileScrapeMetrics = files.FileScrapeMetrics
