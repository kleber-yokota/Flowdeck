# core/boot

The DBOS root of the core: the process-wide singleton that owns the DBOS context, the single queue, the paths graph and the lifecycle (`Launch` · `Shutdown` · `Ready` · `Ctx`).

## Rules (decisions · pinned values)

- Env-driven init: the package `init` boots the DBOS context from `DBOS_APP_NAME` + `DBOS_SYSTEM_DATABASE_URL` (docker compose sets them, so the context exists before `main` runs and the workflow files register in their own `init`). Without env (unit tests, tools) the context stays nil until an explicit boot. *(evidence: `lifecycle.go · init` · `boot`)*
- The single queue: `QueueName = "flowdeck"`. `Launch` registers the queue before `dbos.Launch` — registering it is what makes this process listen to it. *(evidence: `lifecycle.go · Launch` · `QueueName`)*
- One executor per process: a second `Launch` without a `Shutdown` in between returns an error. *(evidence: `lifecycle.go · Launch`)*
- `Ready` is the health contact (a different contact than the Port: lifecycle state, not a domain conversation) — used by the web's `/healthz` and the container healthcheck. `CtxOrErr` returns `ErrNotLaunched` and is the read guard of every query in the core. *(evidence: `lifecycle.go · Ready` · `CtxOrErr` · `web/health.go`)*
- `SetWorkflowsSource` builds the possible-paths graph from the workflow source tree (the embedded tree in production, `os.DirFS` in the tests) — called once, before `Launch` (the graph: `core/graph`). *(evidence: `lifecycle.go · SetWorkflowsSource` · `Paths`)*
- `Shutdown` nils the context before `dbos.Shutdown` runs (and outside the lock), so `List()` during shutdown returns nil instead of blocking. *(evidence: `lifecycle.go · Shutdown`)*
- `warnUnregistered` compares the source-built graph with the DBOS registry after launch: a name the source declares but the registry lacks means `workflows/zz_generated.go` is stale (the subpackage's registration never ran). *(evidence: `lifecycle.go · warnUnregistered`)*
