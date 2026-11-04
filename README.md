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

Todos os testes com banco rodam em SQLite (o Postgres é só a produção do
compose): unitários sem env, booted via `.env.test`, e o e2e (build tag
`e2e`) — o CI roda os dois em SQLite.

```bash
# booted (race, count=2, shuffled — o job `test` do CI)
set -a && source .env.test && set +a
go test -race -count=2 -shuffle=on -p 1 ./...

# e2e (o job `e2e` do CI)
DBOS_APP_NAME=flowdeck_e2e \
DBOS_SYSTEM_DATABASE_URL=sqlite:/tmp/flowdeck-e2e.db \
  go test -tags e2e -run TestE2E -p 1 ./...
```

(`-p 1`: um test binário por vez — os binários compartilham o arquivo
SQLite e um único executor poll a fila.)
