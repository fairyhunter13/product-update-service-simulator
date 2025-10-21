package integration

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type ack2 struct {
	Status      string `json:"status"`
	RequestID   string `json:"request_id"`
	Sequence    uint64 `json:"sequence"`
	ProductID   string `json:"product_id"`
	ReceivedAt  string `json:"received_at"`
	QueueDepth  int    `json:"queue_depth"`
	BacklogSize int    `json:"backlog_size"`
	WorkerCount int    `json:"worker_count"`
}

func TestIntegration_GetUnknownProductNotFound(t *testing.T) {
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
}

func TestIntegration_MethodNotAllowed(t *testing.T) {
	waitReady(t)
	u := baseURL()
	// GET /events -> 405
	r1, _ := http.NewRequest(http.MethodGet, u+"/events", nil)
	resp1, err := http.DefaultClient.Do(r1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp1.Body.Close() }()
	if resp1.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp1.StatusCode)
	}
	// POST /products/{id} -> 405
	r2, _ := http.NewRequest(http.MethodPost, u+"/products/x", bytes.NewBufferString("{}"))
	r2.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(r2)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp2.Body.Close() }()
	if resp2.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", resp2.StatusCode)
	}
}

func TestIntegration_ContentTypeVariants(t *testing.T) {
	waitReady(t)
	u := baseURL()
	variants := []string{
		"application/json",
		"application/json; charset=utf-8",
		"APPLICATION/JSON",
	}
	for _, ctype := range variants {
		r, _ := http.NewRequest(http.MethodPost, u+"/events", bytes.NewBufferString(`{"product_id":"ctv","price":0}`))
		r.Header.Set("Content-Type", ctype)
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusAccepted {
			t.Fatalf("ctype %q expected 202, got %d", ctype, resp.StatusCode)
		}
	}
}

func TestIntegration_NoContentTypeIs415(t *testing.T) {
	waitReady(t)
	u := baseURL()
	r, _ := http.NewRequest(http.MethodPost, u+"/events", bytes.NewBufferString(`{"product_id":"noct","price":1}`))
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("expected 415, got %d", resp.StatusCode)
	}
}

func TestIntegration_AckIncludesRequestIDAndTimestamp(t *testing.T) {
	waitReady(t)
	u := baseURL()
	r, _ := http.NewRequest(http.MethodPost, u+"/events", bytes.NewBufferString(`{"product_id":"ack","price":1}`))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Request-Id", "itest-req-1")
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
	if a.RequestID != "itest-req-1" {
		t.Fatalf("request_id mismatch: %q", a.RequestID)
	}
	if _, err := time.Parse(time.RFC3339, a.ReceivedAt); err != nil {
		t.Fatalf("received_at not RFC3339: %v", err)
	}
}

func TestIntegration_MetricsReflectActivity(t *testing.T) {
	waitReady(t)
	u := baseURL()
	// submit a few events
	for i := 0; i < 5; i++ {
		r, _ := http.NewRequest(http.MethodPost, u+"/events", bytes.NewBufferString(`{"product_id":"m1","stock":0}`))
		r.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = resp.Body.Close() }()
	}
	time.Sleep(500 * time.Millisecond)
	resp, err := http.Get(u + "/debug/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(b), "events_enqueued") || !strings.Contains(string(b), "events_processed") {
		t.Fatalf("metrics missing expected keys: %s", string(b))
	}
}

func TestIntegration_OpenAPIAndVarsEndpoints(t *testing.T) {
	waitReady(t)
	u := baseURL()
	// openapi.yaml
	resp1, err := http.Get(u + "/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp1.Body.Close() }()
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp1.StatusCode)
	}
	// debug vars
	resp2, err := http.Get(u + "/debug/vars")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp2.Body.Close() }()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
}

func TestIntegration_BoundaryValues(t *testing.T) {
	waitReady(t)
	u := baseURL()
	// price 0
	r1, _ := http.NewRequest(http.MethodPost, u+"/events", bytes.NewBufferString(`{"product_id":"bv","price":0}`))
	r1.Header.Set("Content-Type", "application/json")
	resp1, err := http.DefaultClient.Do(r1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp1.Body.Close() }()
	if resp1.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp1.StatusCode)
	}
	// stock 0
	r2, _ := http.NewRequest(http.MethodPost, u+"/events", bytes.NewBufferString(`{"product_id":"bv","stock":0}`))
	r2.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(r2)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp2.Body.Close() }()
	if resp2.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp2.StatusCode)
	}
}

func TestIntegration_LastWriteWins(t *testing.T) {
	waitReady(t)
	u := baseURL()
	// First price 1
	r1, _ := http.NewRequest(http.MethodPost, u+"/events", bytes.NewBufferString(`{"product_id":"lww","price":1}`))
	r1.Header.Set("Content-Type", "application/json")
	resp1, err := http.DefaultClient.Do(r1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp1.Body.Close() }()
	if resp1.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp1.StatusCode)
	}
	// Then price 2
	r2, _ := http.NewRequest(http.MethodPost, u+"/events", bytes.NewBufferString(`{"product_id":"lww","price":2}`))
	r2.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(r2)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp2.Body.Close() }()
	if resp2.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp2.StatusCode)
	}
	// Verify final state
	time.Sleep(750 * time.Millisecond)
	resp3, err := http.Get(u + "/products/lww")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp3.Body.Close() }()
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp3.StatusCode)
	}
	var p struct {
		Price     float64 `json:"price"`
		ProductID string  `json:"product_id"`
	}
	if err := json.NewDecoder(resp3.Body).Decode(&p); err != nil {
		t.Fatal(err)
	}
	if p.ProductID != "lww" || p.Price != 2 {
		t.Fatalf("unexpected product: %+v", p)
	}
}
