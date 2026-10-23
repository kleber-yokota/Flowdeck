# Flowdeck

Painel de orquestração de workflows duráveis: Go + HTMX + DBOS + Postgres 18. O design está em `diagrams/` (leia o `diagrams/README.md`); as orientações de arquitetura estão em `AGENTS.md`.

## Rodando

```bash
cp .env.example .env
docker compose up --build
```

O webserver HTMX fica em `:8080` (porta publicada no host definida por `FLOWDECK_HTTP_PORT` no `.env`).

## Testes

```bash
go test ./...
```
