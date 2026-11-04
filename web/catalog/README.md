# web/catalog

The dashboard (the v6.3 console: the live boxes + the wall) and the workflow pages: the workflow list (from the core registry, paginated), and each workflow page (the dispatch form, the executions list, the possible-paths block diagram).

## Rules (decisions · pinned values)

- The wall: 20 names per page · `?page` default 1 · clamp [1..pages] · the info `"{start}–{end} de {total}"` (en dash · pt-BR) · prev/next `hx-get` links; an empty registry (or a not-booted core) renders the "No workflows registered" empty state. *(evidence: `dashboard.go · handleWorkflowsList` · `workflowsPage` · `templates/workflows.html`)*
- `GET /live` renders the live partial (the three boxes · the 5s poll · no-store · the fallback 0 · the box hrefs `/runnings` · `/queued` · `/failed` are the next WANT). *(evidence: `register.go` · `dashboard.go · handleLive` · `templates/live.html`)*
- The workflow page: the spec form has one input per spec item with the defaults prefilled (the parsed source — no hardcoding); the possible-paths SVG is the block diagram of `p.Graph` (rendered in this process — map lookup, no re-parse, no DB). *(evidence: `workflow.go · handleWorkflowPage` · `workflowPage`)*
- The workflow page reuses `executions.Badge`/`Row` for the execution rows — the badge/row models live in `web/executions`, so catalog does not import `core/status` in production. *(evidence: `workflow.go · workflowPage` · `executions/badge.go · Badge` · `Row`)*
- The dispatch form POSTs the raw form values (the dispatch: `web/executions` · the contract: `core/actions`). *(evidence: `workflow.go` · `executions/dispatch.go · handleRunDispatch`)*
