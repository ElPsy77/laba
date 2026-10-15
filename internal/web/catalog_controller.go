package web

import (
	"net/http"
	"strconv"

	"springlab/internal/app"
	"springlab/internal/service"
	"springlab/pkg/ioc"
)

func init() {
	ioc.Component(NewCatalogController)
}

// CatalogController — the endpoints of laboratory work No. 4.
type CatalogController struct {
	catalogService service.CatalogService
}

func NewCatalogController(catalogService service.CatalogService) *CatalogController {
	return &CatalogController{catalogService: catalogService}
}

func (c *CatalogController) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/lab4/item/{id}", c.findByID)
	mux.HandleFunc("GET /api/lab4/items", c.findAll)
	mux.HandleFunc("DELETE /api/lab4/item/{id}", c.remove)
}

func (c *CatalogController) findByID(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	respond(w, r, func() (any, error) { return c.catalogService.FindByID(id) })
}

func (c *CatalogController) findAll(w http.ResponseWriter, r *http.Request) {
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil {
		limit = 5
	}
	respond(w, r, func() (any, error) { return c.catalogService.FindAll(limit) })
}

func (c *CatalogController) remove(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	respond(w, r, func() (any, error) { return c.catalogService.Remove(id) })
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		app.WriteError(w, r, http.StatusBadRequest, "Bad Request")
		return 0, false
	}
	return id, true
}

// respond writes the result, or — like an unhandled exception in Spring
// MVC — status 500 with the error message.
func respond(w http.ResponseWriter, r *http.Request, call func() (any, error)) {
	res, err := call()
	if err != nil {
		app.WriteJSON(w, http.StatusInternalServerError, map[string]any{
			"status": 500, "error": "Internal Server Error",
			"message": err.Error(), "path": r.URL.Path,
		})
		return
	}
	app.WriteJSON(w, http.StatusOK, map[string]any{"result": res})
}
