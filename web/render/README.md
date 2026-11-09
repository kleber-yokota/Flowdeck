# web/render

The rendering of the web: the embedded templates (one named template per file, rendered per request — no template cache), the two render paths (full page vs HTMX partial) and the mapping of the core's errors onto HTTP error responses.

## Rules (decisions · pinned values)

- `Page` renders a full HTML page (a normal navigation — the browser may cache it); `Partial` renders an HTMX fragment with `Cache-Control: no-store`. *(evidence: `render.go · Page` · `Partial`)*
- `Fail` is the single error→status map: `registry.ErrUnknownWorkflow` / `reads.ErrUnknownExecution` → 404 · `actions.ErrNotResumable` / `actions.ErrNotCancellable` → 400 · `boot.ErrNotLaunched` → 503 · anything else is a logged 500. *(evidence: `render.go · Fail`)*
- Rendering goes into a buffer first, so a render error can never leave a partial 200 body in flight — and is always logged. *(evidence: `render.go · render`)*
- The templates live in `templates/` (embedded, one file per named template: `index` · `workflows` · `live` · `workflow` · `execution` · `execution-detail` · `badge` · `steps` · `runnings` · `runnings-list` · `queued` · `queued-list` · `head`). *(evidence: `render.go · init` · `templates/`)*
