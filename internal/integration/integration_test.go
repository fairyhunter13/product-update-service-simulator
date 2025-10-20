package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fairyhunter13/product-update-service-simulator/internal/config"
	httpapi "github.com/fairyhunter13/product-update-service-simulator/internal/http"
	"github.com/fairyhunter13/product-update-service-simulator/internal/model"
	"github.com/fairyhunter13/product-update-service-simulator/internal/obs"
	"github.com/fairyhunter13/product-update-service-simulator/internal/queue"
	"github.com/fairyhunter13/product-update-service-simulator/internal/store"
)

func TestIntegration_PostThenGet(t *testing.T) {
	cfg := config.Load()
	obs.InitLogger()
	st := store.New()
	q := queue.New(128)
	mgr := queue.NewManager(cfg, q, st)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mgr.Start(ctx)
	app := httpapi.NewApp(cfg, st, mgr)
	h := httpapi.NewRouter(app)
	for i := 0; i < 10; i++ {
		b := bytes.NewBufferString(`{"product_id":"pi","price":1}`)
		r := httptest.NewRequest(http.MethodPost, "/events", b)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusAccepted {
			t.Fatalf("expected 202, got %d", w.Code)
		}
	}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel2()
	if ok := mgr.DrainUntil(ctx2); !ok {
		t.Fatalf("drain timeout")
	}
	rg := httptest.NewRequest(http.MethodGet, "/products/pi", nil)
	wg := httptest.NewRecorder()
	h.ServeHTTP(wg, rg)
	if wg.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", wg.Code)
	}
	var p model.Product
	if err := json.Unmarshal(wg.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p.ProductID != "pi" {
		t.Fatalf("unexpected product: %+v", p)
	}
}
