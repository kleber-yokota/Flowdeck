# web/internal/webtest

The shared HTTP helpers of the web handler tests. The web is only tested
against the booted core (the env-driven context, the `testworkflows`
fixtures) or the real not-launched port (`core.NewPort()` — the 503 and
empty-state paths). There is no fake `Port`.

## Rules (decisions · pinned values)

- No fake `Port`, no canned rows: every handler test uses a real
  `core.Port` — booted (the deterministic fixtures) or not-launched (the
  503 / empty paths). *(evidence: `webtest.go` — no `TestPort`)*
- `Dispatch` posts a dispatch (form `nil` for the no-input dispatch) and
  returns the ACK row fragment (the 200 response).
- `Ack` is `Dispatch` + the queued-badge assertion — for the runs that
  stay live (the deterministic fixtures); a run that settles fast (e.g.
  `failing-test`) is dispatched with `Dispatch` and the terminal state is
  read with `Detail`.
- `Detail` polls `GET /executions/{id}` until the page contains the
  fragment (30s deadline).
- `ExecutionID` extracts the workflow ID from an execution row (the
  dispatch ACK).
- The HTTP helpers (`Get` · `Post` · `PostForm` · `ReadAll` · `ContainsAll`
  · `ContainsAny`) hit the test server; the handler tests use them to
  assert fragments and statuses. *(evidence: `webtest.go`)*
