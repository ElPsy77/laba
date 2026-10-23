package web

import (
	"net/http"
	"strconv"

	"springlab/internal/app"
	"springlab/internal/catalog"
	"springlab/pkg/ioc"
)

func init() {
	ioc.Component(NewBookPageController)
}

// BookPageController — Listing 4: an ordinary @Controller @RequestMapping("/books").
// Its methods do not write the body: they fill in the Model and return a
// logical view name, which the ViewResolver turns into templates/books/list.html.
type BookPageController struct {
	service *catalog.BookService
}

func NewBookPageController(service *catalog.BookService) *BookPageController {
	return &BookPageController{service: service}
}

func (c *BookPageController) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /books", app.Page(c.list))
	mux.HandleFunc("POST /books", app.Page(c.create))
}

// list — GET /books?author=…
func (c *BookPageController) list(r *http.Request, model app.Model, _ *app.RedirectAttributes) (string, error) {
	author := r.URL.Query().Get("author") // @RequestParam(required = false)

	model.AddAttribute("books", c.service.FindAll(author))
	model.AddAttribute("author", author)
	model.AddAttribute("title", "Catalogue")

	return "books/list", nil // -> templates/books/list.html
}

// create — Listing 6, Post/Redirect/Get: POST /books with form fields
// title, author, year (@RequestParam each, so all three are required).
func (c *BookPageController) create(r *http.Request, _ app.Model, redirect *app.RedirectAttributes) (string, error) {
	if err := r.ParseForm(); err != nil {
		return "", app.BadRequest("%v", err)
	}
	for _, name := range []string{"title", "author", "year"} {
		if !r.Form.Has(name) {
			return "", app.BadRequest("Required request parameter '%s' is not present", name)
		}
	}
	year, err := strconv.Atoi(r.Form.Get("year"))
	if err != nil {
		return "", app.BadRequest("Failed to convert value '%s' to required type 'int'", r.Form.Get("year"))
	}

	saved := c.service.Create(catalog.Book{Title: r.Form.Get("title"), Author: r.Form.Get("author"), Year: year})
	redirect.AddFlashAttribute("message", "Book «"+saved.Title+"» has been added")

	return "redirect:/books", nil // 302; the browser performs a GET
}
