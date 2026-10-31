# AGENTS.md — FlowDeck

Guidelines for code agents working in this repository.

## The project

FlowDeck is a durable workflow orchestration dashboard. **A single Go service**: the web layer (HTMX, `:8080`) boots the DBOS core and exposes durable workflows on Postgres 18 (`:5432`). All design (architecture, states, sequences) is documented in `diagrams/` — read `diagrams/README.md` before implementing; open the `.html` files as needed for your task (see its reading rule).

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
2. **Zero workflow hardcoding in the web**: the workflow list comes from the **core registry**. The web lists what the core registered; adding a new workflow = adding a file to `workflows/` (it registers itself on the core's DBOS context in its `init()`; a subpackage additionally needs `go generate ./workflows`), never adding a fixed item to the template/HTML.
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
- **Phase 3 — runs of a workflow**: `GET /workflows/{name}` lists all executions (workflow_id + badge); clicking the workflow_id → detail: status, retries, the stored input values, status of each retry — on failure: error + the failed step.

## Code map (where to read before editing)

- Package map (files, functions, contacts) + task→files routing: the sections of `diagrams/package-map.html` — start with `#tasks` (where to start per task) and `#contacts` (where the modules touch: per edge, file/function + identifiers), then the package sections (`#package-main`, `#package-core`, `#package-workflows`, `#package-gen`, `#package-status`, `#package-graph`, `#package-web`, `#package-webgraph`, `#package-dbos`), each a table: file | responsibility | functions. The SVG in the same file is the human view of the same data. It is the single source for the package/file layout — do not restate it here.

## Implementation rule (always apply)

Implement what `diagrams/` says, **simply and directly**: the diagram is the spec and the source of truth — build exactly what it shows, with the smallest reasonable set of pieces. No abstractions, layers, options or dependencies beyond what the diagram shows. If a solution is harder to read than the diagram, it is wrong — simplify it.

## Conventions

- Code and commit language: **English**; comments/docstrings may be pt-BR.
- HTMX routes return **HTML fragments** (partials), not JSON.
- Do not introduce a frontend framework, bundler, npm, or a database other than Postgres without an explicit decision.
- Postgres is always **18** in the compose (DBOS compatibility).
- The diagrams in `diagrams/` are the source of truth for design: if the code diverges from a diagram, the diagram wins — update the diagram together with the code.
- `docs/` is **not** a source of truth. Do not read `docs/` unless explicitly asked.

## Commands

```bash
# unit + integration (SQLite — the core reads the env in its init, before the process starts)
set -a && source .env.test && set +a
go test -race -count=2 -shuffle=on -p 1 ./...

# e2e (needs Postgres 18; CI runs it against a postgres:18 service)
# Local: the flowdeck_e2e database is isolated from the production smoke — queue names
# are unique per app across a whole system database, so a mixed app fails with ConflictingRegistration.
DBOS_APP_NAME=flowdeck_e2e \
DBOS_SYSTEM_DATABASE_URL="postgres://flowdeck:flowdeck@localhost:5432/flowdeck_e2e?sslmode=disable" \
go test -tags e2e -run TestE2E -p 1 ./...

# new workflow subpackage (workflows/<name>/): regenerate the blank imports
# (writes workflows/zz_generated.go — main.go never changes)
go generate ./workflows

docker compose up --build
```

- The core brings up the DBOS context from `DBOS_APP_NAME` + `DBOS_SYSTEM_DATABASE_URL` in its own `init()`; without them `core.Ctx()` is nil and the workflow files skip registration (tools that only need the graph parser still work).
- `-p 1`: one test binary at a time — the binaries share the SQLite test database (`-p 1` in e2e: only one executor may poll the queue).

Service structure: one compose file (`docker-compose.yml`) with `postgres:18` (port `5432`) and the FlowDeck service (`:8080`). A single `main.go` does the boot (env → core `init` (DBOS context) → `SetWorkflowsSource` (embedded `workflows/` tree) → `Launch` → HTMX webserver); the workflow packages self-register in their `init()` on the core context (the subpackages are pulled in by the generated `workflows/zz_generated.go`, written by `cmd/workflowgen` via `go generate ./workflows` — also run in the Dockerfile and CI); `core.Launch` warns at boot when the embedded source declares a workflow the registry lacks (stale generated file); the package/file layout lives in the Code map above (single source).

## Code analysis (structure, coupling, complexity)

Tool suite used to find refactoring targets — function complexity, package coupling and function-level call relationships. Binaries install into the Go bin:

| Tool | Install | What it shows | Typical run |
| --- | --- | --- | --- |
| gocyclo | `go install github.com/fzipp/gocyclo/cmd/gocyclo@latest` | cyclomatic complexity per function | `gocyclo -top 20 . \| grep -v '_test.go'` |
| gocognit | `go install github.com/uudashr/gocognit/cmd/gocognit@latest` | cognitive complexity per function | `gocognit -top 20 . \| grep -v '_test.go'` |
| goimportmaps | `go install github.com/mickamy/goimportmaps/cmd/goimportmaps@latest` | package import graph + coupling metrics (Ca/Ce/I) | `goimportmaps graph --metrics` |
| gopkgview | `go install github.com/grishy/gopkgview@latest` | visual browser for the package dependency graph (coarse view) | `gopkgview --skip-browser` |
| callgraph (x/tools) | `go install golang.org/x/tools/cmd/callgraph@latest` | function-level call graph, static + RTA (resolves interface dispatch) | `callgraph -algo rta ./... > /tmp/callgraph.txt` |
| go-callvis | `go install github.com/ofabry/go-callvis@latest` | function-level call graph as SVG + interactive server | `go-callvis -nostd -skipbrowser -file /tmp/graph.svg -group pkg -focus flowdeck/core ./...` |

How to read them:
- The function-level tools (`callgraph`, `go-callvis`) are the primary lens — the package view alone misses coupling that lives inside a package.
- RTA over-approximates calls through function values (`core.Port` fields, `ready func() error`): count **static** edges for coupling numbers; read dynamic edges as "possible".
- `go-callvis` published as v0.7.1 pins `golang.org/x/tools v0.28.0`, whose SSA builder panics on the Go 1.27 stdlib (`unexpected expr: *ast.KeyValueExpr`) — rebuild from source with `golang.org/x/tools@v0.52.0`.

Baseline (current): max cyclomatic complexity of any production function is **9** (`main.run`, `core.Detail`); max cognitive complexity **10** (the `(*graphLayout).measure` of `web/graph`); **zero** static `web → dbos` call edges (the 53 static `flowdeck → dbos` edges live in `core` (25, runtime), `workflows` (23) and `workflows/buy` (5, the `init` registrations); `cmd/workflowgen` imports no flowdeck package); fan-in hotspots are `web/graph.num` (10), `workflows.EchoStep` (7), `core.init` (6), `core.ctxOrErr` (5), `web.fail` (5), `core.Ctx` (4), `core.mapState` (4). Re-run the metrics after refactors and keep the baseline — or beat it.

CI gates the baseline: the `ci` workflow runs `gocyclo -over 9` and `gocognit -over 10` over non-test files and fails when any production function is above the baseline; the test job sources `.env.test` and runs the suite with `-race -count=2 -shuffle=on -p 1`, and the e2e job runs `go test -tags e2e -run TestE2E -p 1 ./...` against a `postgres:18` service with `DBOS_APP_NAME`/`DBOS_SYSTEM_DATABASE_URL`. If a change genuinely needs more, raise the threshold in `.github/workflows/ci.yml` and update this baseline in the same commit.
