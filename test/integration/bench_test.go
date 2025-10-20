package integration

import (
	"bytes"
	"net/http"
	"testing"
)

// Benchmark for POST /events; to run: go test -bench=. ./test/integration -run ^$
func BenchmarkPostEvents(b *testing.B) {
	u := baseURL()
	client := &http.Client{}
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			body := []byte(`{"product_id":"b","price":1}`)
			r, _ := http.NewRequest(http.MethodPost, u+"/events", bytes.NewBuffer(body))
			r.Header.Set("Content-Type", "application/json")
			resp, err := client.Do(r)
			if err == nil {
				_ = resp.Body.Close()
			}
		}
	})
}
