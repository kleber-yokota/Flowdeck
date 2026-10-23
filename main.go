package main

import (
	"context"
	"errors"
	"log"
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
		log.Fatal(err)
	}
}

func run() error {
	appName := os.Getenv("DBOS_APP_NAME")
	dbURL := os.Getenv("DBOS_SYSTEM_DATABASE_URL")
	if appName == "" || dbURL == "" {
		return errors.New("DBOS_APP_NAME and DBOS_SYSTEM_DATABASE_URL are required")
	}

	// SIGINT + SIGTERM (docker compose stop) → clean shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Boot the DBOS core (process singleton) — Postgres 18
	if err := core.Boot(ctx, appName, dbURL); err != nil {
		return err
	}
	defer core.Shutdown(5 * time.Second)

	// The core registers its workflows and starts the executor
	if err := core.Launch(); err != nil {
		return err
	}
	log.Println("core DBOS is up")

	// Web: an abstraction over the core — lists what the core registered
	srv := &http.Server{
		Addr:              ":8080",
		Handler:           web.New(core.List),
		ReadHeaderTimeout: 5 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	log.Println("web listening on :8080")

	select {
	case <-ctx.Done():
		shCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shCtx)
	case err := <-errCh:
		// ErrServerClosed is the expected return from ListenAndServe after a graceful Shutdown
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
