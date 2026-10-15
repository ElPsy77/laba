package app

import (
	"encoding/json"
	"net/http"
	"time"
)

// NewRouter is the counterpart of DispatcherServlet: it maps paths to handlers.
// In the generated skeleton no handler is declared yet, so every path is 404.
func NewRouter(props Properties) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", notFound)
	return mux
}

// notFound mirrors the Spring Boot error body (the "Whitelabel" JSON).
func notFound(w http.ResponseWriter, r *http.Request) {
	WriteError(w, r, http.StatusNotFound, "Not Found")
}

// WriteError writes an error body in the Spring Boot format.
func WriteError(w http.ResponseWriter, r *http.Request, status int, reason string) {
	WriteJSON(w, status, map[string]any{
		"timestamp": time.Now().Format("2006-01-02T15:04:05.000"),
		"status":    status,
		"error":     reason,
		"path":      r.URL.Path,
	})
}

// WriteJSON is what Jackson does for @RestController: object -> JSON.
func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
