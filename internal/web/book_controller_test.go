package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"springlab/internal/app"
	"springlab/internal/catalog"
)

func init() { app.ResourcesDir = "../../resources" } // tests run from internal/web

// newBooksMux assembles the web layer of lab 5 on a fresh repository.
func newBooksMux() *http.ServeMux {
	service := catalog.NewBookService(catalog.NewBookRepository())
	mux := app.NewRouter()
	NewBookRestController(service).Register(mux)
	NewBookPageController(service).Register(mux)
	return mux
}

func do(mux *http.ServeMux, method, url, body string, headers ...string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func books(t *testing.T, rec *httptest.ResponseRecorder) []catalog.Book {
	t.Helper()
	var out []catalog.Book
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("not a JSON array: %q", rec.Body.String())
	}
	return out
}

// Listing 3 — every command of task 2.4 with its expected status.
func TestRestOperations(t *testing.T) {
	mux := newBooksMux()

	rec := do(mux, "GET", "/api/books", "")
	if rec.Code != 200 || len(books(t, rec)) != 3 || rec.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}
	rec = do(mux, "GET", "/api/books?author=Bloch&limit=1", "")
	if got := books(t, rec); rec.Code != 200 || len(got) != 1 || got[0].Author != "Joshua Bloch" {
		t.Fatalf("filter: %d %s", rec.Code, rec.Body)
	}
	if rec = do(mux, "GET", "/api/books?limit=2", ""); len(books(t, rec)) != 2 {
		t.Fatalf("limit: %s", rec.Body)
	}
	if rec = do(mux, "GET", "/api/books/1", ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), "Effective Java") {
		t.Fatalf("find 1: %d %s", rec.Code, rec.Body)
	}
	if rec = do(mux, "GET", "/api/books/999", ""); rec.Code != 404 {
		t.Fatalf("find 999: %d", rec.Code)
	}

	rec = do(mux, "POST", "/api/books", `{"title":"Pro Spring 6","author":"Cosmina","year":2023}`,
		"Content-Type", "application/json")
	if rec.Code != 201 || rec.Header().Get("Location") != "/api/books/4" {
		t.Fatalf("create: %d %v %s", rec.Code, rec.Header(), rec.Body)
	}
	if rec = do(mux, "GET", "/api/books/4", ""); rec.Code != 200 {
		t.Fatalf("created book is not readable at Location: %d", rec.Code)
	}

	rec = do(mux, "PUT", "/api/books/4", `{"title":"Pro Spring 6","author":"Iuliana Cosmina","year":2023}`,
		"Content-Type", "application/json")
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), "Iuliana Cosmina") {
		t.Fatalf("put: %d %s", rec.Code, rec.Body)
	}
	if rec = do(mux, "PUT", "/api/books/999", `{"title":"x","author":"y","year":1}`,
		"Content-Type", "application/json"); rec.Code != 404 {
		t.Fatalf("put 999: %d", rec.Code)
	}

	if rec = do(mux, "DELETE", "/api/books/2", ""); rec.Code != 204 || rec.Body.Len() != 0 {
		t.Fatalf("first delete: %d", rec.Code)
	}
	if rec = do(mux, "DELETE", "/api/books/2", ""); rec.Code != 404 {
		t.Fatalf("second delete: %d", rec.Code)
	}
}

// Task 4.3 — the five error responses.
func TestStatusCodeTable(t *testing.T) {
	mux := newBooksMux()
	cases := []struct {
		name, method, url, body string
		headers                 []string
		want                    int
	}{
		{"path variable is not a number", "GET", "/api/books/abc", "", nil, 400},
		{"limit is not a number", "GET", "/api/books?limit=ten", "", nil, 400},
		{"nothing maps the path", "GET", "/api/nothing", "", nil, 404},
		{"path mapped, method not", "DELETE", "/api/books", "", nil, 405},
		{"body is not JSON", "POST", "/api/books", "Pro Spring 6", []string{"Content-Type", "text/plain"}, 415},
		{"client wants XML only", "GET", "/api/books", "", []string{"Accept", "application/xml"}, 406},
	}
	for _, c := range cases {
		rec := do(mux, c.method, c.url, c.body, c.headers...)
		if rec.Code != c.want {
			t.Errorf("%s: %s %s -> %d, want %d (%s)", c.name, c.method, c.url, rec.Code, c.want, rec.Body)
		}
	}
	if allow := do(mux, "DELETE", "/api/books", "").Header().Get("Allow"); allow != "GET, POST" {
		t.Errorf("405 Allow header = %q", allow)
	}
}

func TestPageRendersModel(t *testing.T) {
	mux := newBooksMux()
	rec := do(mux, "GET", "/books", "")
	body := rec.Body.String()
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/html") ||
		!strings.Contains(body, "<title>Catalogue</title>") || !strings.Contains(body, "Clean Code") {
		t.Fatalf("page: %d %s", rec.Code, body)
	}
	body = do(mux, "GET", "/books?author=Bloch", "").Body.String()
	if !strings.Contains(body, "Effective Java") || strings.Contains(body, "Clean Code") {
		t.Fatalf("filtered page shows the wrong books")
	}
}

func TestStaticResources(t *testing.T) {
	mux := newBooksMux()
	rec := do(mux, "GET", "/css/app.css", "")
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/css") {
		t.Fatalf("css: %d %v", rec.Code, rec.Header())
	}
	if rec = do(mux, "GET", "/", ""); rec.Code != 200 || !strings.Contains(rec.Body.String(), `href="/books"`) {
		t.Fatalf("welcome page: %d", rec.Code)
	}
	if rec = do(mux, "GET", "/css/../../go.mod", ""); rec.Code == 200 || strings.Contains(rec.Body.String(), "module") {
		t.Fatalf("path traversal must not leave static/: %d", rec.Code)
	}
}

// Task 4 — Post/Redirect/Get: 302, the flash message is shown once, F5 adds nothing.
func TestPostRedirectGet(t *testing.T) {
	mux := newBooksMux()
	rec := do(mux, "POST", "/books", "title=Java Precisely&author=Sestoft&year=2016",
		"Content-Type", "application/x-www-form-urlencoded")
	if rec.Code != 302 || rec.Header().Get("Location") != "/books" {
		t.Fatalf("post: %d %v", rec.Code, rec.Header())
	}
	cookie := rec.Result().Cookies()[0]
	session := cookie.Name + "=" + cookie.Value

	page := do(mux, "GET", "/books", "", "Cookie", session).Body.String()
	if !strings.Contains(page, `class="flash"`) || !strings.Contains(page, "Java Precisely") {
		t.Fatalf("after redirect: flash or new book missing")
	}
	page = do(mux, "GET", "/books", "", "Cookie", session).Body.String() // F5
	if strings.Contains(page, `class="flash"`) {
		t.Fatalf("flash message must disappear after one use")
	}
	if strings.Count(page, "Java Precisely") != 1 {
		t.Fatalf("refresh must not create a duplicate")
	}

	if rec = do(mux, "POST", "/books", "title=X&author=Y&year=abc",
		"Content-Type", "application/x-www-form-urlencoded"); rec.Code != 400 {
		t.Fatalf("bad year: %d", rec.Code)
	}
	if rec = do(mux, "POST", "/books", "title=X", "Content-Type", "application/x-www-form-urlencoded"); rec.Code != 400 {
		t.Fatalf("missing params: %d", rec.Code)
	}
}

// Variant 1 — search by a fragment of the title on three data sets.
func TestSearchVariant1(t *testing.T) {
	mux := newBooksMux()
	cases := map[string][]string{
		"/api/books/search?title=java":   {"Effective Java"},
		"/api/books/search?title=CLEAN":  {"Clean Code"},
		"/api/books/search?title=python": {},
		"/api/books/search?title=in":     {"Spring in Action"},
	}
	for url, want := range cases {
		rec := do(mux, "GET", url, "")
		got := books(t, rec)
		if rec.Code != 200 || len(got) != len(want) {
			t.Errorf("%s: %d %s", url, rec.Code, rec.Body)
			continue
		}
		for i := range want {
			if got[i].Title != want[i] {
				t.Errorf("%s: got %s, want %s", url, got[i].Title, want[i])
			}
		}
	}
	rec := do(mux, "GET", "/api/books/search?title=python", "")
	if b, _ := io.ReadAll(rec.Body); strings.TrimSpace(string(b)) != "[]" {
		t.Errorf("empty result must be [], got %s", b)
	}
	if rec := do(mux, "GET", "/api/books/search", ""); rec.Code != 400 {
		t.Errorf("missing title: %d", rec.Code)
	}
}
