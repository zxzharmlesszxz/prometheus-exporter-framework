package exporter

import (
	"net/http"

	"github.com/zxzharmlesszxz/prometheus-exporter-framework/exporter/internal/app"
)

type HandlerOptions = app.HandlerOptions

func NewHandler(opts HandlerOptions) http.Handler {
	return app.NewHandler(opts)
}

func NewHandlerChecked(opts HandlerOptions) (http.Handler, error) {
	return app.NewHandlerChecked(opts)
}
