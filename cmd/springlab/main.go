// Command springlab is the start-up class of the application —
// the Go counterpart of SpringLabApplication with @SpringBootApplication.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"springlab/internal/app"
	"springlab/internal/web"
	"springlab/pkg/ioc"

	// @ComponentScan: a package takes part in the container only if it is
	// imported here (directly or through another package). Each package
	// registers its components in init().
	_ "springlab/internal/config"
	_ "springlab/internal/lifecycle"
	_ "springlab/internal/notify"
	_ "springlab/internal/scope"
)

func main() {
	started := time.Now()
	printBanner()

	props, err := app.LoadProperties("resources/application.properties")
	if err != nil {
		fail(err)
	}
	slog.Info(fmt.Sprintf("Starting SpringLabApplication using Go %s with PID %d",
		runtime.Version(), os.Getpid()))

	// 1. read every bean definition, 2. create the singletons
	ctx, err := ioc.New(map[string]any{"applicationProperties": props})
	if err != nil {
		fail(err)
	}
	if err := ctx.Refresh(); err != nil {
		fail(err)
	}

	// DispatcherServlet: collect every controller bean.
	mux := app.NewRouter(props)
	controllers, err := ioc.BeansOfType[web.Controller](ctx)
	if err != nil {
		fail(err)
	}
	for _, c := range controllers {
		c.Register(mux)
	}

	port := props.Get("server.port", "8080")
	srv := &http.Server{Addr: ":" + port, Handler: mux}
	slog.Info(fmt.Sprintf("HTTP server initialized with port %s (http)", port))
	slog.Info(fmt.Sprintf("Started SpringLabApplication in %.3f seconds",
		time.Since(started).Seconds()))

	runners, err := ioc.BeansOfType[ioc.CommandLineRunner](ctx)
	if err != nil {
		fail(err)
	}
	for _, r := range runners {
		if err := r.Run(os.Args[1:]); err != nil {
			fail(err)
		}
	}

	// Graceful stop on Ctrl+C / SIGTERM: the context is closed and
	// @PreDestroy runs. SIGKILL (kill -9) cannot be caught — no @PreDestroy.
	stop, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go func() {
		<-stop.Done()
		slog.Info("Shutting down: closing the application context")
		shutdownCtx, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = srv.Shutdown(shutdownCtx)
	}()

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("Web server failed to start", "error", err)
		ctx.Close()
		os.Exit(1)
	}
	ctx.Close()
}

func fail(err error) {
	slog.Error("Application run failed", "error", err)
	os.Exit(1)
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
