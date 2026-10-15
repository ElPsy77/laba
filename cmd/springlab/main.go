// Command springlab is the start-up class of the application —
// the Go counterpart of SpringLabApplication with @SpringBootApplication.
package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime"
	"time"

	"springlab/internal/app"
)

func main() {
	started := time.Now()
	printBanner()

	props, err := app.LoadProperties("resources/application.properties")
	if err != nil {
		slog.Error("Application run failed", "error", err)
		os.Exit(1)
	}

	slog.Info(fmt.Sprintf("Starting SpringLabApplication using Go %s with PID %d",
		runtime.Version(), os.Getpid()))

	mux := app.NewRouter(props)
	port := props.Get("server.port", "8080")

	slog.Info(fmt.Sprintf("HTTP server initialized with port %s (http)", port))
	slog.Info(fmt.Sprintf("Started SpringLabApplication in %.3f seconds",
		time.Since(started).Seconds()))

	if err := http.ListenAndServe(":"+port, mux); err != nil {
		slog.Error("Web server failed to start", "error", err)
		os.Exit(1)
	}
}

func printBanner() {
	fmt.Print(`
   ____  ___        _          _
  / ___|/ _ \      | |    __ _| |__
 | |  _| | | |_____| |   / _' | '_ \
 | |_| | |_| |_____| |__| (_| | |_) |
  \____|\___/      |_____\__,_|_.__/   spring-lab-01 (Go edition)
`)
}
