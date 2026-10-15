// Package audit holds the custom @Audited annotation.
package audit

// Audited — Listing 5. Go has no annotations, so an annotation is a plain
// struct type, and it is attached to a method by a comment above the
// method:
//
//	// @Audited(action = "CATALOG_LIST", logArguments = true)
//	func (s *catalogService) FindAll(limit int) ([]string, error)
//
// The comment alone exists only in the source (RetentionPolicy.SOURCE).
// `go generate` turns it into a value stored with the method metadata —
// that is what @Retention(RetentionPolicy.RUNTIME) achieves in Java: the
// annotation instance is available at run time and reaches the advice.
type Audited struct {
	Action       string // String action();
	LogArguments bool   // boolean logArguments() default false;
}
