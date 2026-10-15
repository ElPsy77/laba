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
	"strings"
	"syscall"
	"time"

	"springlab/internal/app"
	"springlab/internal/web"
	"springlab/pkg/env"
	"springlab/pkg/ioc"

	// @ComponentScan: a package takes part in the container only if it is
	// imported here (directly or through another package). Each package
	// registers its components in init().
	_ "springlab/internal/aspect"
	_ "springlab/internal/autoconfigure" // auto-configurations (.imports)
	_ "springlab/internal/config"
	_ "springlab/internal/lifecycle"
	_ "springlab/internal/notify"
	_ "springlab/internal/scope"
	_ "springlab/internal/service"

	// spring-boot-starter-aop: importing the AOP package switches on the
	// auto-proxy creator (AopAutoConfiguration).
	_ "springlab/pkg/aop"
)

func main() {
	started := time.Now()
	printBanner()

	environment, err := env.Load(env.Options{
		Args:         os.Args[1:],
		ResourcesDir: "resources", // inside the artefact ("classpath")
		ConfigDir:    "config",    // ./config next to the binary
	})
	if err != nil {
		fail(err)
	}
	configureLogging(environment)

	slog.Info(fmt.Sprintf("Starting SpringLabApplication using Go %s with PID %d",
		runtime.Version(), os.Getpid()))
	if p := environment.ActiveProfiles(); len(p) > 0 {
		slog.Info(fmt.Sprintf("The following %d profile is active: \"%s\"", len(p), strings.Join(p, "\", \"")))
	} else {
		slog.Info("No active profile set, falling back to 1 default profile: \"default\"")
	}

	// 1. read every bean definition (conditions are checked here),
	// 2. create the singletons
	ctx, err := ioc.New(environment, map[string]any{"environment": environment})
	if err != nil {
		fail(err)
	}
	if environment.Debug {
		printConditionsReport(ctx.ConditionsReport())
	}
	if err := ctx.Refresh(); err != nil {
		fail(err)
	}

	// DispatcherServlet: collect every controller bean.
	mux := app.NewRouter()
	controllers, err := ioc.BeansOfType[web.Controller](ctx)
	if err != nil {
		fail(err)
	}
	for _, c := range controllers {
		c.Register(mux)
	}

	port := environment.Get("server.port", "8080")
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
	var bind *env.BindErrors
	if errors.As(err, &bind) {
		fmt.Fprint(os.Stderr, bind.Error())
		os.Exit(1)
	}
	slog.Error("Application run failed", "error", err)
	os.Exit(1)
}

// configureLogging applies logging.level.kz.iitu (the application's loggers).
func configureLogging(e *env.Environment) {
	levels := map[string]slog.Level{"TRACE": slog.LevelDebug, "DEBUG": slog.LevelDebug,
		"INFO": slog.LevelInfo, "WARN": slog.LevelWarn, "ERROR": slog.LevelError}
	if lvl, ok := levels[strings.ToUpper(e.Get("logging.level.kz.iitu", "INFO"))]; ok {
		slog.SetLogLoggerLevel(lvl)
	}
}

// printConditionsReport is what --debug prints: the auto-configuration report.
func printConditionsReport(r ioc.Report) {
	fmt.Print("\n============================\nCONDITIONS EVALUATION REPORT\n============================\n\n")
	fmt.Print("Positive matches:\n-----------------\n\n")
	for _, o := range r.Positive {
		fmt.Printf("   %s matched:\n      - %s %s (%s)\n\n", o.Source, o.Condition, o.Message, o.Bean)
	}
	fmt.Print("Negative matches:\n-----------------\n\n")
	for _, o := range r.Negative {
		fmt.Printf("   %s:\n      Did not match:\n         - %s %s (%s)\n\n", o.Source, o.Condition, o.Message, o.Bean)
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
