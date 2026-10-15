package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"springlab/internal/app"
	"springlab/pkg/env"
)

func newMux() *http.ServeMux {
	mux := app.NewRouter()
	NewHelloController(env.FromMap(map[string]string{"app.owner": "Karimbayev Makzhan, 2502IS"})).Register(mux)
	return mux
}

func get(t *testing.T, url string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	newMux().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	return rec
}

func TestHello(t *testing.T) {
	cases := map[string]string{
		"/api/hello":           "Hello, world!",
		"/api/hello?name=IITU": "Hello, IITU!",
	}
	for url, want := range cases {
		var g Greeting
		rec := get(t, url)
		_ = json.Unmarshal(rec.Body.Bytes(), &g)
		if rec.Code != 200 || g.Message != want || g.Owner == "" {
			t.Errorf("%s: got %d %+v", url, rec.Code, g)
		}
	}
}

func TestSumThreeDataSets(t *testing.T) {
	cases := []struct {
		url             string
		sum, diff, prod int64
	}{
		{"/api/sum?a=7&b=3", 10, 4, 21},
		{"/api/sum?a=-5&b=12", 7, -17, -60},
		{"/api/sum?a=0&b=0", 0, 0, 0},
	}
	for _, c := range cases {
		var r SumResult
		rec := get(t, c.url)
		_ = json.Unmarshal(rec.Body.Bytes(), &r)
		if rec.Code != 200 || r.Sum != c.sum || r.Difference != c.diff || r.Product != c.prod {
			t.Errorf("%s: got %d %+v", c.url, rec.Code, r)
		}
	}
}

func TestSumBadRequest(t *testing.T) {
	for _, url := range []string{"/api/sum", "/api/sum?a=1", "/api/sum?a=x&b=2"} {
		if rec := get(t, url); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: want 400, got %d", url, rec.Code)
		}
	}
}

func TestUnknownPathIs404(t *testing.T) {
	if rec := get(t, "/api/none"); rec.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", rec.Code)
	}
}
