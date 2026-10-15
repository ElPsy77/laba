// Command aopgen generates AOP proxies — the build-time counterpart of the
// CGLIB subclass Spring creates at run time.
//
// Usage (in a //go:generate line of the package that owns the type):
//
//	//go:generate go run springlab/cmd/aopgen -type catalogService -iface CatalogService -annotation Audited=springlab/internal/audit
//
// It reads the methods of -iface, the annotations written in the doc
// comments of the -type methods (// @Audited(action = "CATALOG_LIST")),
// and writes <type>_aop.go with a proxy that implements -iface and routes
// every call through aop.Handler. Annotations become run-time values only
// through this step — like @Retention(RUNTIME); a comment alone is SOURCE.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

type annotationFlags map[string]string // name -> import path

func (a annotationFlags) String() string { return fmt.Sprint(map[string]string(a)) }
func (a annotationFlags) Set(v string) error {
	name, path, ok := strings.Cut(v, "=")
	if !ok {
		return fmt.Errorf("want Name=import/path, got %q", v)
	}
	a[name] = path
	return nil
}

func main() {
	typeName := flag.String("type", "", "the implementation type (receiver of the methods)")
	ifaceName := flag.String("iface", "", "the interface the proxy implements")
	annotations := annotationFlags{}
	flag.Var(annotations, "annotation", "Name=import/path of an annotation type (repeatable)")
	flag.Parse()
	if *typeName == "" || *ifaceName == "" {
		log.Fatal("aopgen: -type and -iface are required")
	}
	dir, _ := os.Getwd()
	src, err := generate(dir, *typeName, *ifaceName, annotations)
	if err != nil {
		log.Fatal("aopgen: ", err)
	}
	out := filepath.Join(dir, strings.ToLower(*typeName)+"_aop.go")
	if err := os.WriteFile(out, src, 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Println("aopgen: wrote", filepath.Base(out))
}

type param struct{ typ string }

type method struct {
	name        string
	params      []param
	results     []string
	annotations []string // Go composite literals
}

func generate(dir, typeName, ifaceName string, ann annotationFlags) ([]byte, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go") && !strings.HasSuffix(fi.Name(), "_aop.go")
	}, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	var pkg *ast.Package
	for _, p := range pkgs {
		pkg = p
	}
	if pkg == nil {
		return nil, fmt.Errorf("no Go package in %s", dir)
	}
	importPath, err := importPathOf(dir)
	if err != nil {
		return nil, err
	}

	var iface *ast.InterfaceType
	docs := map[string]*ast.CommentGroup{}
	for _, f := range pkg.Files {
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				for _, s := range d.Specs {
					if ts, ok := s.(*ast.TypeSpec); ok && ts.Name.Name == ifaceName {
						iface, _ = ts.Type.(*ast.InterfaceType)
					}
				}
			case *ast.FuncDecl:
				if d.Recv != nil && receiverName(d.Recv) == typeName {
					docs[d.Name.Name] = d.Doc
				}
			}
		}
	}
	if iface == nil {
		return nil, fmt.Errorf("interface %s not found", ifaceName)
	}

	expr := func(e ast.Expr) string {
		var b bytes.Buffer
		_ = printer.Fprint(&b, fset, e)
		return b.String()
	}
	usedImports := map[string]string{}
	var methods []method
	for _, field := range iface.Methods.List {
		ft, ok := field.Type.(*ast.FuncType)
		if !ok {
			return nil, fmt.Errorf("embedded interfaces are not supported")
		}
		for _, n := range field.Names {
			m := method{name: n.Name}
			for _, p := range ft.Params.List {
				if _, variadic := p.Type.(*ast.Ellipsis); variadic {
					return nil, fmt.Errorf("%s: variadic parameters are not supported", n.Name)
				}
				count := len(p.Names)
				if count == 0 {
					count = 1
				}
				for i := 0; i < count; i++ {
					m.params = append(m.params, param{expr(p.Type)})
				}
			}
			if ft.Results != nil {
				for _, r := range ft.Results.List {
					count := max(len(r.Names), 1)
					for i := 0; i < count; i++ {
						m.results = append(m.results, expr(r.Type))
					}
				}
			}
			if len(m.results) == 0 || m.results[len(m.results)-1] != "error" || len(m.results) > 2 {
				return nil, fmt.Errorf("%s: methods must return (T, error) or error", n.Name)
			}
			if _, ok := docs[n.Name]; !ok {
				return nil, fmt.Errorf("%s has no method %s", typeName, n.Name)
			}
			lits, err := parseAnnotations(docs[n.Name], ann, usedImports)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", n.Name, err)
			}
			m.annotations = lits
			methods = append(methods, m)
		}
	}
	return render(pkg.Name, importPath, typeName, ifaceName, methods, usedImports)
}

func receiverName(fl *ast.FieldList) string {
	t := fl.List[0].Type
	if s, ok := t.(*ast.StarExpr); ok {
		t = s.X
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

var annRe = regexp.MustCompile(`^@([A-Z]\w*)(?:\((.*)\))?\s*$`)

// parseAnnotations turns "// @Audited(action = "X", logArguments = true)"
// into audit.Audited{Action: "X", LogArguments: true}.
func parseAnnotations(doc *ast.CommentGroup, ann annotationFlags, used map[string]string) ([]string, error) {
	if doc == nil {
		return nil, nil
	}
	var out []string
	for _, c := range doc.List {
		line := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
		m := annRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		path, known := ann[m[1]]
		if !known {
			return nil, fmt.Errorf("unknown annotation @%s (add -annotation %s=import/path)", m[1], m[1])
		}
		alias := filepath.Base(path)
		used[path] = alias
		var fields []string
		for _, kv := range splitArgs(m[2]) {
			k, v, ok := strings.Cut(kv, "=")
			if !ok {
				return nil, fmt.Errorf("@%s: expected key = value, got %q", m[1], kv)
			}
			k = strings.TrimSpace(k)
			r := []rune(k)
			r[0] = unicode.ToUpper(r[0])
			fields = append(fields, string(r)+": "+strings.TrimSpace(v))
		}
		out = append(out, fmt.Sprintf("%s.%s{%s}", alias, m[1], strings.Join(fields, ", ")))
	}
	return out, nil
}

func splitArgs(s string) []string {
	var out []string
	var cur strings.Builder
	inStr := false
	for _, r := range s {
		switch {
		case r == '"':
			inStr = !inStr
		case r == ',' && !inStr:
			if t := strings.TrimSpace(cur.String()); t != "" {
				out = append(out, t)
			}
			cur.Reset()
			continue
		}
		cur.WriteRune(r)
	}
	if t := strings.TrimSpace(cur.String()); t != "" {
		out = append(out, t)
	}
	return out
}

func importPathOf(dir string) (string, error) {
	for d := dir; ; d = filepath.Dir(d) {
		data, err := os.ReadFile(filepath.Join(d, "go.mod"))
		if err == nil {
			m := regexp.MustCompile(`(?m)^module\s+(\S+)`).FindSubmatch(data)
			rel, _ := filepath.Rel(d, dir)
			return filepath.ToSlash(filepath.Join(string(m[1]), rel)), nil
		}
		if filepath.Dir(d) == d {
			return "", fmt.Errorf("go.mod not found above %s", dir)
		}
	}
}

func render(pkg, importPath, typeName, iface string, methods []method, imports map[string]string) ([]byte, error) {
	proxy := typeName + "AopProxy"
	metas := typeName + "AopMethods"
	dotted := strings.ReplaceAll(importPath, "/", ".")
	var b bytes.Buffer
	w := func(f string, a ...any) { fmt.Fprintf(&b, f, a...) }

	w("// Code generated by aopgen. DO NOT EDIT.\n\npackage %s\n\nimport (\n\t\"reflect\"\n\n\t\"springlab/pkg/aop\"\n", pkg)
	paths := make([]string, 0, len(imports))
	for p := range imports {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		w("\t%q\n", p)
	}
	w(")\n\n")
	w("// %s is the generated proxy of *%s (the Go counterpart of\n", proxy, typeName)
	w("// %s$$SpringCGLIB$$0): it implements %s, holds the target and\n", iface, iface)
	w("// routes every call through the advice chain.\ntype %s struct {\n\ttarget *%s\n\th      *aop.Handler\n}\n\n", proxy, typeName)
	w("var _ %s = (*%s)(nil)\n\n", iface, proxy)
	w("func (p *%s) TargetSource() any { return p.target }\n\n", proxy)

	w("var %s = []*aop.Method{\n", metas)
	for _, m := range methods {
		w("\t{Package: %q, Type: %q, Name: %q, Exported: true", dotted, iface, m.name)
		if len(m.annotations) > 0 {
			w(", Annotations: []any{%s}", strings.Join(m.annotations, ", "))
		}
		w("},\n")
	}
	w("}\n\n")

	for i, m := range methods {
		var params, args, call []string
		for j, p := range m.params {
			params = append(params, fmt.Sprintf("a%d %s", j, p.typ))
			args = append(args, fmt.Sprintf("a%d", j))
			call = append(call, fmt.Sprintf("aop.Arg[%s](args, %d)", p.typ, j))
		}
		w("func (p *%s) %s(%s) ", proxy, m.name, strings.Join(params, ", "))
		if len(m.results) == 2 {
			w("(%s, error) {\n", m.results[0])
			w("\tres, err := p.h.Invoke(%s[%d], []any{%s}, func(args []any) (any, error) {\n",
				metas, i, strings.Join(args, ", "))
			w("\t\tr, err := p.target.%s(%s)\n\t\treturn r, err\n\t})\n", m.name, strings.Join(call, ", "))
			w("\tr0, _ := res.(%s)\n\treturn r0, err\n}\n\n", m.results[0])
		} else {
			w("error {\n")
			w("\t_, err := p.h.Invoke(%s[%d], []any{%s}, func(args []any) (any, error) {\n",
				metas, i, strings.Join(args, ", "))
			w("\t\treturn nil, p.target.%s(%s)\n\t})\n\treturn err\n}\n\n", m.name, strings.Join(call, ", "))
		}
	}
	w("func init() {\n\taop.RegisterProxy(reflect.TypeFor[*%s](), aop.ProxyFactory{\n", typeName)
	w("\t\tMethods: %s,\n\t\tNew: func(target any, h *aop.Handler) any {\n", metas)
	w("\t\t\tp := &%s{target: target.(*%s), h: h}\n\t\t\th.SetProxy(p)\n\t\t\treturn p\n\t\t},\n\t})\n}\n", proxy, typeName)

	src, err := format.Source(b.Bytes())
	if err != nil {
		return b.Bytes(), fmt.Errorf("formatting generated code: %w", err)
	}
	return src, nil
}
