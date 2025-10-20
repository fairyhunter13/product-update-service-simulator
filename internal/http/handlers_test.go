package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fairyhunter13/product-update-service-simulator/internal/config"
	"github.com/fairyhunter13/product-update-service-simulator/internal/model"
	"github.com/fairyhunter13/product-update-service-simulator/internal/obs"
	"github.com/fairyhunter13/product-update-service-simulator/internal/queue"
	"github.com/fairyhunter13/product-update-service-simulator/internal/store"
)

type ackResp struct {
	Status      string `json:"status"`
	RequestID   string `json:"request_id"`
	Sequence    uint64 `json:"sequence"`
	ProductID   string `json:"product_id"`
	ReceivedAt  string `json:"received_at"`
	QueueDepth  int    `json:"queue_depth"`
	BacklogSize int    `json:"backlog_size"`
	WorkerCount int    `json:"worker_count"`
}

func TestOpenAPIServed(t *testing.T) {
    _, _, cleanup, mux := setupApp(t)
    defer cleanup()
    req := httptest.NewRequest(http.MethodGet, "/openapi.yaml", nil)
    rr := httptest.NewRecorder()
    mux.ServeHTTP(rr, req)
    if rr.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d", rr.Code)
    }
    if ct := rr.Header().Get("Content-Type"); ct == "" {
        t.Fatalf("expected content-type set")
    }
    if !bytes.Contains(rr.Body.Bytes(), []byte("openapi:")) {
        t.Fatalf("expected openapi content")
    }
}

func TestDocsServed(t *testing.T) {
    _, _, cleanup, mux := setupApp(t)
    defer cleanup()
    req := httptest.NewRequest(http.MethodGet, "/docs", nil)
    rr := httptest.NewRecorder()
    mux.ServeHTTP(rr, req)
    if rr.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d", rr.Code)
    }
    body := rr.Body.String()
    if !strings.Contains(body, "swagger-ui") {
        t.Fatalf("expected swagger-ui in docs body")
    }
}

func TestHealthzOK(t *testing.T) {
    _, _, cleanup, mux := setupApp(t)
    defer cleanup()
    req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
    rr := httptest.NewRecorder()
    mux.ServeHTTP(rr, req)
    if rr.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d", rr.Code)
    }
}

func TestMetricsHandler(t *testing.T) {
    _, mgr, cleanup, mux := setupApp(t)
    defer cleanup()
    // enqueue some items to alter metrics
    for i := 0; i < 5; i++ {
        b := bytes.NewBufferString(`{"product_id":"m","price":1}`)
        r := httptest.NewRequest(http.MethodPost, "/events", b)
        r.Header.Set("Content-Type", "application/json")
        w := httptest.NewRecorder()
        mux.ServeHTTP(w, r)
        if w.Code != http.StatusAccepted { t.Fatalf("expected 202, got %d", w.Code) }
    }
    req := httptest.NewRequest(http.MethodGet, "/debug/metrics", nil)
    rr := httptest.NewRecorder()
    mux.ServeHTTP(rr, req)
    if rr.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d", rr.Code)
    }
    var m map[string]any
    if err := json.Unmarshal(rr.Body.Bytes(), &m); err != nil {
        t.Fatalf("metrics json decode: %v", err)
    }
    if _, ok := m["worker_count"]; !ok { t.Fatalf("missing worker_count") }
    if _, ok := m["queue_depth"]; !ok { t.Fatalf("missing queue_depth") }
    // drain to avoid goroutines accumulating
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()
    if ok := mgr.DrainUntil(ctx); !ok { t.Fatalf("drain timeout") }
}

func setupApp(t *testing.T) (*App, *queue.Manager, context.CancelFunc, http.Handler) {
	t.Helper()
	cfg := config.Load()
	obs.InitLogger()
	st := store.New()
	q := queue.New(128)
	mgr := queue.NewManager(cfg, q, st)
	ctx, cancel := context.WithCancel(context.Background())
	mgr.Start(ctx)
	app := NewApp(cfg, st, mgr)
	mux := NewRouter(app)
	return app, mgr, func() { cancel(); mgr.Stop() }, mux
}

func TestPostEvents_HappyPath(t *testing.T) {
	_, mgr, cleanup, mux := setupApp(t)
	defer cleanup()
	body := `{"product_id":"p-1","price":10.5}`
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-Id", "test-req-1")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rr.Code)
	}
	var ac ackResp
	if err := json.Unmarshal(rr.Body.Bytes(), &ac); err != nil {
		t.Fatalf("decode ack: %v", err)
	}
	if ac.RequestID != "test-req-1" || ac.ProductID != "p-1" || ac.Status != "accepted" {
		t.Fatalf("unexpected ack: %+v", ac)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if ok := mgr.DrainUntil(ctx); !ok {
		t.Fatalf("drain timeout")
	}
	req2 := httptest.NewRequest(http.MethodGet, "/products/p-1", nil)
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr2.Code)
	}
	var p model.Product
	if err := json.Unmarshal(rr2.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode product: %v", err)
	}
	if p.ProductID != "p-1" || p.Price != 10.5 {
		t.Fatalf("unexpected product: %+v", p)
	}
}

func TestPostEvents_UnknownFields(t *testing.T) {
	_, _, cleanup, mux := setupApp(t)
	defer cleanup()
	body := `{"product_id":"p-2","price":1.0,"foo":"bar"}`
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rr.Code)
	}
}

func TestPostEvents_UnsupportedMediaType(t *testing.T) {
	_, _, cleanup, mux := setupApp(t)
	defer cleanup()
	req := httptest.NewRequest(http.MethodPost, "/events", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "text/plain")
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d", rr.Code)
	}
}

func TestGetProduct_NotFound(t *testing.T) {
	_, _, cleanup, mux := setupApp(t)
	defer cleanup()
	req := httptest.NewRequest(http.MethodGet, "/products/unknown", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}
}

func TestPartialUpdates(t *testing.T) {
	_, mgr, cleanup, mux := setupApp(t)
	defer cleanup()
	b1 := `{"product_id":"p-3","price":9.9}`
	r1 := httptest.NewRequest(http.MethodPost, "/events", bytes.NewBufferString(b1))
	r1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	mux.ServeHTTP(w1, r1)
	if w1.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", w1.Code)
	}
	b2 := `{"product_id":"p-3","stock":7}`
	r2 := httptest.NewRequest(http.MethodPost, "/events", bytes.NewBufferString(b2))
	r2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, r2)
	if w2.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", w2.Code)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if ok := mgr.DrainUntil(ctx); !ok {
		t.Fatalf("drain timeout")
	}
	gr := httptest.NewRequest(http.MethodGet, "/products/p-3", nil)
	gw := httptest.NewRecorder()
	mux.ServeHTTP(gw, gr)
	if gw.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", gw.Code)
	}
	var p model.Product
	if err := json.Unmarshal(gw.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode product: %v", err)
	}
	if p.ProductID != "p-3" || p.Price != 9.9 || p.Stock != 7 {
		t.Fatalf("unexpected product: %+v", p)
	}
}

func TestShutdownBehavior(t *testing.T) {
	app, _, cleanup, mux := setupApp(t)
	defer cleanup()
	app.StartShutdown()
	r := httptest.NewRequest(http.MethodPost, "/events", bytes.NewBufferString(`{"product_id":"p-4"}`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}
