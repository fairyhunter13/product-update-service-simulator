package integration

import (
	"bytes"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"
)

// Sends many POST /events concurrently and asserts 202 responses (no 503 backpressure)
func TestIntegration_HighLoadNonBlocking(t *testing.T) {
	waitReady(t)
	u := baseURL()
	concurrency := 50
	perGoroutine := 20
	client := &http.Client{Timeout: 5 * time.Second}

	var wg sync.WaitGroup
	wg.Add(concurrency)
	errCh := make(chan error, concurrency*perGoroutine)
	for g := 0; g < concurrency; g++ {
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < perGoroutine; i++ {
				body := []byte(fmt.Sprintf(`{"product_id":"pl-%d-%d","price":1}`, gid, i))
				r, _ := http.NewRequest(http.MethodPost, u+"/events", bytes.NewBuffer(body))
				r.Header.Set("Content-Type", "application/json")
				resp, err := client.Do(r)
				if err != nil {
					errCh <- err
					return
				}
				if resp.StatusCode != http.StatusAccepted {
					errCh <- fmt.Errorf("expected 202, got %d", resp.StatusCode)
				}
				_ = resp.Body.Close()
			}
		}(g)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
}
