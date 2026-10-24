# AGENTS.md — FlowDeck

Guidelines for code agents working in this repository.

## The project

FlowDeck is a durable workflow orchestration dashboard. **A single Go service**: the web layer (HTMX, `:8080`) boots the DBOS core and exposes durable workflows on Postgres 18 (`:5432`). All design (architecture, states, sequences) is documented in `diagrams/` — read `diagrams/README.md` and the `.html` files before implementing.

## Stack

| Layer | Technology | Notes |
| --- | --- | --- |
| Backend + Web | **Go** (single service) | webserver on `:8080`, serves HTMX templates |
| Frontend | **HTMX** (in the browser, no SPA/JS framework) | pages are server-side HTML + `hx-*`; interactions are HTMX GET/POST |
| Orchestration | **DBOS Go** — `github.com/dbos-inc/dbos-transact-golang` | workflows, queues and durable state on Postgres; API: `dbos.NewContext(ctx, dbos.Config{...})`, `RegisterQueue`, `Enqueue`/`WithQueue` |
| Database | **Postgres 18** (`:5432`) | via `DBOS_SYSTEM_DATABASE_URL`; DBOS requires a dedicated system database |
| Deploy | **docker-compose**, one machine | FlowDeck service + Postgres 18; no other containers |

## Architecture invariants (do not violate)

1. **One process, two worlds**: the web is an *abstraction over the DBOS core boot* — the core starts inside the Go service and the web calls into it. DBOS **does not run any webserver**; if you see DBOS exposing ports/routes, it is wrong.
2. **Zero workflow hardcoding in the web**: the workflow list comes from the **core registry**. The web lists what the core registered; adding a new workflow = registering it in the core, never adding a fixed item to the template/HTML.
3. **Queue in DBOS**: dispatch = enqueue (`POST` HTMX → web → core → `Enqueue` on Postgres). The DBOS executor picks the job off the queue, runs it and updates the status — the webserver never executes a workflow directly.
4. **Status by polling**: the front polls the status every **2s** (HTMX polling, `hx-trigger="every 2s"` or similar); no websockets/SSE for status.

## Workflow state machine

```
queued → running (retry ≤ 3) → success | failed
```

- Mapped DBOS states: `enqueued` → queued, `running` → running, `success` → success, `error` → failed (retries exhausted).
- **Terminal** states (success/failed) show a dot in the UI.

## Status colors (same as the UI badges — do not invent others)

| Status | Color | Hex |
| --- | --- | --- |
| queued | light blue | `#7cc0ff` |
| running | orange | `#ff9142` |
| success | green | `#46d19a` |
| failed | red | `#ff5450` |

Visual theme: dark metallic background `#0b0c0e`, steel borders (`#b8bcc4`/`#8f96a3`), text `#e8e6e1`, orange accent `#ff9142` (the diagrams palette).

## Flows (reference: `diagrams/`)

- **Phase 1 — dynamic listing**: HTMX `GET` → core lists the registry workflows → UI renders the list.
- **Phase 2 — dispatch and track**: HTMX `POST` → enqueue on Postgres (ACK) → DBOS runs the job and updates the status → the 2s polling updates the badge through the four colors.
- **Phase 3 — runs of a workflow**: `GET /workflows/{name}` lists all executions (workflow_id + badge); clicking the workflow_id → detail: status, retries, status of each retry — on failure: error + the failed step.

## Implementation rule (always apply)

Implement what `diagrams/` says, **simply and directly**: the diagram is the spec and the source of truth — build exactly what it shows, with the smallest reasonable set of pieces. No abstractions, layers, options or dependencies beyond what the diagram shows. If a solution is harder to read than the diagram, it is wrong — simplify it.

## Conventions

- Code and commit language: **English**; comments/docstrings may be pt-BR.
- HTMX routes return **HTML fragments** (partials), not JSON.
- Do not introduce a frontend framework, bundler, npm, or a database other than Postgres without an explicit decision.
- Postgres is always **18** in the compose (DBOS compatibility).
- The diagrams in `diagrams/` are the source of truth for design: if the code diverges from a diagram, the diagram wins — update the diagram together with the code.

## Commands

```bash
go test ./...
docker compose up --build
```

Service structure: one compose file (`docker-compose.yml`) with `postgres:18` (port `5432`) and the FlowDeck service (`:8080`). A single `main.go` does the boot (DBOS core → HTMX webserver); the code lives in two packages: `core/` (the DBOS core singleton: `Boot`/`Launch`/`Shutdown`/`List` in `core.go`, steps in `steps.go`, workflows in `workflows.go` — each file registers itself in its own `init()`, no central list) and `web/` (HTMX webserver + templates).
