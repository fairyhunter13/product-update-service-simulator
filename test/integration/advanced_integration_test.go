package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"
	"time"
)

func TestIntegration_MetricsIncreaseAndSane(t *testing.T) {
	waitReady(t)
	u := baseURL()

	// snapshot metrics
	before := map[string]any{}
	resp0, err := http.Get(u + "/debug/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp0.Body.Close() }()
	if resp0.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp0.StatusCode)
	}
	if err := json.NewDecoder(resp0.Body).Decode(&before); err != nil {
		t.Fatal(err)
	}

	// drive activity
	const n = 10
	for i := 0; i < n; i++ {
		r, _ := http.NewRequest(http.MethodPost, u+"/events", bytes.NewBufferString(`{"product_id":"mx","stock":0}`))
		r.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusAccepted {
			t.Fatalf("expected 202, got %d", resp.StatusCode)
		}
	}
	time.Sleep(600 * time.Millisecond)

	after := map[string]any{}
	resp1, err := http.Get(u + "/debug/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp1.Body.Close() }()
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp1.StatusCode)
	}
	if err := json.NewDecoder(resp1.Body).Decode(&after); err != nil {
		t.Fatal(err)
	}

	bProc := toFloat(before["events_processed"])
	aProc := toFloat(after["events_processed"])
	if aProc < bProc {
		t.Fatalf("events_processed did not increase: before=%v after=%v", bProc, aProc)
	}
	uptime := toFloat(after["uptime_sec"])
	if uptime < 0 {
		t.Fatalf("uptime_sec negative: %v", uptime)
	}
	w := toFloat(after["worker_count"])
	if w <= 0 {
		t.Fatalf("worker_count should be > 0, got %v", w)
	}
}

func TestIntegration_GetUnknownProduct_NotFoundJSON(t *testing.T) {
	waitReady(t)
	u := baseURL()
	resp, err := http.Get(u + "/products/does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct == "" || ct[:16] != "application/json" {
		t.Fatalf("unexpected content-type: %q", ct)
	}
	var m map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatal(err)
	}
	if m["error"] != "not_found" {
		t.Fatalf("expected error=not_found, got: %+v", m)
	}
}

func TestIntegration_GetEmptyID_NotFoundJSON(t *testing.T) {
	waitReady(t)
	u := baseURL()
	resp, err := http.Get(u + "/products/")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct == "" || ct[:16] != "application/json" {
		t.Fatalf("unexpected content-type: %q", ct)
	}
	var m map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatal(err)
	}
	if m["error"] != "not_found" {
		t.Fatalf("expected error=not_found, got: %+v", m)
	}
}

func TestIntegration_MethodNotAllowedOnProductsID(t *testing.T) {
	waitReady(t)
	u := baseURL()
	r, _ := http.NewRequest(http.MethodPost, u+"/products/mm", nil)
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct == "" || ct[:16] != "application/json" {
		t.Fatalf("unexpected content-type: %q", ct)
	}
	var m map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		t.Fatal(err)
	}
	if m["error"] != "method_not_allowed" {
		t.Fatalf("expected error=method_not_allowed, got: %+v", m)
	}
}

func TestIntegration_GetExistingProduct_JSONShape(t *testing.T) {
	waitReady(t)
	u := baseURL()
	// seed full payload
	r, _ := http.NewRequest(http.MethodPost, u+"/events", bytes.NewBufferString(`{"product_id":"shape","price":10.5,"stock":7}`))
	r.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	time.Sleep(600 * time.Millisecond)

	// GET and validate JSON shape and values
	respG, err := http.Get(u + "/products/shape")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = respG.Body.Close() }()
	if respG.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", respG.StatusCode)
	}
	if ct := respG.Header.Get("Content-Type"); ct == "" || ct[:16] != "application/json" {
		t.Fatalf("unexpected content-type: %q", ct)
	}
	var m map[string]any
	if err := json.NewDecoder(respG.Body).Decode(&m); err != nil {
		t.Fatal(err)
	}
	// keys exist and have expected types/values
	if _, ok := m["product_id"]; !ok {
		t.Fatalf("missing product_id key: %+v", m)
	}
	if m["product_id"] != "shape" {
		t.Fatalf("unexpected product_id: %v", m["product_id"])
	}
	if _, ok := m["price"]; !ok {
		t.Fatalf("missing price key: %+v", m)
	}
	if _, ok := m["stock"]; !ok {
		t.Fatalf("missing stock key: %+v", m)
	}
	if p := toFloat(m["price"]); p != 10.5 {
		t.Fatalf("unexpected price: %v", p)
	}
	if s := toFloat(m["stock"]); s != 7 {
		t.Fatalf("unexpected stock: %v", s)
	}
}

func TestIntegration_ResponseContentTypeHeaders(t *testing.T) {
	waitReady(t)
	u := baseURL()
	// seed product
	r, _ := http.NewRequest(http.MethodPost, u+"/events", bytes.NewBufferString(`{"product_id":"ct","price":1}`))
	r.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	// allow processing
	time.Sleep(500 * time.Millisecond)
	// GET product content-type
	resp1, err := http.Get(u + "/products/ct")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp1.Body.Close() }()
	if ct := resp1.Header.Get("Content-Type"); ct == "" || ct[:16] != "application/json" {
		t.Fatalf("unexpected content-type: %q", ct)
	}
	// healthz content-type
	resp2, err := http.Get(u + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp2.Body.Close() }()
	if ct := resp2.Header.Get("Content-Type"); ct == "" || ct[:16] != "application/json" {
		t.Fatalf("unexpected content-type: %q", ct)
	}
}

func TestIntegration_GeneratedRequestIDWhenMissing(t *testing.T) {
	waitReady(t)
	u := baseURL()
	r, _ := http.NewRequest(http.MethodPost, u+"/events", bytes.NewBufferString(`{"product_id":"gen","stock":1}`))
	r.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
	var a ack2
	if err := json.NewDecoder(resp.Body).Decode(&a); err != nil {
		t.Fatal(err)
	}
	if a.RequestID == "" {
		t.Fatalf("expected generated request_id when header missing")
	}
}

// helper: safely cast number-like interface{} to float64
func toFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case float32:
		return float64(x)
	case int:
		return float64(x)
	case int64:
		return float64(x)
	case json.Number:
		f, _ := x.Float64()
		return f
	default:
		return 0
	}
}
