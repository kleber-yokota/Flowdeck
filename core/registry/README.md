# core/registry

The registration read of the core: `List` (which workflows are registered — the web listing's source) and `Spec` (the parameter spec the dispatch form renders and sanitizes against).

## Rules (decisions · pinned values)

- `List` reads `dbos.ListRegisteredWorkflows` — the single source of truth for the web listing, no hardcoding. It returns nil when the core is not booted (or is shutting down) — the web renders the empty state (`web/catalog`). The list is sorted by name. *(evidence: `registry.go · List`)*
- `Spec`/`SpecOf` read the parsed source (`boot.Paths().Specs`) — the spec comes from the workflow source, not from the runtime registry. `SpecOf` is false before `SetWorkflowsSource` and for unknown names. *(evidence: `spec.go · Spec` · `SpecOf`)*
- `ErrUnknownWorkflow` is shared by `Enqueue` (not registered in the DBOS) and by `Spec` (not in the parsed source) — one error for both unknown-workflow paths. *(evidence: `registry.go` · `actions/dispatch.go · Enqueue`)*
