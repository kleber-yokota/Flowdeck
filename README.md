# Flowdeck

Painel de orquestração de workflows duráveis: Go + HTMX + DBOS + Postgres 18. O design está em `diagrams/` (leia o `diagrams/README.md`); as orientações de arquitetura estão em `code.md`.

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
compose). As suítes são self-booting: cada teste booted boota e desliga o
seu próprio core (um arquivo fresco por teste) — sem env, nada skipa, sem
`-p 1` (a definição das suítes: `diagrams/package-map.html#suites`).

```bash
# o job `unit` do CI
go test -count=1 ./...

# booted (race, count=2, shuffled — o job `race` do CI)
go test -race -count=2 -shuffle=on ./...
```
