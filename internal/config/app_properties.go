package config

import (
	"time"

	"springlab/pkg/env"
	"springlab/pkg/ioc"
)

// Listing 5. Typed configuration. The Go struct plays the role of the
// record; tags replace the annotations:
//
//	@NotBlank          -> validate:"notblank"
//	@Min(1) @Max(10)   -> validate:"min=1;max=10"
//	@DefaultValue("3") -> default:"3"
//	@Valid (nested)    -> validate:"valid"
type AppProperties struct {
	Owner string `validate:"notblank"`
	Group string `validate:"notblank"`
	Mail  Mail   `validate:"valid"`
	UI    UI     `prop:"ui" validate:"valid"` // variant 1
}

type Mail struct {
	From       string        `validate:"notblank;email"`
	RetryCount int           `default:"3" validate:"min=1;max=10"`
	Timeout    time.Duration `default:"5s"`
	Enabled    bool          `default:"true"`
}

// UI — individual assignment, variant 1: prefix app.ui,
// theme (LIGHT/DARK), items-per-page @Min(5) @Max(50).
type UI struct {
	Theme        string `default:"LIGHT" validate:"notblank;oneof=LIGHT DARK"`
	ItemsPerPage int    `default:"10" validate:"min=5;max=50"`
}

// Prefix — @ConfigurationProperties(prefix = "app").
const Prefix = "app"

// Registering the binder is the counterpart of @ConfigurationPropertiesScan:
// the bean is named like in Spring: "<prefix>-<fully qualified type>".
func init() {
	ioc.Bean(BindAppProperties, ioc.Name("app-springlab/internal/config.AppProperties"))
}

// BindAppProperties binds and validates at start-up (@Validated): an
// invalid setting stops the application instead of failing under load.
func BindAppProperties(e *env.Environment) (*AppProperties, error) {
	p := &AppProperties{}
	if err := e.Bind(Prefix, p); err != nil {
		return nil, err
	}
	return p, nil
}
