package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/fairyhunter13/product-update-service-simulator/internal/config"
	"github.com/fairyhunter13/product-update-service-simulator/internal/model"
	"github.com/fairyhunter13/product-update-service-simulator/internal/obs"
	httpopenapi "github.com/fairyhunter13/product-update-service-simulator/internal/http/openapi"
	"github.com/fairyhunter13/product-update-service-simulator/internal/queue"
	"github.com/fairyhunter13/product-update-service-simulator/internal/store"
)

type App struct {
	Cfg      config.Config
	Store    *store.Store
	Manager  *queue.Manager
	closing  bool
	started  time.Time
}

type ack struct {
	Status      string `json:"status"`
	RequestID   string `json:"request_id"`
	Sequence    uint64 `json:"sequence"`
	ProductID   string `json:"product_id"`
	ReceivedAt  string `json:"received_at"`
	QueueDepth  int    `json:"queue_depth"`
	BacklogSize int    `json:"backlog_size"`
	WorkerCount int    `json:"worker_count"`
}

func NewApp(cfg config.Config, st *store.Store, m *queue.Manager) *App {
	return &App{Cfg: cfg, Store: st, Manager: m, started: time.Now()}
}

func (a *App) StartShutdown() {
	a.closing = true
	a.Manager.CloseIntake()
}

func (a *App) postEventsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		WriteJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "")
		return
	}
	if a.closing || a.Manager.IsShuttingDown() {
		WriteJSONError(w, http.StatusServiceUnavailable, "shutting_down", "")
		return
	}
	ct := r.Header.Get("Content-Type")
	if !strings.HasPrefix(strings.ToLower(ct), "application/json") {
		WriteJSONError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "expected application/json")
		return
	}
	var ev model.Event
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&ev); err != nil {
		WriteJSONError(w, http.StatusBadRequest, "invalid_json", err.Error())
		return
	}
	if ev.ProductID == "" {
		WriteJSONError(w, http.StatusBadRequest, "validation_error", "product_id is required")
		return
	}
	if ev.Price != nil && *ev.Price < 0 {
		WriteJSONError(w, http.StatusBadRequest, "validation_error", "price must be >= 0")
		return
	}
	if ev.Stock != nil && *ev.Stock < 0 {
		WriteJSONError(w, http.StatusBadRequest, "validation_error", "stock must be >= 0")
		return
	}
	seq := a.Manager.NextSequence()
	ev.Sequence = seq
	ok := a.Manager.Enqueue(ev)
	if !ok {
		WriteJSONError(w, http.StatusServiceUnavailable, "shutting_down", "")
		return
	}
	enqTime := time.Now().UTC().Format(time.RFC3339)
	ac := ack{
		Status:      "accepted",
		RequestID:   RequestIDFromContext(r.Context()),
		Sequence:    seq,
		ProductID:   ev.ProductID,
		ReceivedAt:  enqTime,
		QueueDepth:  a.Manager.QueueDepth(),
		BacklogSize: a.Manager.BacklogSize(),
		WorkerCount: a.Manager.WorkerCount(),
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(ac)
	obs.Logger.Info("event_accepted",
		"request_id", ac.RequestID,
		"sequence", ac.Sequence,
		"product_id", ac.ProductID,
		"queue_depth", ac.QueueDepth,
		"backlog_size", ac.BacklogSize,
		"worker_count", ac.WorkerCount,
	)
}

func (a *App) getProductHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "")
		return
	}
	prefix := "/products/"
	if !strings.HasPrefix(r.URL.Path, prefix) {
		WriteJSONError(w, http.StatusNotFound, "not_found", "")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, prefix)
	if id == "" {
		WriteJSONError(w, http.StatusNotFound, "not_found", "")
		return
	}
	p, ok := a.Store.Get(id)
	if !ok {
		WriteJSONError(w, http.StatusNotFound, "not_found", "")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(p)
}

func (a *App) healthHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    _ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (a *App) metricsHandler(w http.ResponseWriter, r *http.Request) {
    enq, proc, backlog, depth := a.Manager.QueueMetrics()
    m := map[string]any{
        "events_enqueued": enq,
        "events_processed": proc,
        "backlog_size": backlog,
        "queue_depth": depth,
        "worker_count": a.Manager.WorkerCount(),
        "uptime_sec": time.Since(a.started).Seconds(),
    }
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(m)
}

func (a *App) openapiHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/yaml")
    _, _ = w.Write(httpopenapi.YAML)
}

func (a *App) docsHandler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/html; charset=utf-8")
    html := `<!doctype html>
<html>
  <head>
    <meta charset="utf-8" />
    <title>API Docs</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  </head>
  <body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script>
      window.ui = SwaggerUIBundle({
        url: '/openapi.yaml',
        dom_id: '#swagger-ui'
      });
    </script>
  </body>
</html>`
    _, _ = w.Write([]byte(html))
}
