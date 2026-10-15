package env

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const sharedYML = `app:
  mail:
    from: no-reply@iitu.kz   # comment
    retry-count: 3
    timeout: 5s
    recipients:
      - dean@iitu.kz
      - registrar@iitu.kz
  section:                   # a comment right after the key
    name: x
`

func setup(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		p := filepath.Join(dir, name)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func load(t *testing.T, dir string, args ...string) *Environment {
	t.Helper()
	e, err := Load(Options{Args: args, ResourcesDir: filepath.Join(dir, "resources"),
		ConfigDir: filepath.Join(dir, "config")})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func TestYAMLFlattening(t *testing.T) {
	e := load(t, setup(t, map[string]string{"resources/application.yml": sharedYML}))
	for key, want := range map[string]string{
		"app.mail.from":          "no-reply@iitu.kz",
		"app.mail.recipients[1]": "registrar@iitu.kz",
		"app.section.name":       "x",
		"app.mail.retryCount":    "3", // relaxed binding: camelCase finds the kebab key
	} {
		if got := e.Get(key, "?"); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestPrecedence(t *testing.T) {
	dir := setup(t, map[string]string{
		"resources/application.yml":     sharedYML,
		"resources/application-dev.yml": "app:\n  mail:\n    retry-count: 1\n",
	})
	check := func(want string, args ...string) {
		t.Helper()
		if got := load(t, dir, args...).Get("app.mail.retry-count", ""); got != want {
			t.Errorf("args %v: got %s, want %s", args, got, want)
		}
	}
	check("3")
	check("1", "--spring.profiles.active=dev")
	t.Setenv("APP_MAIL_RETRYCOUNT", "7")
	check("7", "--spring.profiles.active=dev")
	check("5", "-Dapp.mail.retry-count=5")
	check("9", "-Dapp.mail.retry-count=5", "--app.mail.retry-count=9")
}

func TestPropertiesBeatYAMLInSameLocation(t *testing.T) {
	dir := setup(t, map[string]string{
		"resources/application.yml":        sharedYML,
		"resources/application.properties": "app.mail.retry-count=8\n",
	})
	if got := load(t, dir).Get("app.mail.retry-count", ""); got != "8" {
		t.Fatalf("got %s: a leftover application.properties must win over application.yml", got)
	}
}

type mail struct {
	From       string        `validate:"notblank;email"`
	RetryCount int           `default:"3" validate:"min=1;max=10"`
	Timeout    time.Duration `default:"1s"`
	Recipients []string      `validate:"notempty;email"`
}
type root struct {
	Mail mail `validate:"valid"`
}

func TestBindAndValidate(t *testing.T) {
	e := load(t, setup(t, map[string]string{"resources/application.yml": sharedYML}))
	var r root
	if err := e.Bind("app", &r); err != nil {
		t.Fatal(err)
	}
	if r.Mail.Timeout != 5*time.Second || len(r.Mail.Recipients) != 2 || r.Mail.RetryCount != 3 {
		t.Fatalf("bound %+v", r)
	}

	bad := load(t, setup(t, map[string]string{"resources/application.yml": sharedYML}),
		"--app.mail.retry-count=99")
	err := bad.Bind("app", &root{})
	var be *BindErrors
	if !errors.As(err, &be) || be.Errors[0].Property != "app.mail.retry-count" ||
		!strings.Contains(be.Errors[0].Reason, "less than or equal to 10") {
		t.Fatalf("got %v", err)
	}
}

func TestTabIsRejected(t *testing.T) {
	dir := setup(t, map[string]string{"resources/application.yml": "app:\n\tname: x\n"})
	_, err := Load(Options{ResourcesDir: filepath.Join(dir, "resources")})
	if err == nil || !strings.Contains(err.Error(), "found character '\\t'") {
		t.Fatalf("got %v", err)
	}
}
