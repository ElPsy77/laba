package env

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Entry is one flattened property with its origin (file, line, column).
type Entry struct {
	Key    string
	Value  string
	Origin string
}

// parseYAML reads the subset of YAML used for configuration files:
// nested mappings, block lists ("- item"), scalars, quotes and comments.
// The result is flattened to dotted keys: app.mail.retry-count,
// app.mail.recipients[0] — exactly the keys Spring Boot works with.
func parseYAML(path, display string) ([]Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	type frame struct {
		indent int
		key    string
	}
	var (
		stack   []frame
		out     []Entry
		counter = map[string]int{} // next list index per parent path
	)
	parent := func() string {
		keys := make([]string, len(stack))
		for i, f := range stack {
			keys[i] = f.key
		}
		return strings.Join(keys, ".")
	}
	for n, raw := range strings.Split(string(data), "\n") {
		lineNo := n + 1
		body := strings.TrimRight(raw, " \r")
		trimmed := strings.TrimLeft(body, " \t")
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || trimmed == "---" {
			continue
		}
		lead := body[:len(body)-len(trimmed)]
		if strings.Contains(lead, "\t") {
			return nil, fmt.Errorf("while scanning for the next token found character '\\t' "+
				"that cannot start any token. (Do not use \\t(TAB) for indentation)\n in '%s', line %d, column %d",
				display, lineNo, strings.Index(lead, "\t")+1)
		}
		indent := len(lead)
		item := trimmed == "-" || strings.HasPrefix(trimmed, "- ")
		for len(stack) > 0 {
			top := stack[len(stack)-1]
			if top.indent > indent || (top.indent == indent && !item) {
				stack = stack[:len(stack)-1]
				continue
			}
			break
		}
		origin := func(col int) string { return fmt.Sprintf("%s - %d:%d", display, lineNo, col) }

		if item {
			p := parent()
			idx := counter[p]
			counter[p] = idx + 1
			val := cleanScalar(strings.TrimSpace(strings.TrimPrefix(trimmed, "-")))
			out = append(out, Entry{Key: fmt.Sprintf("%s[%d]", p, idx), Value: val,
				Origin: origin(indent + 3)})
			continue
		}
		key, rest, ok := strings.Cut(trimmed, ":")
		if !ok {
			return nil, fmt.Errorf("could not find expected ':' in '%s', line %d", display, lineNo)
		}
		key = strings.TrimSpace(key)
		full := key
		if p := parent(); p != "" {
			full = p + "." + key
		}
		val := cleanScalar(strings.TrimSpace(rest))
		if val == "" {
			stack = append(stack, frame{indent: indent, key: key})
			continue
		}
		col := indent + len(key) + 3
		out = append(out, Entry{Key: full, Value: val, Origin: origin(col)})
	}
	return out, nil
}

// cleanScalar removes an inline comment and surrounding quotes.
func cleanScalar(s string) string {
	if s == "" || s[0] == '#' {
		return ""
	}
	if s[0] == '"' || s[0] == '\'' {
		if end := strings.IndexByte(s[1:], s[0]); end >= 0 {
			v := s[1 : end+1]
			if s[0] == '"' {
				if u, err := strconv.Unquote(`"` + v + `"`); err == nil {
					return u
				}
			}
			return v
		}
	}
	if i := strings.Index(s, " #"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}
