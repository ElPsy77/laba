// Package web holds the HTTP controllers — the counterpart of kz.iitu.springlab.web.
package web

import (
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"springlab/internal/app"
)

// HelloController is the counterpart of @RestController @RequestMapping("/api").
type HelloController struct {
	owner string // @Value("${app.owner:unknown}")
}

// NewHelloController injects the settings through the constructor.
func NewHelloController(props app.Properties) *HelloController {
	return &HelloController{owner: props.Get("app.owner", "unknown")}
}

// Greeting and Info are the Java records; JSON tags play the role of Jackson.
type Greeting struct {
	Message   string `json:"message"`
	Owner     string `json:"owner"`
	Timestamp string `json:"timestamp"`
}

type Info struct {
	Owner     string `json:"owner"`
	GoVersion string `json:"goVersion"`
	CPUCores  int    `json:"cpuCores"`
	PID       int    `json:"pid"`
}

// Register is @GetMapping for each method of the controller.
func (c *HelloController) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/hello", c.Hello)
	mux.HandleFunc("GET /api/info", c.Info)
	mux.HandleFunc("GET /api/sum", c.Sum) // individual assignment, variant 1
}

// Hello — GET /api/hello?name=... with @RequestParam(defaultValue = "world").
func (c *HelloController) Hello(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "world"
	}
	app.WriteJSON(w, http.StatusOK, Greeting{
		Message:   "Hello, " + name + "!",
		Owner:     c.owner,
		Timestamp: time.Now().Format("2006-01-02T15:04:05.000"),
	})
}

// Info — GET /api/info.
func (c *HelloController) Info(w http.ResponseWriter, r *http.Request) {
	app.WriteJSON(w, http.StatusOK, Info{
		Owner:     c.owner,
		GoVersion: runtime.Version(),
		CPUCores:  runtime.NumCPU(),
		PID:       os.Getpid(),
	})
}

// SumResult is the response of the variant-1 endpoint.
type SumResult struct {
	A          int64 `json:"a"`
	B          int64 `json:"b"`
	Sum        int64 `json:"sum"`
	Difference int64 `json:"difference"`
	Product    int64 `json:"product"`
}

// Sum — variant 1: GET /api/sum?a=&b= returns the sum, difference and product.
// A missing or non-integer parameter gives status 400, as @RequestParam does.
func (c *HelloController) Sum(w http.ResponseWriter, r *http.Request) {
	a, errA := strconv.ParseInt(r.URL.Query().Get("a"), 10, 64)
	b, errB := strconv.ParseInt(r.URL.Query().Get("b"), 10, 64)
	if errA != nil || errB != nil {
		app.WriteError(w, r, http.StatusBadRequest,
			"Bad Request: parameters a and b must be integers")
		return
	}
	app.WriteJSON(w, http.StatusOK, SumResult{
		A: a, B: b, Sum: a + b, Difference: a - b, Product: a * b,
	})
}
