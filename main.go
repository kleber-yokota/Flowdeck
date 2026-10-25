package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"flowdeck/core"
	"flowdeck/web"

	_ "flowdeck/workflows" // the workflow files register themselves in the core DBOS context (init); zz_generated.go pulls in every subpackage
)

// The source of the workflow packages, embedded so the core's paths parser
// can read it at boot — inside the container the source is not on disk.
// The directory pattern embeds the whole tree (subpackages included), so a
// new workflow folder is picked up by the parser for free.
//
//go:embed workflows
var workflowsSource embed.FS

func main() {
	healthcheck := flag.Bool("healthcheck", false,
		"ping the running server's /healthz route and exit (used by the container healthcheck; distroless has no curl)")
	flag.Parse()
	if *healthcheck {
		os.Exit(healthcheckExit(os.Getenv("FLOWDECK_HTTP_ADDR")))
	}
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

// config is the process configuration, read from the environment.
type config struct {
	httpAddr    string
	httpTimeout time.Duration
	coreTimeout time.Duration
}

func loadConfig() (config, error) {
	// The core reads these in its own init; validate here for an early
	// error before the web boots.
	if os.Getenv("DBOS_APP_NAME") == "" || os.Getenv("DBOS_SYSTEM_DATABASE_URL") == "" {
		return config{}, errors.New("DBOS_APP_NAME and DBOS_SYSTEM_DATABASE_URL are required")
	}
	return config{
		httpAddr:    envOr("FLOWDECK_HTTP_ADDR", ":8080"),
		httpTimeout: envDuration("FLOWDECK_HTTP_TIMEOUT", 5*time.Second),
		coreTimeout: envDuration("FLOWDECK_CORE_TIMEOUT", 5*time.Second),
	}, nil
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		slog.Warn("invalid duration env, using default",
			"env", key, "value", v, "default", def.String(), "err", err)
		return def
	}
	return d
}

func run() error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	// SIGINT + SIGTERM (docker compose stop) → clean shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	defer core.Shutdown(cfg.coreTimeout)

	// The possible-paths graph (phase 4, STEP 1): parse the embedded
	// workflows source (each file's RegisterWorkflow + the workflow funcs),
	// once, before the web serves it.
	if err := core.SetWorkflowsSource(workflowsSource); err != nil {
		return err
	}

	// The core DBOS context is up since core's init (env), and the workflow
	// files registered their workflows there in their init — Launch starts
	// the executor.
	if err := core.Launch(); err != nil {
		return err
	}
	slog.Info("core DBOS is up")

	// Listen before logging, so the log line only prints if the port is free
	ln, err := net.Listen("tcp", cfg.httpAddr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.httpAddr, err)
	}

	// Web: an abstraction over the core — lists what the core registered
	// and dispatches through it (enqueue → DBOS queue)
	srv := &http.Server{
		Addr:              cfg.httpAddr,
		Handler:           web.New(core.NewPort(), core.Ready),
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       15 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Serve(ln) }()
	slog.Info("web listening", "addr", cfg.httpAddr)

	select {
	case <-ctx.Done():
		shCtx, cancel := context.WithTimeout(context.Background(), cfg.httpTimeout)
		defer cancel()
		return srv.Shutdown(shCtx)
	case err := <-errCh:
		// ErrServerClosed is the expected return from Serve after a graceful Shutdown
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}

// healthcheckExit pings the running server's /healthz route (which checks
// the core) and returns 0/1 for the container healthcheck.
func healthcheckExit(addr string) int {
	if addr == "" {
		addr = ":8080"
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck: invalid address %q: %v\n", addr, err)
		return 1
	}
	if host == "" { // ":8080" — any interface
		host = "127.0.0.1"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + net.JoinHostPort(host, port) + "/healthz")
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck: %v\n", err)
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "healthcheck: /healthz returned %d\n", resp.StatusCode)
		return 1
	}
	return 0
}
