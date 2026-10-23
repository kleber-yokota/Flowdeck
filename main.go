package main

import (
	"context"
	"log"
	"net/http"
	"os"
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Boot do core DBOS — conecta no Postgres 18 e roda as migrations de sistema
	dbosCtx, err := dbos.NewContext(ctx, dbos.Config{
		AppName:     os.Getenv("DBOS_APP_NAME"),
		DatabaseURL: os.Getenv("DBOS_SYSTEM_DATABASE_URL"),
	})
	if err != nil {
		log.Fatalf("dbos: boot: %v", err)
	}
	defer dbos.Shutdown(dbosCtx, 5*time.Second)

	if err := dbos.Launch(dbosCtx); err != nil {
		log.Fatalf("dbos: launch: %v", err)
	}
	log.Println("core DBOS no ar")

	log.Println("web ouvindo em :8080")
	log.Fatal(http.ListenAndServe(":8080", newRouter()))
}
