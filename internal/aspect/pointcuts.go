// Package aspect holds the aspects of the application.
package aspect

import "springlab/pkg/aop"

// Pointcuts — Listing 2. A library of named pointcuts: every expression is
// written once, here, and referred to by name from the aspects. In Java it
// is a class with @Pointcut methods; in Go it is a named library.
//
// Package paths are written with dots: springlab/internal/service ->
// springlab.internal.service. In Go "public" means exported (capitalised).
var Pointcuts = aop.Library("springlab.internal.aspect.Pointcuts",
	aop.P("serviceLayer", "within(springlab.internal.service..*)"),
	aop.P("publicMethod", "execution(public * *(..))"),
	aop.P("serviceOperation", "serviceLayer() && publicMethod()"),
)

// ServiceOperation is the full name aspects refer to.
const ServiceOperation = "springlab.internal.aspect.Pointcuts.serviceOperation()"
