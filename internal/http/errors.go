// Package httpapi exposes the HTTP API layer of the service.
package httpapi

import (
	"encoding/json"
	"net/http"
)

// jsonError represents a JSON error payload.
type jsonError struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// WriteJSONError writes a JSON error payload with the given status code.
func WriteJSONError(w http.ResponseWriter, status int, message, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(jsonError{Error: message, Details: details})
}
