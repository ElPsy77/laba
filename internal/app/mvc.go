package app

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"html/template"
	"log/slog"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
)

// ---------------------------------------------------------------------------
// @Controller: Model, view names, ViewResolver, redirect:, flash attributes
// ---------------------------------------------------------------------------

// Model is the Model argument of a @Controller method: attributes for the view.
type Model map[string]any

func (m Model) AddAttribute(name string, value any) { m[name] = value }

// RedirectAttributes — addFlashAttribute survives exactly one redirect.
type RedirectAttributes struct{ flash map[string]any }

func (ra *RedirectAttributes) AddFlashAttribute(name string, value any) {
	if ra.flash == nil {
		ra.flash = map[string]any{}
	}
	ra.flash[name] = value
}

// PageHandler is a @Controller method: it fills in the model and returns a
// logical view name ("books/list") or "redirect:/path".
type PageHandler func(r *http.Request, model Model, redirect *RedirectAttributes) (string, error)

// Page is the HandlerAdapter for a @Controller method: it prepares the
// arguments, calls the method and then lets the ViewResolver turn the
// returned name into HTML — or answers 302 for "redirect:".
func Page(h PageHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		model := Model{}
		for k, v := range flashes.take(w, r) { // flash attributes of the previous request
			model[k] = v
		}
		redirect := &RedirectAttributes{}
		view, err := h(r, model, redirect)
		if err != nil {
			writeHandlerError(w, r, err)
			return
		}
		if target, ok := strings.CutPrefix(view, "redirect:"); ok {
			flashes.save(w, r, target, redirect.flash)
			http.Redirect(w, r, target, http.StatusFound) // 302, the browser does a GET
			return
		}
		render(w, r, view, model)
	}
}

// render is the ThymeleafViewResolver: prefix templates/, suffix .html.
// Templates are parsed on every request — like spring.thymeleaf.cache=false
// under DevTools, an edited template is picked up without a restart.
func render(w http.ResponseWriter, r *http.Request, view string, model Model) {
	file := filepath.Join(ResourcesDir, "templates", filepath.FromSlash(view)+".html")
	tmpl, err := template.ParseFiles(file)
	if err != nil { // TemplateInputException
		slog.Error("Error resolving template", "view", view, "error", err)
		WriteError(w, r, http.StatusInternalServerError, "Internal Server Error")
		return
	}
	w.Header().Set("Content-Type", "text/html;charset=UTF-8")
	if err := tmpl.Execute(w, model); err != nil {
		slog.Error("Error rendering template", "view", view, "error", err)
	}
}

// ---------------------------------------------------------------------------
// FlashMapManager: flash attributes are kept in the HTTP session between the
// POST and the GET that follows the redirect, and removed once used.
// ---------------------------------------------------------------------------

const sessionCookie = "JSESSIONID"

type flashMap struct {
	target string
	attrs  map[string]any
}

type flashStore struct {
	mu       sync.Mutex
	sessions map[string][]flashMap // session id -> pending flash maps
}

var flashes = &flashStore{sessions: map[string][]flashMap{}}

func (s *flashStore) save(w http.ResponseWriter, r *http.Request, target string, attrs map[string]any) {
	if len(attrs) == 0 {
		return
	}
	id := sessionID(w, r)
	u, _ := url.Parse(target)
	s.mu.Lock()
	s.sessions[id] = append(s.sessions[id], flashMap{target: u.Path, attrs: attrs})
	s.mu.Unlock()
}

// take returns the flash attributes meant for this path and deletes them.
func (s *flashStore) take(w http.ResponseWriter, r *http.Request) map[string]any {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	pending := s.sessions[c.Value]
	for i, fm := range pending {
		if fm.target == r.URL.Path {
			s.sessions[c.Value] = append(pending[:i], pending[i+1:]...)
			return fm.attrs
		}
	}
	return nil
}

func sessionID(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie(sessionCookie); err == nil {
		return c.Value
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	id := strings.ToUpper(hex.EncodeToString(b))
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: id, Path: "/", HttpOnly: true})
	return id
}

// ---------------------------------------------------------------------------
// What HandlerAdapter and the message converters reject before the
// controller method is called.
// ---------------------------------------------------------------------------

// StatusError is an error with an HTTP status — what Spring raises as
// MissingServletRequestParameterException (400), TypeMismatch (400),
// HttpMediaTypeNotSupportedException (415), HttpMediaTypeNotAcceptableException (406).
type StatusError struct {
	Status  int
	Message string
}

func (e *StatusError) Error() string { return e.Message }

func BadRequest(format string, a ...any) error {
	return &StatusError{http.StatusBadRequest, fmt.Sprintf(format, a...)}
}

// writeHandlerError writes a *StatusError with its own status, anything else as 500.
func writeHandlerError(w http.ResponseWriter, r *http.Request, err error) {
	var se *StatusError
	if errors.As(err, &se) {
		WriteJSON(w, se.Status, map[string]any{
			"status": se.Status, "error": http.StatusText(se.Status),
			"message": se.Message, "path": r.URL.Path,
		})
		return
	}
	WriteJSON(w, http.StatusInternalServerError, map[string]any{
		"status": 500, "error": "Internal Server Error", "message": err.Error(), "path": r.URL.Path,
	})
}

// WriteHandlerError is exported for the REST controllers.
func WriteHandlerError(w http.ResponseWriter, r *http.Request, err error) {
	writeHandlerError(w, r, err)
}

// CheckJSONBody is the HttpMessageConverter lookup for @RequestBody: only a
// body declared as application/json can be read into an object — otherwise 415.
func CheckJSONBody(r *http.Request) error {
	ct := r.Header.Get("Content-Type")
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil || !(mt == "application/json" || strings.HasSuffix(mt, "+json")) {
		if ct == "" {
			ct = "application/octet-stream" // what Spring assumes when the header is missing
		}
		return &StatusError{http.StatusUnsupportedMediaType, "Content-Type '" + ct + "' is not supported"}
	}
	return nil
}

// CheckAcceptsJSON is content negotiation for the response: the only
// converter for a @RestController answer is JSON, so a client that accepts
// only XML cannot be served — 406. No Accept header means "anything".
func CheckAcceptsJSON(r *http.Request) error {
	accept := r.Header.Get("Accept")
	if accept == "" {
		return nil
	}
	for _, part := range strings.Split(accept, ",") {
		mt, params, err := mime.ParseMediaType(strings.TrimSpace(part))
		if err != nil || params["q"] == "0" {
			continue
		}
		if mt == "*/*" || mt == "application/*" || mt == "application/json" {
			return nil
		}
	}
	return &StatusError{http.StatusNotAcceptable, "No acceptable representation"}
}
