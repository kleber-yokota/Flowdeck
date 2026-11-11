# Coverage — the statement coverage analysis

Measured with `go test -count=1 -coverprofile=… . ./cmd/… ./core/… ./web/…`
(statement coverage, `mode: set`).

## scope

- `scripts/` — no Go files; not a measurement target.
- `workflows/` — excluded from the profile: the fixture workflows, exercised
  through the booted cores in the core and web tests.
- The test harnesses — `core/testcore` · `core/testworkflows` ·
  `web/internal/webtest` — are test-support packages, not production
  targets; their numbers are informational only.

## state

**98.2% of the production statements** (1815 of 1849).

| package | statements |
|---|---|
| `flowdeck` (main) | 90.3% |
| `cmd/workflowgen` | 97.1% |
| `core` | 100.0% |
| `core/actions` | 88.9% |
| `core/board` | 100.0% |
| `core/boot` | 100.0% |
| `core/graph` | 100.0% |
| `core/reads` | 99.2% |
| `core/registry` | 100.0% |
| `core/status` | 100.0% |
| `web` | 96.9% |
| `web/catalog` | 94.8% |
| `web/executions` | 100.0% |
| `web/executions/badge` | 100.0% |
| `web/graph` | 100.0% |
| `web/graph/block` | 100.0% |
| `web/graph/executed` | 100.0% |
| `web/render` | 90.0% |

The tests run the stack real: booted cores, real SQLite files, real
DBOS SDK calls (v1.7.0), real HTTP. The read-failure seams are a second
SQLite connection invalidating the schema (`DropTable`, `SetUnmappedState`,
`DeleteStatusRow` in `core/testcore`), never mocks.

## documented unreachable branches

The lines below are defensive branches that no real stack can reach
deterministically (a race, a build-time failure, or an invariant the
caller guarantees). They are documented, not tested.

### `flowdeck/main.go`

- **37–38** — the `--healthcheck` `os.Exit`: `os.Exit` kills the test
  binary; unreachable in-process.
- **40–42** — the fatal `run` error `os.Exit`: same.
- **119–120** — `SetWorkflowsSource` failure: the source is the embedded
  tree of this build; it always parses.
- **163–166** — the `errCh` arm of `serve`: `http.Serve` returns a real
  error only if the listener dies mid-serve; it returns
  `http.ErrServerClosed` only after `Shutdown`, and `Shutdown` runs only
  after `ctx.Done` — in which case the select returns from the `ctx.Done`
  arm. No deterministic seam.

### `cmd/workflowgen/main.go`

- **28–30** — the `main` `os.Exit` on a `run` error: same as above.
- **95–96** — `os.Getwd` failure: unreachable on a live process.

### `core/actions` — the post-success ACK branches

`cancelAck`, `dispatchACK`, and `resumeAck` read the row back after a
successful SDK call. Each branch needs the schema to change between the
SDK write and the ACK read (a race) or a mock:

- **`cancel.go:47–48`** — `ListByID` failure after `CancelWorkflow`
  succeeded: the SDK returns only once the row is `CANCELLED`.
- **`cancel.go:50–51`** — the row missing after cancel: the row the SDK
  just wrote is gone.
- **`cancel.go:54–55`** — an unmapped state after cancel: the SDK writes
  only mapped states.
- **`dispatch.go:69–70`** — `ListByID` failure after a successful
  `Enqueue`: same shape.
- **`dispatch.go:72–73`** — the row count not 1 after enqueue: the
  enqueue just wrote exactly one row.
- **`dispatch.go:76–77`** — an unmapped state after enqueue.
- **`resume.go:107–108`** — `ListByID` failure after resume/rewind.
- **`resume.go:110–111`** — the row missing after resume/rewind.
- **`resume.go:114–115`** — an unmapped state after resume/rewind.
- **`resume.go:117–118`** — the row still `ERROR` after a successful
  resume/rewind: the re-enqueued run would have to fail again before the
  ACK read (a race with the executor).

### `core/reads/detail.go`

- **106–107** — the `childrenOf` error after the parent's rows and steps
  were read: `childRows` queries the same `workflow_status` table the
  parent reads just used; failing it requires the schema to change
  mid-call.
- **133–134** — the same, at the `childRows` level.

### `web/render/render.go`

- **34, 39, 43** — the `init` branches: the embedded template tree exists
  at build time, so the read/skip failures cannot occur.

### `web/web.go`

- **30** — the `init` panic on the embedded static tree: same invariant.

### `web/catalog`

- **`failed.go:60–62`** — a malformed `CreatedAt`: `core/board` always
  formats `CreatedAt` as RFC3339.
- **`queued.go:47–49`** — same invariant.
- **`runnings.go:47–49`** — same invariant.
- **`workflow.go:78–80`** — a `Spec` error for a name `List` returned:
  membership in `List` guarantees `Spec` succeeds.
