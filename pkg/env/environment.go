// Package env is the Spring Environment: an ordered list of property
// sources. The value of a key is taken from the FIRST source that has it,
// and the more "external" a source is, the higher it stands in the list.
package env

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Source is one PropertySource.
type Source struct {
	Name    string
	entries map[string]Entry // normalised key -> entry
	keys    []string         // original keys in file order
	lookup  func(key string) (Entry, bool)
}

func newSource(name string, list []Entry) *Source {
	s := &Source{Name: name, entries: map[string]Entry{}}
	for _, e := range list {
		nk := normalize(e.Key)
		if _, seen := s.entries[nk]; !seen {
			s.keys = append(s.keys, e.Key)
		}
		s.entries[nk] = e
	}
	return s
}

// Get looks a key up in this source only.
func (s *Source) Get(key string) (Entry, bool) {
	if s.lookup != nil {
		return s.lookup(key)
	}
	e, ok := s.entries[normalize(key)]
	return e, ok
}

// Entries lists the properties of the source in declaration order.
func (s *Source) Entries() []Entry {
	if s.lookup != nil { // environment variables: list the relevant ones
		var out []Entry
		for _, kv := range os.Environ() {
			k, v, _ := strings.Cut(kv, "=")
			for _, p := range []string{"APP_", "SPRING_", "SERVER_", "MANAGEMENT_", "LOGGING_"} {
				if strings.HasPrefix(k, p) {
					out = append(out, Entry{Key: k, Value: v, Origin: "System Environment Property \"" + k + "\""})
				}
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
		return out
	}
	out := make([]Entry, len(s.keys))
	for i, k := range s.keys {
		out[i] = s.entries[normalize(k)]
	}
	return out
}

// normalize implements relaxed binding: retry-count, retryCount,
// retry_count and RETRYCOUNT all become "retrycount".
func normalize(key string) string {
	return strings.NewReplacer("-", "", "_", "").Replace(strings.ToLower(key))
}

// envVarName: app.mail.retry-count -> APP_MAIL_RETRYCOUNT,
// app.mail.recipients[0] -> APP_MAIL_RECIPIENTS_0_.
func envVarName(key string) string {
	r := strings.NewReplacer(".", "_", "-", "", "[", "_", "]", "_")
	return strings.ToUpper(r.Replace(key))
}

// Environment holds the sources and the active profiles.
type Environment struct {
	sources  []*Source
	profiles []string
	Debug    bool
}

// Options say where to look for the files.
type Options struct {
	Args         []string // os.Args[1:]
	ResourcesDir string   // the "classpath": files packed with the artefact
	ConfigDir    string   // ./config next to the binary
}

// Load assembles the environment in Spring Boot order (strongest first):
//  1. command-line arguments          --app.mail.retry-count=9
//  2. "system properties"             -Dapp.mail.retry-count=5
//  3. environment variables           APP_MAIL_RETRYCOUNT=7
//  4. ./config/application-{profile}.yml
//  5. ./config/application.yml
//  6. application-{profile}.yml inside the artefact
//  7. application.properties inside the artefact (beats .yml)
//  8. application.yml inside the artefact
func Load(o Options) (*Environment, error) {
	e := &Environment{}
	var argv, sysProps []Entry
	for _, a := range o.Args {
		switch {
		case a == "--debug":
			e.Debug = true
			argv = append(argv, Entry{Key: "debug", Value: "true", Origin: "\"--debug\" from command line"})
		case strings.HasPrefix(a, "--"):
			k, v, _ := strings.Cut(a[2:], "=")
			argv = append(argv, Entry{Key: k, Value: v, Origin: fmt.Sprintf("\"%s\" from command line", a)})
		case strings.HasPrefix(a, "-D"):
			k, v, _ := strings.Cut(a[2:], "=")
			sysProps = append(sysProps, Entry{Key: k, Value: v, Origin: "System Property \"" + k + "\""})
		}
	}
	envSource := &Source{Name: "systemEnvironment", lookup: func(key string) (Entry, bool) {
		name := envVarName(key)
		v, ok := os.LookupEnv(name)
		return Entry{Key: name, Value: v, Origin: "System Environment Property \"" + name + "\""}, ok
	}}
	e.sources = append(e.sources,
		newSource("commandLineArgs", argv),
		newSource("systemProperties", sysProps),
		envSource)

	// Files without profile first: they are needed to learn the active profile.
	base, err := e.fileSources(o, "")
	if err != nil {
		return nil, err
	}
	e.sources = append(e.sources, base...)
	e.profiles = e.resolveProfiles()

	// Profile-specific files sit above the shared file of the same location.
	// When several profiles are active, the last one wins.
	var outside, inside []*Source
	for i := len(e.profiles) - 1; i >= 0; i-- {
		p := e.profiles[i]
		if s, err := loadFile(filepath.Join(o.ConfigDir, "application-"+p+".yml"),
			"file [config/application-"+p+".yml]"); err != nil {
			return nil, err
		} else if s != nil {
			outside = append(outside, s)
		}
		if s, err := loadFile(filepath.Join(o.ResourcesDir, "application-"+p+".yml"),
			"class path resource [application-"+p+".yml]"); err != nil {
			return nil, err
		} else if s != nil {
			inside = append(inside, s)
		}
	}
	var ordered []*Source
	ordered = append(ordered, e.sources[:3]...)
	ordered = append(ordered, outside...)
	for _, s := range base {
		if strings.HasPrefix(s.Name, "Config resource 'file") {
			ordered = append(ordered, s)
		}
	}
	ordered = append(ordered, inside...)
	for _, s := range base {
		if !strings.HasPrefix(s.Name, "Config resource 'file") {
			ordered = append(ordered, s)
		}
	}
	e.sources = ordered
	return e, nil
}

func (e *Environment) fileSources(o Options, _ string) ([]*Source, error) {
	var out []*Source
	candidates := []struct{ path, display string }{
		{filepath.Join(o.ConfigDir, "application.yml"), "file [config/application.yml]"},
		{filepath.Join(o.ResourcesDir, "application.properties"), "class path resource [application.properties]"},
		{filepath.Join(o.ResourcesDir, "application.yml"), "class path resource [application.yml]"},
	}
	for _, c := range candidates {
		s, err := loadFile(c.path, c.display)
		if err != nil {
			return nil, err
		}
		if s != nil {
			out = append(out, s)
		}
	}
	return out, nil
}

func loadFile(path, display string) (*Source, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}
	var entries []Entry
	var err error
	if strings.HasSuffix(path, ".properties") {
		entries, err = parseProperties(path, display)
	} else {
		entries, err = parseYAML(path, display)
	}
	if err != nil {
		return nil, err
	}
	location := "optional:classpath:/"
	if strings.HasPrefix(display, "file") {
		location = "optional:file:./config/"
	}
	return newSource("Config resource '"+display+"' via location '"+location+"'", entries), nil
}

func parseProperties(path, display string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Entry
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || line[0] == '#' || line[0] == '!' {
			continue
		}
		k, v, _ := strings.Cut(line, "=")
		out = append(out, Entry{Key: strings.TrimSpace(k), Value: strings.TrimSpace(v),
			Origin: fmt.Sprintf("%s - %d:%d", display, n, len(k)+2)})
	}
	return out, sc.Err()
}

func (e *Environment) resolveProfiles() []string {
	v, _, ok := e.Lookup("spring.profiles.active")
	if !ok || strings.TrimSpace(v) == "" {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	var add func(p string)
	add = func(p string) {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
		// profile groups: spring.profiles.group.prod=prod,metrics
		if g, _, ok := e.Lookup("spring.profiles.group." + p); ok {
			for _, m := range strings.Split(g, ",") {
				add(m)
			}
		}
	}
	for _, p := range strings.Split(v, ",") {
		add(p)
	}
	return out
}

// ---- reading -------------------------------------------------------------

// Lookup returns the raw value, the name of the source that won, and ok.
func (e *Environment) Lookup(key string) (string, *Source, bool) {
	for _, s := range e.sources {
		if en, ok := s.Get(key); ok {
			return en.Value, s, true
		}
	}
	return "", nil, false
}

// LookupEntry also returns the entry (with its origin).
func (e *Environment) LookupEntry(key string) (Entry, *Source, bool) {
	for _, s := range e.sources {
		if en, ok := s.Get(key); ok {
			return en, s, true
		}
	}
	return Entry{}, nil, false
}

var placeholder = regexp.MustCompile(`\$\{([^}:]+)(?::([^}]*))?\}`)

// Get is ${key:def}: the value with ${...} placeholders resolved.
func (e *Environment) Get(key, def string) string {
	v, _, ok := e.Lookup(key)
	if !ok {
		return def
	}
	return e.Resolve(v)
}

// Property implements ioc.PropertyResolver.
func (e *Environment) Property(key string) (string, bool) {
	v, _, ok := e.Lookup(key)
	return e.Resolve(v), ok
}

// Resolve replaces ${NAME:default} placeholders, e.g. ${DB_PASSWORD}.
func (e *Environment) Resolve(v string) string {
	return placeholder.ReplaceAllStringFunc(v, func(m string) string {
		p := placeholder.FindStringSubmatch(m)
		return e.Get(p[1], p[2])
	})
}

// ActiveProfiles — environment.getActiveProfiles().
func (e *Environment) ActiveProfiles() []string { return append([]string(nil), e.profiles...) }

// Sources lists the property sources, strongest first.
func (e *Environment) Sources() []*Source { return e.sources }

// FromMap builds an environment from a plain map (tests).
func FromMap(m map[string]string) *Environment {
	var list []Entry
	for k, v := range m {
		list = append(list, Entry{Key: k, Value: v, Origin: "test"})
	}
	return &Environment{sources: []*Source{newSource("test", list)}}
}
