package aop

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"unicode"
)

// Pointcut is a compiled AspectJ-style expression. Supported designators:
//
//	execution(public * *(..))           execution(* *..*Service.Update*(..))
//	within(springlab.internal.service..*)
//	@annotation(springlab.internal.audit.Audited)   by type
//	@annotation(audited)                             bound to an advice parameter
//	bean(*Service)                                    by bean name (Spring extension)
//	&&  ||  !  ( )  and references to named pointcuts: Pointcuts.serviceOperation()
type Pointcut struct {
	Expr  string
	match func(m *Method, bean string) bool
	// binding: the annotation type bound by @annotation(name), if any.
	binding reflect.Type
}

// Matches tells whether the pointcut selects the method of the given bean.
func (p *Pointcut) Matches(m *Method, bean string) bool { return p.match(m, bean) }

// ---- named pointcuts ---------------------------------------------------------

var named = map[string]*Pointcut{} // full name -> pointcut

// PointcutDef is one @Pointcut method of a pointcut library.
type PointcutDef struct{ Name, Expr string }

// P declares a named pointcut: @Pointcut("expr") public void name() { }
func P(name, expr string) PointcutDef { return PointcutDef{name, expr} }

// Library registers a library of named pointcuts under a class-like name,
// for example "springlab.internal.aspect.Pointcuts". Inside the library
// pointcuts refer to each other by short name: serviceLayer(); from
// aspects they are referred to by full name:
// springlab.internal.aspect.Pointcuts.serviceOperation().
func Library(class string, defs ...PointcutDef) string {
	for _, d := range defs {
		pc, err := compile(d.Expr, nil, class)
		if err != nil {
			panic(fmt.Sprintf("IllegalArgumentException: %s.%s(): %v", class, d.Name, err))
		}
		named[class+"."+d.Name] = pc
	}
	return class
}

// ---- compilation -------------------------------------------------------------

// compile parses expr. binding is the annotation type the advice receives
// (nil if none); scope is the library used to resolve short references.
func compile(expr string, binding reflect.Type, scope string) (*Pointcut, error) {
	p := &parser{src: expr, binding: binding, scope: scope}
	m, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	p.skip()
	if p.pos < len(p.src) {
		return nil, p.fail("unexpected '%s'", p.src[p.pos:])
	}
	if binding != nil && !p.bound {
		return nil, fmt.Errorf("formal unbound in pointcut: the advice expects %s "+
			"but the expression has no @annotation(name)", binding)
	}
	return &Pointcut{Expr: expr, match: m, binding: binding}, nil
}

type matcher func(m *Method, bean string) bool

type parser struct {
	src     string
	pos     int
	binding reflect.Type
	bound   bool
	scope   string
}

func (p *parser) fail(format string, a ...any) error {
	return fmt.Errorf("error at ::%d %s in pointcut \"%s\"", p.pos, fmt.Sprintf(format, a...), p.src)
}

func (p *parser) skip() {
	for p.pos < len(p.src) && p.src[p.pos] == ' ' {
		p.pos++
	}
}

func (p *parser) eat(tok string) bool {
	p.skip()
	if strings.HasPrefix(p.src[p.pos:], tok) {
		p.pos += len(tok)
		return true
	}
	return false
}

func (p *parser) parseOr() (matcher, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for p.eat("||") {
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		l := left
		left = func(m *Method, b string) bool { return l(m, b) || right(m, b) }
	}
	return left, nil
}

func (p *parser) parseAnd() (matcher, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for p.eat("&&") {
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		l := left
		left = func(m *Method, b string) bool { return l(m, b) && right(m, b) }
	}
	return left, nil
}

func (p *parser) parseUnary() (matcher, error) {
	if p.eat("!") {
		inner, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return func(m *Method, b string) bool { return !inner(m, b) }, nil
	}
	if p.eat("(") {
		inner, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if !p.eat(")") {
			return nil, p.fail("missing ')'")
		}
		return inner, nil
	}
	return p.parsePrimary()
}

// parsePrimary reads designator(args) or a named reference name().
func (p *parser) parsePrimary() (matcher, error) {
	p.skip()
	start := p.pos
	for p.pos < len(p.src) && (isIdent(rune(p.src[p.pos])) || p.src[p.pos] == '.' || p.src[p.pos] == '@') {
		p.pos++
	}
	word := p.src[start:p.pos]
	if word == "" || !p.eat("(") {
		return nil, p.fail("expected a designator or a pointcut reference")
	}
	depth, argStart := 1, p.pos
	for p.pos < len(p.src) && depth > 0 {
		switch p.src[p.pos] {
		case '(':
			depth++
		case ')':
			depth--
		}
		p.pos++
	}
	if depth != 0 {
		return nil, p.fail("missing ')'")
	}
	arg := strings.TrimSpace(p.src[argStart : p.pos-1])

	switch word {
	case "execution":
		return execution(arg)
	case "within":
		re := typePattern(arg)
		return func(m *Method, _ string) bool { return re.MatchString(m.QualifiedType()) }, nil
	case "bean":
		re := glob(arg)
		return func(_ *Method, b string) bool { return re.MatchString(b) }, nil
	case "@annotation":
		return p.annotation(arg)
	case "call", "get", "set", "cflow", "if", "initialization", "staticinitialization":
		return nil, p.fail("unsupported pointcut primitive '%s': Spring AOP intercepts method executions only", word)
	}
	// a reference to a named pointcut
	if arg != "" {
		return nil, p.fail("unknown designator '%s'", word)
	}
	ref, ok := named[word]
	if !ok && p.scope != "" {
		ref, ok = named[p.scope+"."+word]
	}
	if !ok {
		return nil, p.fail("can't find referenced pointcut %s", word)
	}
	return ref.match, nil
}

func (p *parser) annotation(arg string) (matcher, error) {
	if arg != "" && unicode.IsLower(rune(arg[0])) && !strings.Contains(arg, ".") {
		// @annotation(audited): bind by the type of the advice parameter
		if p.binding == nil {
			return nil, p.fail("formal unbound in pointcut: '%s' is not a parameter of the advice", arg)
		}
		p.bound = true
		t := p.binding
		return func(m *Method, _ string) bool { _, ok := m.annotationOfType(t); return ok }, nil
	}
	return func(m *Method, _ string) bool {
		for _, a := range m.Annotations {
			t := reflect.TypeOf(a)
			full := strings.ReplaceAll(t.PkgPath(), "/", ".") + "." + t.Name()
			if full == arg || t.Name() == arg {
				return true
			}
		}
		return false
	}, nil
}

var execRe = regexp.MustCompile(`^(public\s+|private\s+)?(\S+)\s+(\S+?)\((.*)\)$`)

// execution(modifiers? return-type declaring-type? name(parameters))
func execution(arg string) (matcher, error) {
	parts := execRe.FindStringSubmatch(strings.TrimSpace(arg))
	if parts == nil {
		return nil, fmt.Errorf("malformed execution(%s)", arg)
	}
	modifier := strings.TrimSpace(parts[1])
	full := parts[3]
	declaring, name := "", full
	if i := strings.LastIndex(full, "."); i >= 0 {
		declaring, name = full[:i], full[i+1:]
		declaring = strings.TrimSuffix(declaring, ".") // "*..*Service." -> keep ".." semantics
	}
	nameRe := glob(name)
	var typeRe *regexp.Regexp
	if declaring != "" {
		typeRe = typePattern(declaring)
	}
	return func(m *Method, _ string) bool {
		if modifier == "public" && !m.Exported || modifier == "private" && m.Exported {
			return false
		}
		if typeRe != nil && !typeRe.MatchString(m.QualifiedType()) && !typeRe.MatchString(m.Type) {
			return false
		}
		return nameRe.MatchString(m.Name)
	}, nil
}

// typePattern converts an AspectJ type pattern: * is one segment,
// .. is any number of packages.
func typePattern(p string) *regexp.Regexp {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(p); i++ {
		switch {
		case strings.HasPrefix(p[i:], ".."):
			b.WriteString(`(\.[^.]+)*\.`)
			i++
		case p[i] == '*':
			b.WriteString(`[^.]*`)
		default:
			b.WriteString(regexp.QuoteMeta(string(p[i])))
		}
	}
	b.WriteString("$")
	return regexp.MustCompile(b.String())
}

func glob(p string) *regexp.Regexp {
	return regexp.MustCompile("^" + strings.ReplaceAll(regexp.QuoteMeta(p), `\*`, ".*") + "$")
}

func isIdent(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }
