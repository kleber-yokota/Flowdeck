package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
)

// Router da web — isolado do boot do DBOS para poder fazer unit test
func newRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Hello, world!"))
	})
	return mux
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	appName := os.Getenv("DBOS_APP_NAME")
	dbURL := os.Getenv("DBOS_SYSTEM_DATABASE_URL")
	if appName == "" || dbURL == "" {
		return errors.New("DBOS_APP_NAME e DBOS_SYSTEM_DATABASE_URL são obrigatórias")
	}

	// SIGINT + SIGTERM (docker compose stop) → encerramento limpo
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Boot do core DBOS — conecta no Postgres 18 e roda as migrations de sistema
	dbosCtx, err := dbos.NewContext(ctx, dbos.Config{
		AppName:     appName,
		DatabaseURL: dbURL,
	})
	if err != nil {
		return fmt.Errorf("dbos boot: %w", err)
	}
	defer dbos.Shutdown(dbosCtx, 5*time.Second)

	if err := dbos.Launch(dbosCtx); err != nil {
		return fmt.Errorf("dbos launch: %w", err)
	}
	log.Println("core DBOS no ar")

	srv := &http.Server{
		Addr:              ":8080",
		Handler:           newRouter(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	log.Println("web ouvindo em :8080")

	select {
	case <-ctx.Done():
		shCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shCtx)
	case err := <-errCh:
		// ErrServerClosed é o retorno esperado do ListenAndServe após o Shutdown gracioso
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
}
