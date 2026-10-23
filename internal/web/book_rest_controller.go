package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"springlab/internal/app"
	"springlab/internal/catalog"
	"springlab/pkg/ioc"
)

func init() {
	ioc.Component(NewBookRestController)
}

// BookRestController — Listing 2: @RestController @RequestMapping("/api/books").
// Every method answers with an object written to the body as JSON
// (@ResponseBody); the status comes from ResponseEntity.
type BookRestController struct {
	service *catalog.BookService
}

func NewBookRestController(service *catalog.BookService) *BookRestController {
	return &BookRestController{service: service}
}

func (c *BookRestController) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/books", c.list)
	mux.HandleFunc("GET /api/books/{id}", c.find)
	mux.HandleFunc("POST /api/books", c.create)
	mux.HandleFunc("PUT /api/books/{id}", c.replace)
	mux.HandleFunc("DELETE /api/books/{id}", c.delete)
	mux.HandleFunc("GET /api/books/search", c.search) // individual assignment, variant 1
}

// list — GET /api/books?author=…&limit=…
//
//	@RequestParam(required = false) String author
//	@RequestParam(defaultValue = "10") int limit
func (c *BookRestController) list(w http.ResponseWriter, r *http.Request) {
	if !accepts(w, r) {
		return
	}
	author := r.URL.Query().Get("author") // absent -> "" (null)
	limit, err := intParam(r, "limit", 10)
	if err != nil {
		app.WriteHandlerError(w, r, err)
		return
	}
	books := c.service.FindAll(author)
	books = books[:min(limit, len(books))] // stream().limit(limit)
	app.WriteJSON(w, http.StatusOK, books)
}

// find — GET /api/books/{id}: 200 or 404, built with ResponseEntity, not an exception.
func (c *BookRestController) find(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r) // "abc" is not a long -> 400 before the method body
	if !ok || !accepts(w, r) {
		return
	}
	book, found := c.service.FindByID(id)
	if !found {
		w.WriteHeader(http.StatusNotFound) // ResponseEntity.notFound().build()
		return
	}
	app.WriteJSON(w, http.StatusOK, book) // ResponseEntity.ok(book)
}

// create — POST /api/books with @RequestBody: 201 Created + Location.
func (c *BookRestController) create(w http.ResponseWriter, r *http.Request) {
	if !accepts(w, r) {
		return
	}
	book, ok := readBook(w, r)
	if !ok {
		return
	}
	saved := c.service.Create(book)
	w.Header().Set("Location", fmt.Sprintf("/api/books/%d", saved.ID))
	app.WriteJSON(w, http.StatusCreated, saved)
}

// replace — PUT /api/books/{id} (TODO 2.2): 200 or 404.
func (c *BookRestController) replace(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok || !accepts(w, r) {
		return
	}
	book, ok := readBook(w, r)
	if !ok {
		return
	}
	updated, found := c.service.Replace(id, book)
	if !found {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	app.WriteJSON(w, http.StatusOK, updated)
}

// delete — DELETE /api/books/{id} (TODO 2.3): 204 No Content or 404.
func (c *BookRestController) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if !c.service.Delete(id) {
		w.WriteHeader(http.StatusNotFound) // already deleted: nothing to remove
		return
	}
	w.WriteHeader(http.StatusNoContent) // ResponseEntity.noContent().build()
}

// search — variant 1: GET /api/books/search?title=…
// A case-insensitive search by a fragment of the title. Nothing found is
// still a successful search: 200 with an empty array, not 404.
// @RequestParam String title — required, so a missing title is 400.
func (c *BookRestController) search(w http.ResponseWriter, r *http.Request) {
	if !accepts(w, r) {
		return
	}
	if !r.URL.Query().Has("title") {
		app.WriteHandlerError(w, r, app.BadRequest(
			"Required request parameter 'title' for method parameter type String is not present"))
		return
	}
	app.WriteJSON(w, http.StatusOK, c.service.SearchByTitle(r.URL.Query().Get("title")))
}

// readBook is @RequestBody Book: the Content-Type decides which converter
// reads the body (415 if none fits); a malformed body is 400.
func readBook(w http.ResponseWriter, r *http.Request) (catalog.Book, bool) {
	var book catalog.Book
	if err := app.CheckJSONBody(r); err != nil {
		app.WriteHandlerError(w, r, err)
		return book, false
	}
	if err := json.NewDecoder(r.Body).Decode(&book); err != nil {
		app.WriteHandlerError(w, r, app.BadRequest("JSON parse error: %v", err))
		return book, false
	}
	return book, true
}

// accepts checks the Accept header: JSON is the only representation — else 406.
func accepts(w http.ResponseWriter, r *http.Request) bool {
	if err := app.CheckAcceptsJSON(r); err != nil {
		app.WriteHandlerError(w, r, err)
		return false
	}
	return true
}

// intParam is @RequestParam(defaultValue = "…") int name: absent -> default,
// not a number -> 400 (MethodArgumentTypeMismatchException).
func intParam(r *http.Request, name string, def int) (int, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return def, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, app.BadRequest("Method parameter '%s': Failed to convert value of type "+
			"'java.lang.String' to required type 'int'; For input string: \"%s\"", name, raw)
	}
	if n < 0 {
		return 0, app.BadRequest("Method parameter '%s': must not be negative", name)
	}
	return n, nil
}
