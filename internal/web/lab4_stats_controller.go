package web

import (
	"net/http"

	"springlab/internal/app"
	"springlab/internal/aspect"
	"springlab/pkg/ioc"
)

func init() {
	ioc.Component(NewLab4StatsController)
}

// Lab4StatsController prints the call statistics of variant 1.
type Lab4StatsController struct{ counter *aspect.CallCounterAspect }

func NewLab4StatsController(counter *aspect.CallCounterAspect) *Lab4StatsController {
	return &Lab4StatsController{counter: counter}
}

func (c *Lab4StatsController) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/lab4/stats", func(w http.ResponseWriter, r *http.Request) {
		app.WriteJSON(w, http.StatusOK, c.counter.Stats())
	})
}
