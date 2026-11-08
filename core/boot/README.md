# core/boot

The DBOS root of the core: the process-wide singleton that owns the DBOS context, the single queue, the paths graph and the lifecycle (`Launch` · `Shutdown` · `Ready` · `Ctx`).

## Rules (decisions · pinned values)

- Explicit boot: no package `init`; `Boot(ctx, appName, databaseURL)` is exported and called by `main.go` and the test harnesses (the env vars are read at the call site); the error surfaces before `Launch`. *(evidence: `lifecycle.go · Boot` · `main.go`)*
- The single queue: `QueueName = "flowdeck"`. `Launch` registers the queue before `dbos.Launch` — registering it is what makes this process listen to it. *(evidence: `lifecycle.go · Launch` · `QueueName`)*
- One executor per process: a second `Launch` without a `Shutdown` in between returns an error. *(evidence: `lifecycle.go · Launch`)*
- `Ready` is the health contact (a different contact than the Port: lifecycle state, not a domain conversation) — used by the web's `/healthz` and the container healthcheck. `CtxOrErr` returns `ErrNotLaunched` and is the read guard of every query in the core. *(evidence: `lifecycle.go · Ready` · `CtxOrErr` · `web/health.go`)*
- `SetWorkflowsSource` builds the possible-paths graph from the workflow source tree (the embedded tree in production, `os.DirFS` in the tests) — called once, before `Launch` (the graph: `core/graph`). *(evidence: `lifecycle.go · SetWorkflowsSource` · `Paths`)*
- `Shutdown` nils the context before `dbos.Shutdown` runs (and outside the lock), so `List()` during shutdown returns nil instead of blocking. *(evidence: `lifecycle.go · Shutdown`)*
- `warnUnregistered` compares the source-built graph with the DBOS registry after launch: a name the source declares but the registry lacks means `workflows/zz_generated.go` is stale (the subpackage's registration never ran). *(evidence: `lifecycle.go · warnUnregistered`)*

## Facts (validated runtime)

| Fact | Evidence | Validated |
|---|---|---|
| The boot→`Launch`→`Shutdown` cycle is repeatable in one process: a fresh `dbos.DBOS` per `Boot`. | the spike `TestReBootSpike` (deleted 2026-11-06) · `TestBootTestBootsAndCleans` (the re-verification · dbos v1.7.0) | 2026-11-07 |
| `dbos.RegisterQueue` is repeatable: the second `Boot` re-registers the same queue name (the `frozen` queue) without error. | the repeatable-cycle test · the `frozen` queue re-registered | 2026-11-07 |
| `warnUnregistered` returns early when the source is nil (no embedded FS) — the warning path is silent without the source. | `warnUnregistered` · `main.go` (the embedded FS is non-nil) | 2026-11-07 |
