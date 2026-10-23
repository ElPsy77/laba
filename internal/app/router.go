package app

import (
	"encoding/json"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// ResourcesDir is the "classpath": static/ and templates/ live under it.
// Tests that run from a package directory point it at ../../resources.
var ResourcesDir = "resources"

// NewRouter is the counterpart of DispatcherServlet. Controllers register
// their own routes (HandlerMapping); what nothing maps reaches the fallback,
// which does what Spring Boot does after the controllers:
//
//  1. ResourceHttpRequestHandler — a GET for a file under static/ serves the
//     file, and "/" serves static/index.html (the welcome page);
//  2. the path is mapped, but for other HTTP methods — 405 Method Not Allowed;
//  3. nothing matches at all — 404 Not Found.
func NewRouter() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if (r.Method == http.MethodGet || r.Method == http.MethodHead) && serveStatic(w, r) {
			return
		}
		if allowed := allowedMethods(mux, r); len(allowed) > 0 {
			w.Header().Set("Allow", strings.Join(allowed, ", "))
			WriteError(w, r, http.StatusMethodNotAllowed, "Method Not Allowed")
			return
		}
		WriteError(w, r, http.StatusNotFound, "Not Found")
	})
	return mux
}

// serveStatic serves resources/static/** — no controller maps these URLs.
func serveStatic(w http.ResponseWriter, r *http.Request) bool {
	rel := path.Clean("/" + r.URL.Path) // "/../x" cannot leave static/
	if strings.HasSuffix(rel, "/") {
		rel += "index.html"
	}
	file := filepath.Join(ResourcesDir, "static", filepath.FromSlash(rel))
	f, err := os.Open(file)
	if err != nil {
		return false
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		return false
	}
	http.ServeContent(w, r, info.Name(), info.ModTime(), f)
	return true
}

// allowedMethods asks the mux which other methods are mapped for this path.
func allowedMethods(mux *http.ServeMux, r *http.Request) []string {
	var allowed []string
	for _, m := range []string{http.MethodGet, http.MethodPost, http.MethodPut,
		http.MethodPatch, http.MethodDelete} {
		if m == r.Method {
			continue
		}
		probe := r.Clone(r.Context())
		probe.Method = m
		if _, pattern := mux.Handler(probe); pattern != "" && pattern != "/" {
			allowed = append(allowed, m)
		}
	}
	return allowed
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
