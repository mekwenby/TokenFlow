// Package httputil provides shared HTTP helpers used by admin and account handlers.
package httputil

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"tokenflow/internal/store"
)

// WriteError writes a JSON error response.
func WriteError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": message})
}

// IDParam extracts the "id" query parameter as int64.
func IDParam(r *http.Request) int64 {
	id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	return id
}

// DecodePayload reads JSON from the request body into dst. Writes a 400 error on failure.
func DecodePayload(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

// WriteResult encodes body as JSON, or writes an error response if err != nil.
// If err is store.ErrNotFound, it responds with 404 and notFoundMsg.
func WriteResult(w http.ResponseWriter, body any, err error, notFoundMsg string) {
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, store.ErrNotFound) {
			status = http.StatusNotFound
			if notFoundMsg != "" {
				WriteError(w, status, notFoundMsg)
				return
			}
		}
		WriteError(w, status, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(body)
}
