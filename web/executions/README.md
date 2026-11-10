# web/executions

The execution routes: the dispatch, the detail, the steps (the executed-path poll), the resume, the cancel. The badge (the 2s poll fragment) and the badge/row models it renders live in the `badge` subpackage.

## Rules (decisions · pinned values)

- The six routes are registered in one place (`Register`): `POST /run/{name}` · `GET /status/{id}` · `GET /executions/{id}` · `GET /executions/{id}/steps` · `POST /resume/{id}` · `POST /cancel/{id}`. *(evidence: `register.go · Register`)*
- The 2s poll is HTMX-side: the badge and steps fragments carry `hx-trigger="every 2s"` while the run is live; a terminal badge is static — the poll stops. *(evidence: `templates/badge.html` · `templates/steps.html` · `badge/badge.go · HandleBadge` · `steps.go · handleExecutionSteps`)*
- The dispatch: form values → `spec.Sanitize` (400 on values that do not sanitize) → `p.Enqueue` → the ACK execrow fragment (the browser prepends it; the badge starts the 2s poll). *(evidence: `dispatch.go · handleRunDispatch` · `formValues`)*
- The attempts display is `attempts − 1` recovered rows (`retriesFor` — the definition: `core/reads`). *(evidence: `detail.go · retriesFor` · `executionDetail`)*
- The cancel ACK (the cancelled badge, terminal) replaces the badge. The resume ACK is the row read back (the same id, enqueued again). *(evidence: `cancel.go · handleCancel` · `resume.go · handleResume`)*
