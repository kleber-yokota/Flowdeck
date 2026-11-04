# web/catalog

The dashboard and the workflow pages: the workflow list (from the core registry), and each workflow page (the dispatch form, the executions list, the possible-paths block diagram).

## Rules (decisions · pinned values)

- The workflow list comes from the registry (`p.List()`); an empty registry (or a not-booted core) renders the "No workflows registered" empty state. *(evidence: `dashboard.go · handleWorkflowsList` · `core/registry/registry.go · List`)*
- The workflow page: the spec form has one input per spec item with the defaults prefilled (the parsed source — no hardcoding); the possible-paths SVG is the block diagram of `p.Graph` (rendered in this process — map lookup, no re-parse, no DB). *(evidence: `workflow.go · handleWorkflowPage` · `workflowPage`)*
- The workflow page reuses `executions.Badge`/`Row` for the execution rows — the badge/row models live in `web/executions`, so catalog does not import `core/status` in production. *(evidence: `workflow.go · workflowPage` · `executions/badge.go · Badge` · `Row`)*
- The dispatch form POSTs the raw form values (the dispatch: `web/executions` · the contract: `core/actions`). *(evidence: `workflow.go` · `executions/dispatch.go · handleRunDispatch`)*
