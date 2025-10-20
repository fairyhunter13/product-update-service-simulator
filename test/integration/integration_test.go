package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func baseURL() string {
	if v := os.Getenv("BASE_URL"); v != "" {
		return v
	}
    return "http://localhost:8080"
}

func TestIntegration_OpenAPIServed(t *testing.T) {
    waitReady(t)
    u := baseURL()
    resp, err := http.Get(u+"/openapi.yaml")
    if err != nil { t.Fatal(err) }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK { t.Fatalf("expected 200, got %d", resp.StatusCode) }
}

func TestIntegration_DocsServed(t *testing.T) {
    waitReady(t)
    u := baseURL()
    resp, err := http.Get(u+"/docs")
    if err != nil { t.Fatal(err) }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK { t.Fatalf("expected 200, got %d", resp.StatusCode) }
    // best-effort: read up to a small buffer to search for swagger-ui token
    buf := make([]byte, 1024)
    n, _ := resp.Body.Read(buf)
    if !strings.Contains(string(buf[:n]), "swagger-ui") {
        t.Fatalf("expected swagger-ui in docs page")
    }
}

func waitReady(t *testing.T) {
	t.Helper()
	url := fmt.Sprintf("%s/products/__ping__", baseURL())
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("service not ready")
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

type product struct {
	ProductID string  `json:"product_id"`
	Price     float64 `json:"price"`
	Stock     int64   `json:"stock"`
}

func TestIntegration_PostThenGet(t *testing.T) {
	waitReady(t)
	u := baseURL()
	for i := 0; i < 10; i++ {
		body := []byte(`{"product_id":"pi","price":1}`)
		r, err := http.NewRequest(http.MethodPost, u+"/events", bytes.NewBuffer(body))
		if err != nil { t.Fatal(err) }
		r.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(r)
		if err != nil { t.Fatal(err) }
		if resp.StatusCode != http.StatusAccepted { t.Fatalf("expected 202, got %d", resp.StatusCode) }
		_ = resp.Body.Close()
	}
	time.Sleep(2 * time.Second)
	rg, err := http.NewRequest(http.MethodGet, u+"/products/pi", nil)
	if err != nil { t.Fatal(err) }
	respg, err := http.DefaultClient.Do(rg)
	if err != nil { t.Fatal(err) }
	if respg.StatusCode != http.StatusOK { t.Fatalf("expected 200, got %d", respg.StatusCode) }
	defer respg.Body.Close()
	var p product
	if err := json.NewDecoder(respg.Body).Decode(&p); err != nil { t.Fatal(err) }
	if p.ProductID != "pi" { t.Fatalf("unexpected product: %+v", p) }
}

func TestIntegration_StrictDecoding_UnknownField(t *testing.T) {
	waitReady(t)
	u := baseURL()
	body := []byte(`{"product_id":"pi2","price":1,"unknown":"x"}`)
	r, err := http.NewRequest(http.MethodPost, u+"/events", bytes.NewBuffer(body))
	if err != nil { t.Fatal(err) }
	r.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(r)
	if err != nil { t.Fatal(err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest { t.Fatalf("expected 400, got %d", resp.StatusCode) }
}

func TestIntegration_PartialUpdates(t *testing.T) {
	waitReady(t)
	u := baseURL()
	// price only
	body1 := []byte(`{"product_id":"p-up","price":9.9}`)
	r1, err := http.NewRequest(http.MethodPost, u+"/events", bytes.NewBuffer(body1))
	if err != nil { t.Fatal(err) }
	r1.Header.Set("Content-Type", "application/json")
	resp1, err := http.DefaultClient.Do(r1)
	if err != nil { t.Fatal(err) }
	resp1.Body.Close()
	if resp1.StatusCode != http.StatusAccepted { t.Fatalf("expected 202, got %d", resp1.StatusCode) }
	// stock only
	body2 := []byte(`{"product_id":"p-up","stock":7}`)
	r2, err := http.NewRequest(http.MethodPost, u+"/events", bytes.NewBuffer(body2))
	if err != nil { t.Fatal(err) }
	r2.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(r2)
	if err != nil { t.Fatal(err) }
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusAccepted { t.Fatalf("expected 202, got %d", resp2.StatusCode) }
	// wait and verify
	time.Sleep(2 * time.Second)
	rg, _ := http.NewRequest(http.MethodGet, u+"/products/p-up", nil)
	respg, err := http.DefaultClient.Do(rg)
	if err != nil { t.Fatal(err) }
	defer respg.Body.Close()
	if respg.StatusCode != http.StatusOK { t.Fatalf("expected 200, got %d", respg.StatusCode) }
	var p product
	if err := json.NewDecoder(respg.Body).Decode(&p); err != nil { t.Fatal(err) }
	if p.ProductID != "p-up" || p.Price != 9.9 || p.Stock != 7 { t.Fatalf("unexpected product: %+v", p) }
}

func TestIntegration_UnsupportedMediaType(t *testing.T) {
	waitReady(t)
	u := baseURL()
	r, _ := http.NewRequest(http.MethodPost, u+"/events", bytes.NewBufferString("{}"))
	r.Header.Set("Content-Type", "text/plain")
	resp, err := http.DefaultClient.Do(r)
	if err != nil { t.Fatal(err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType { t.Fatalf("expected 415, got %d", resp.StatusCode) }
}
