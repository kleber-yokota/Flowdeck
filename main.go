package main

import (
	"context"
	"errors"
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
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

// config is the process configuration, read from the environment.
type config struct {
	appName     string
	dbURL       string
	httpAddr    string
	httpTimeout time.Duration
	coreTimeout time.Duration
}

func loadConfig() (config, error) {
	appName := os.Getenv("DBOS_APP_NAME")
	dbURL := os.Getenv("DBOS_SYSTEM_DATABASE_URL")
	if appName == "" || dbURL == "" {
		return config{}, errors.New("DBOS_APP_NAME and DBOS_SYSTEM_DATABASE_URL are required")
	}
	return config{
		appName:     appName,
		dbURL:       dbURL,
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

	// The core lifecycle is controlled by core.Shutdown, not the signal:
	// DBOS derives its internal context from the one passed to Boot, so a
	// signal-cancelable ctx would cancel the core before the ordered
	// Shutdown below runs.
	if err := core.Boot(context.WithoutCancel(ctx), cfg.appName, cfg.dbURL); err != nil {
		return err
	}
	defer core.Shutdown(cfg.coreTimeout)

	// The core registers its workflows and starts the executor
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
	srv := &http.Server{
		Addr:              cfg.httpAddr,
		Handler:           web.New(core.List),
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
