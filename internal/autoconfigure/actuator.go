package autoconfigure

import (
	"net/http"
	"regexp"
	"strings"

	"springlab/internal/app"
	"springlab/internal/config"
	"springlab/pkg/env"
	"springlab/pkg/ioc"
)

// ActuatorController — the diagnostic endpoints of Spring Boot Actuator:
// /actuator/health, /actuator/env[/{key}], /actuator/configprops,
// /actuator/conditions. Only the endpoints listed in
// management.endpoints.web.exposure.include are registered.
type ActuatorController struct {
	env   *env.Environment
	ctx   *ioc.Container
	props *config.AppProperties
}

func NewActuatorController(e *env.Environment, ctx *ioc.Container,
	props *config.AppProperties) *ActuatorController {
	return &ActuatorController{env: e, ctx: ctx, props: props}
}

func (a *ActuatorController) Register(mux *http.ServeMux) {
	exposed := map[string]bool{}
	for _, n := range strings.Split(a.env.Get("management.endpoints.web.exposure.include", ""), ",") {
		exposed[strings.TrimSpace(n)] = true
	}
	all := exposed["*"]
	if all || exposed["health"] {
		mux.HandleFunc("GET /actuator/health", a.health)
	}
	if all || exposed["env"] {
		mux.HandleFunc("GET /actuator/env", a.envAll)
		mux.HandleFunc("GET /actuator/env/{key}", a.envKey)
	}
	if all || exposed["configprops"] {
		mux.HandleFunc("GET /actuator/configprops", a.configprops)
	}
	if all || exposed["conditions"] {
		mux.HandleFunc("GET /actuator/conditions", a.conditions)
	}
}

func (a *ActuatorController) health(w http.ResponseWriter, _ *http.Request) {
	app.WriteJSON(w, http.StatusOK, map[string]string{"status": "UP"})
}

var secret = regexp.MustCompile(`(?i)password|secret|token|key$|credentials`)

func sanitize(key, value string) string {
	if secret.MatchString(key) {
		return "******"
	}
	return value
}

func (a *ActuatorController) profiles() []string {
	p := a.env.ActiveProfiles()
	if p == nil {
		p = []string{}
	}
	return p
}

func (a *ActuatorController) envAll(w http.ResponseWriter, _ *http.Request) {
	type prop struct {
		Value  string `json:"value"`
		Origin string `json:"origin"`
	}
	type source struct {
		Name       string          `json:"name"`
		Properties map[string]prop `json:"properties"`
	}
	var sources []source
	for _, s := range a.env.Sources() {
		ps := map[string]prop{}
		for _, e := range s.Entries() {
			ps[e.Key] = prop{sanitize(e.Key, e.Value), e.Origin}
		}
		sources = append(sources, source{s.Name, ps})
	}
	app.WriteJSON(w, http.StatusOK, map[string]any{
		"activeProfiles":  a.profiles(),
		"propertySources": sources,
	})
}

// envKey — /actuator/env/{key}: not only the value but the source of the
// key, and every source that has it (the first one wins).
func (a *ActuatorController) envKey(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	entry, winner, ok := a.env.LookupEntry(key)
	if !ok {
		app.WriteError(w, r, http.StatusNotFound, "Not Found")
		return
	}
	type found struct {
		Value  string `json:"value"`
		Origin string `json:"origin"`
	}
	type src struct {
		Name     string `json:"name"`
		Property *found `json:"property,omitempty"`
	}
	var list []src
	for _, s := range a.env.Sources() {
		if e, ok := s.Get(key); ok {
			list = append(list, src{s.Name, &found{sanitize(key, e.Value), e.Origin}})
		} else {
			list = append(list, src{Name: s.Name})
		}
	}
	app.WriteJSON(w, http.StatusOK, map[string]any{
		"property": map[string]string{
			"source": winner.Name,
			"value":  sanitize(key, entry.Value),
		},
		"activeProfiles":  a.profiles(),
		"defaultProfiles": []string{"default"},
		"propertySources": list,
	})
}

func (a *ActuatorController) configprops(w http.ResponseWriter, _ *http.Request) {
	p := a.props
	app.WriteJSON(w, http.StatusOK, map[string]any{
		"contexts": map[string]any{"application": map[string]any{"beans": map[string]any{
			"app-springlab/internal/config.AppProperties": map[string]any{
				"prefix": config.Prefix,
				"properties": map[string]any{
					"owner": p.Owner,
					"group": p.Group,
					"mail": map[string]any{
						"from": p.Mail.From, "retryCount": p.Mail.RetryCount,
						"timeout": p.Mail.Timeout.String(), "enabled": p.Mail.Enabled,
					},
					"ui": map[string]any{"theme": p.UI.Theme, "itemsPerPage": p.UI.ItemsPerPage},
				},
			},
		}}},
	})
}

func (a *ActuatorController) conditions(w http.ResponseWriter, _ *http.Request) {
	app.WriteJSON(w, http.StatusOK, a.ctx.ConditionsReport())
}
