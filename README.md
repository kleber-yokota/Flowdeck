# Flowdeck

Painel de orquestração de workflows duráveis: Go + HTMX + DBOS + Postgres 18. O design está em `diagrams/` (leia o `diagrams/README.md`); as orientações de arquitetura estão em `AGENTS.md`.

## Pré-requisitos

- **Docker + docker compose** para rodar (o compose sobe o `postgres:18` sozinho);
- **Go 1.27.3** (a versão do `go.mod`) apenas para compilar/testar fora do container.

## Rodando

```bash
cp .env.example .env
docker compose up --build
```

- Webserver HTMX: porta `FLOWDECK_HTTP_PORT` no `.env` (padrão `8080`; a porta dentro do container é sempre `8080`).
- Health endpoint: `GET /healthz` (o healthcheck do container usa `flowdeck -healthcheck`, já que a imagem é distroless e não tem `curl`).

## Testes

```bash
go test ./...
```

Teste e2e (build tag `e2e`) contra um Postgres real:

```bash
DBOS_E2E_DATABASE_URL=postgres://flowdeck:flowdeck@localhost:5432/flowdeck \
  go test -tags e2e -run TestE2E ./...
```

(ou aponte `DBOS_E2E_DATABASE_URL` para qualquer Postgres 18 com um banco dedicado).
