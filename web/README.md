# web

The HTMX presentation layer of FlowDeck: the router (the catalog routes and the execution routes each register themselves in their subpackage), the healthz, the static assets and the middlewares. Talks to the core through `core.Port` — it never touches DBOS/Postgres (forbidden import, enforced by the gate).

## Rules (decisions · pinned values)

- The router takes exactly `(core.Port, ready func() error)` — never the core internals. The readiness check is injected (production wires `boot.Ready` from `main`) so the 503 guard is testable. *(evidence: `web.go · New` · `main.go · run`)*
- The routes are registered by the subpackages: `catalog.Register(mux, p)` · `executions.Register(mux, p)` — the router itself owns only `/healthz` and `/static/`. *(evidence: `web.go · New` · `catalog/register.go` · `executions/register.go`)*
- The healthz mirrors the core: 200 when ready, 503 when it is not (the container healthcheck uses it). *(evidence: `health.go · handleHealthz`)*
- Middlewares: `withRecovery` turns a handler panic into a logged 500; `withRequestLog` logs method · path · status · duration. *(evidence: `middleware.go`)*
- The static assets (vendored HTMX + stylesheet) are embedded and served as-is — no CDN at runtime. The templates live in `web/render` (their own embed). *(evidence: `web.go · staticFS` · `render/render.go · init`)*
