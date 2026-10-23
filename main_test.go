package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Teste do webserver: sobe o servidor real (porta aleatória) e faz um round-trip HTTP
func TestWebserverHelloWorld(t *testing.T) {
	srv := httptest.NewServer(newRouter())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d want %d", resp.StatusCode, http.StatusOK)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ler body: %v", err)
	}
	if got := string(body); got != "Hello, world!" {
		t.Fatalf("body: got %q want %q", got, "Hello, world!")
	}
}
