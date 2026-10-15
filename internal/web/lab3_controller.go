package web

import (
	"net/http"

	"springlab/internal/app"
	"springlab/internal/config"
	"springlab/pkg/env"
	"springlab/pkg/ioc"
)

func init() {
	ioc.Component(NewLab3Controller)
}

// Lab3Controller — Listing 8: shows the current configuration.
type Lab3Controller struct {
	props       *config.AppProperties
	banner      config.EnvironmentBanner
	environment *env.Environment
}

func NewLab3Controller(props *config.AppProperties, banner config.EnvironmentBanner,
	environment *env.Environment) *Lab3Controller {
	return &Lab3Controller{props: props, banner: banner, environment: environment}
}

func (c *Lab3Controller) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/lab3/config", c.Config)
}

func (c *Lab3Controller) Config(w http.ResponseWriter, r *http.Request) {
	profiles := c.environment.ActiveProfiles()
	if profiles == nil {
		profiles = []string{}
	}
	app.WriteJSON(w, http.StatusOK, map[string]any{
		"owner":          c.props.Owner,
		"group":          c.props.Group,
		"mailFrom":       c.props.Mail.From,
		"mailRetryCount": c.props.Mail.RetryCount,
		"mailTimeout":    c.props.Mail.Timeout.String(),
		"mailEnabled":    c.props.Mail.Enabled,
		"serverPort":     c.environment.Get("server.port", ""),
		"activeProfiles": profiles,
		"banner":         c.banner.Describe(),
		// individual assignment, variant 1
		"uiTheme":        c.props.UI.Theme,
		"uiItemsPerPage": c.props.UI.ItemsPerPage,
	})
}
