package config

import "springlab/pkg/ioc"

// Listing 7. Profile-specific beans: one interface, three implementations,
// each switched on by its own profile.
type EnvironmentBanner interface {
	Describe() string
}

func init() {
	ioc.Component(newDevBanner, ioc.Profile("dev"))
	ioc.Component(newProdBanner, ioc.Profile("prod"))
	ioc.Component(newDefaultBanner, ioc.Profile("!dev & !prod")) // when no profile is set
}

type devBanner struct{}

func newDevBanner() *devBanner      { return &devBanner{} }
func (*devBanner) Describe() string { return "DEVELOPMENT: the data is test data" }

type prodBanner struct{}

func newProdBanner() *prodBanner     { return &prodBanner{} }
func (*prodBanner) Describe() string { return "PRODUCTION: handle with care" }

type defaultBanner struct{}

func newDefaultBanner() *defaultBanner  { return &defaultBanner{} }
func (*defaultBanner) Describe() string { return "no profile is active" }
