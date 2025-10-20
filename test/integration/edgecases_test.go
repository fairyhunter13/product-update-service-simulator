package integration

import (
	"bytes"
	"net/http"
	"testing"
)

func TestIntegration_ValidationErrors(t *testing.T) {
	waitReady(t)
	u := baseURL()

	cases := []struct {
		name, body, ctype string
		want              int
	}{
		{"missing_product_id", `{}`, "application/json", http.StatusBadRequest},
		{"negative_price", `{"product_id":"e1","price":-1}`, "application/json", http.StatusBadRequest},
		{"negative_stock", `{"product_id":"e2","stock":-1}`, "application/json", http.StatusBadRequest},
		{"malformed_json", `{"product_id":"e3",`, "application/json", http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, _ := http.NewRequest(http.MethodPost, u+"/events", bytes.NewBufferString(tc.body))
			r.Header.Set("Content-Type", tc.ctype)
			resp, err := http.DefaultClient.Do(r)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tc.want {
				t.Fatalf("%s: expected %d, got %d", tc.name, tc.want, resp.StatusCode)
			}
		})
	}
}
