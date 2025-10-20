package httpapi

import (
	"encoding/json"
	"net/http"
)

type jsonError struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

func WriteJSONError(w http.ResponseWriter, status int, message, details string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(jsonError{Error: message, Details: details})
}
