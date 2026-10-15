package web

import (
	"net/http"

	"springlab/internal/app"
	"springlab/internal/lifecycle"
	"springlab/internal/notify"
	"springlab/internal/scope"
	"springlab/pkg/ioc"
)

// Listing 5. The laboratory controller. Parameter 3 is
// @Qualifier("upper") — the individual assignment.
func init() {
	ioc.Component(NewLab2Controller, ioc.Qualifier(3, "upper"))
}

type Lab2Controller struct {
	notifications *notify.NotificationService
	lifecycle     *lifecycle.LifecycleDemo
	office        *scope.TicketOffice
	custom        notify.Notifier
}

func NewLab2Controller(n *notify.NotificationService, l *lifecycle.LifecycleDemo,
	o *scope.TicketOffice, custom notify.Notifier) *Lab2Controller {
	return &Lab2Controller{notifications: n, lifecycle: l, office: o, custom: custom}
}

func (c *Lab2Controller) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/lab2/notify", c.Notify)
	mux.HandleFunc("GET /api/lab2/lifecycle", c.Lifecycle)
	mux.HandleFunc("GET /api/lab2/scopes", c.Scopes)
	mux.HandleFunc("GET /api/lab2/custom", c.Custom)
}

func textParam(r *http.Request, def string) string {
	if t := r.URL.Query().Get("text"); t != "" {
		return t
	}
	return def
}

func (c *Lab2Controller) Notify(w http.ResponseWriter, r *http.Request) {
	text := textParam(r, "Hello")
	app.WriteJSON(w, http.StatusOK, map[string]any{
		"primary":   c.notifications.ViaPrimary(text),
		"console":   c.notifications.ViaConsole(text),
		"all":       c.notifications.ViaAll(text),
		"beanNames": c.notifications.Names(),
	})
}

func (c *Lab2Controller) Lifecycle(w http.ResponseWriter, r *http.Request) {
	app.WriteJSON(w, http.StatusOK, c.lifecycle.Events())
}

func (c *Lab2Controller) Scopes(w http.ResponseWriter, r *http.Request) {
	app.WriteJSON(w, http.StatusOK, c.office.Demo())
}

// Custom — /api/lab2/custom?text=... calls exactly the variant implementation.
func (c *Lab2Controller) Custom(w http.ResponseWriter, r *http.Request) {
	app.WriteJSON(w, http.StatusOK, map[string]string{
		"channel": c.custom.Channel(),
		"result":  c.custom.Send(textParam(r, "Hello")),
	})
}
