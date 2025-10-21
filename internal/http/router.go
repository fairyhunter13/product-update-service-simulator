package httpapi

import (
	"expvar"
	"net/http"
)

// NewRouter registers HTTP routes and returns the handler with middleware.
func NewRouter(app *App) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/events", app.postEventsHandler)
	mux.HandleFunc("/products/", app.getProductHandler)
	mux.HandleFunc("/healthz", app.healthHandler)
	mux.HandleFunc("/debug/metrics", app.metricsHandler)
	mux.Handle("/debug/vars", expvar.Handler())
	mux.HandleFunc("/openapi.yaml", app.openapiHandler)
	mux.HandleFunc("/docs", app.docsHandler)
	return WithRequestID(WithLogging(mux))
}
