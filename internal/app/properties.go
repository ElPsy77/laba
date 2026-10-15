// Package app holds the application bootstrap: settings and routing.
package app

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Properties is a minimal reader of application.properties
// (key=value lines, '#' and '!' start a comment line).
type Properties map[string]string

// LoadProperties reads a .properties file. A missing file is not an error:
// the application then runs on its defaults, like Spring Boot does.
func LoadProperties(path string) (Properties, error) {
	props := Properties{}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return props, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("%s:%d: expected key=value", path, n)
		}
		props[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	return props, sc.Err()
}

// Get returns the value of key or def — the analogue of ${key:def}.
func (p Properties) Get(key, def string) string {
	if v, ok := p[key]; ok {
		return v
	}
	return def
}
