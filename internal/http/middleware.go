package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/fairyhunter13/product-update-service-simulator/internal/obs"
	"github.com/google/uuid"
)

type ctxKey int

const (
	ctxKeyRequestID ctxKey = iota
)

func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyRequestID).(string)
	return v
}

type statusRecorder struct {
	h   http.ResponseWriter
	st  int
	n   int
}

func (w *statusRecorder) Header() http.Header { return w.h.Header() }
func (w *statusRecorder) WriteHeader(code int) {
	w.st = code
	w.h.WriteHeader(code)
}
func (w *statusRecorder) Write(b []byte) (int, error) {
	n, err := w.h.Write(b)
	w.n += n
	return n, err
}

func WithRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-Id")
		if reqID == "" {
			reqID = uuid.NewString()
		}
		w.Header().Set("X-Request-Id", reqID)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKeyRequestID, reqID)))
	})
}

func WithLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sr := &statusRecorder{h: w, st: 200}
		next.ServeHTTP(sr, r)
		lat := time.Since(start)
		obs.Logger.Info("http_request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", sr.st,
			"bytes", sr.n,
			"latency_ms", float64(lat.Microseconds())/1000.0,
			"request_id", RequestIDFromContext(r.Context()),
		)
	})
}
