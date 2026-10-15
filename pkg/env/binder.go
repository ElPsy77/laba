package env

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// BindError describes one failed property, in the Spring Boot format.
type BindError struct {
	Property, Value, Origin, Reason string
}

// BindErrors is returned when binding or validation fails.
type BindErrors struct {
	Target string
	Errors []BindError
}

func (b *BindErrors) Error() string {
	var sb strings.Builder
	sb.WriteString("\n\n***************************\nAPPLICATION FAILED TO START\n***************************\n\n")
	sb.WriteString("Description:\n\n")
	fmt.Fprintf(&sb, "Binding to target %s failed:\n", b.Target)
	for _, e := range b.Errors {
		fmt.Fprintf(&sb, "\n    Property: %s\n    Value: %q\n    Origin: %s\n    Reason: %s\n",
			e.Property, e.Value, e.Origin, e.Reason)
	}
	sb.WriteString("\n\nAction:\n\nUpdate your application's configuration\n")
	return sb.String()
}

// Bind is @ConfigurationProperties(prefix = ...) + @Validated: it fills the
// struct pointed to by target from the branch of settings under prefix and
// then checks the `validate` constraints. Struct tags:
//
//	prop:"retry-count"   the key (default: field name in kebab-case)
//	default:"3"          @DefaultValue("3")
//	validate:"min=1;max=10"  Bean Validation constraints (separated by ;)
//	validate:"valid"     @Valid: also validate the nested struct
func (e *Environment) Bind(prefix string, target any) error {
	v := reflect.ValueOf(target).Elem()
	errs := &BindErrors{Target: v.Type().PkgPath() + "." + v.Type().Name()}
	e.bindStruct(prefix, v, true, errs)
	if len(errs.Errors) > 0 {
		return errs
	}
	return nil
}

func (e *Environment) bindStruct(prefix string, v reflect.Value, validate bool, errs *BindErrors) {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		key := f.Tag.Get("prop")
		if key == "" {
			key = kebab(f.Name)
		}
		full := prefix + "." + key
		fv := v.Field(i)
		rules := splitRules(f.Tag.Get("validate"))

		if f.Type.Kind() == reflect.Struct && f.Type != reflect.TypeFor[time.Duration]() {
			_, nested := rules["valid"]
			e.bindStruct(full, fv, validate && nested, errs)
			continue
		}

		raw, origin, found := e.raw(full, f.Type)
		if !found {
			if d, ok := f.Tag.Lookup("default"); ok {
				raw, origin, found = d, "@DefaultValue", true
			}
		}
		if found {
			if err := setValue(fv, raw, e); err != nil {
				errs.Errors = append(errs.Errors, BindError{full, fmt.Sprint(raw), origin,
					"failed to convert java.lang.String to " + f.Type.String() + " (" + err.Error() + ")"})
				continue
			}
		}
		if !validate {
			continue
		}
		if reason := check(fv, rules); reason != "" {
			errs.Errors = append(errs.Errors, BindError{full, display(raw), origin, reason})
		}
	}
}

// raw returns the textual value of a key: a scalar, or for a list the
// items key[0], key[1]... joined by commas.
func (e *Environment) raw(key string, t reflect.Type) (string, string, bool) {
	if t.Kind() == reflect.Slice {
		var items []string
		origin := ""
		for i := 0; ; i++ {
			en, _, ok := e.LookupEntry(fmt.Sprintf("%s[%d]", key, i))
			if !ok {
				break
			}
			if origin == "" {
				origin = en.Origin
			}
			items = append(items, e.Resolve(en.Value))
		}
		if len(items) > 0 {
			return strings.Join(items, ","), origin, true
		}
	}
	en, _, ok := e.LookupEntry(key)
	if !ok {
		return "", "", false
	}
	return e.Resolve(en.Value), en.Origin, true
}

func display(raw string) string { return raw }

func setValue(fv reflect.Value, raw string, _ *Environment) error {
	switch {
	case fv.Type() == reflect.TypeFor[time.Duration]():
		d, err := ParseDuration(raw)
		if err != nil {
			return err
		}
		fv.SetInt(int64(d))
	case fv.Type() == reflect.TypeFor[DataSize]():
		d, err := ParseDataSize(raw)
		if err != nil {
			return err
		}
		fv.SetInt(int64(d))
	case fv.Kind() == reflect.String:
		fv.SetString(raw)
	case fv.Kind() == reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		fv.SetBool(b)
	case fv.Kind() >= reflect.Int && fv.Kind() <= reflect.Int64:
		n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			return err
		}
		fv.SetInt(n)
	case fv.Kind() == reflect.Slice && fv.Type().Elem().Kind() == reflect.String:
		var items []string
		for _, s := range strings.Split(raw, ",") {
			if s = strings.TrimSpace(s); s != "" {
				items = append(items, s)
			}
		}
		fv.Set(reflect.ValueOf(items))
	default:
		return fmt.Errorf("unsupported type %s", fv.Type())
	}
	return nil
}

// ParseDuration accepts the Spring Boot forms: 5s, 2m, 500ms, PT15M,
// and a bare number of milliseconds.
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Duration(n) * time.Millisecond, nil
	}
	if strings.HasPrefix(strings.ToUpper(s), "PT") {
		return time.ParseDuration(strings.ToLower(s[2:]))
	}
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		return time.Duration(n) * 24 * time.Hour, err
	}
	return time.ParseDuration(s)
}

// DataSize is the Spring DataSize type: 10MB, 512KB.
type DataSize int64

func (d DataSize) String() string {
	switch {
	case d%(1<<20) == 0:
		return fmt.Sprintf("%dMB", d>>20)
	case d%(1<<10) == 0:
		return fmt.Sprintf("%dKB", d>>10)
	}
	return fmt.Sprintf("%dB", int64(d))
}

func ParseDataSize(s string) (DataSize, error) {
	units := []struct {
		suffix string
		mul    int64
	}{{"GB", 1 << 30}, {"MB", 1 << 20}, {"KB", 1 << 10}, {"B", 1}}
	u := strings.ToUpper(strings.TrimSpace(s))
	for _, x := range units {
		if strings.HasSuffix(u, x.suffix) {
			n, err := strconv.ParseInt(strings.TrimSuffix(u, x.suffix), 10, 64)
			return DataSize(n * x.mul), err
		}
	}
	n, err := strconv.ParseInt(u, 10, 64)
	return DataSize(n), err
}

// ---- validation (Jakarta Bean Validation subset) -------------------------

func splitRules(tag string) map[string]string {
	out := map[string]string{}
	if tag == "" {
		return out
	}
	for _, r := range strings.Split(tag, ";") {
		k, v, _ := strings.Cut(strings.TrimSpace(r), "=")
		out[k] = v
	}
	return out
}

var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// check returns the first violated constraint, with Spring's wording.
func check(fv reflect.Value, rules map[string]string) string {
	for _, name := range []string{"notnull", "notblank", "notempty", "email", "oneof",
		"pattern", "positive", "min", "max", "size"} {
		arg, ok := rules[name]
		if !ok {
			continue
		}
		switch name {
		case "notnull":
			if fv.IsZero() {
				return "must not be null"
			}
		case "notblank":
			if strings.TrimSpace(fv.String()) == "" {
				return "must not be blank"
			}
		case "notempty":
			if fv.Len() == 0 {
				return "must not be empty"
			}
		case "email":
			items := []string{}
			if fv.Kind() == reflect.Slice {
				items = fv.Interface().([]string)
			} else if fv.String() != "" {
				items = []string{fv.String()}
			}
			for _, s := range items {
				if !emailRe.MatchString(s) {
					return "must be a well-formed email address"
				}
			}
		case "oneof":
			allowed := strings.Fields(arg)
			ok := false
			for _, a := range allowed {
				ok = ok || a == fv.String()
			}
			if !ok {
				return "must be one of " + strings.Join(allowed, ", ")
			}
		case "pattern":
			if !regexp.MustCompile(arg).MatchString(fv.String()) {
				return fmt.Sprintf("must match \"%s\"", arg)
			}
		case "positive":
			if fv.Int() <= 0 {
				return "must be greater than 0"
			}
		case "min":
			n, _ := strconv.ParseInt(arg, 10, 64)
			if fv.Int() < n {
				return "must be greater than or equal to " + arg
			}
		case "max":
			n, _ := strconv.ParseInt(arg, 10, 64)
			if fv.Int() > n {
				return "must be less than or equal to " + arg
			}
		case "size":
			lo, _ := strconv.Atoi(arg)
			if fv.Len() < lo {
				return fmt.Sprintf("size must be at least %d", lo)
			}
		}
	}
	return ""
}

// kebab: RetryCount -> retry-count (the canonical form of the key).
func kebab(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('-')
			}
			r = unicode.ToLower(r)
		}
		b.WriteRune(r)
	}
	return b.String()
}
