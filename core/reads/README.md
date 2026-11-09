# core/reads

The read side of the core: `Executions` (list the runs of a workflow), `Status` (the 2s badge poll), `Steps` (the executed path), `Detail` (the execution detail page in one read), `Live` (the three live boxes). Observes durable state — nothing is executed here.

## Rules (decisions · pinned values)

- `MapState` is the total contract: `ENQUEUED/DELAYED → queued` · `PENDING → running` · `SUCCESS → success` · `CANCELLED → cancelled` · `ERROR | MAX_RECOVERY_ATTEMPTS_EXCEEDED → error`. Unknown DBOS states are rejected, not guessed. *(evidence: `status.go · MapState`)*
- The display attempts are the dispatches so far, minimum 1: the SDK column counts the current claim (0 until the first claim — a resume re-enqueues and starts the count again), so `displayAttempts` floors it at 1; the display renders `attempts − 1` recovered rows (the final attempt is the badge, not a row). *(evidence: `detail.go · displayAttempts` · `web/executions/detail.go · retriesFor`)*
- The stored input is written once at enqueue — immutable per `workflow_id`; the detail page shows the stored values, never the form values. *(evidence: `detail.go · Detail` · `inputValues`)*
- Step rows come back in `step_id` order (the branch the run took). Row kind: the wait names are checked **before** the child id — a `DBOS.getResult` row carries its child's id but is the wait, not the child. *(evidence: `steps.go · kindOf` · `stepRows`)*
- The child rows are one batched query by the child ids, issued only when the run dispatched a child. *(evidence: `steps.go · stepRows` · `childRows`)*
- `ListByID` loads no input/output — the 2s badge poll must stay cheap. *(evidence: `status.go · ListByID`)*
- `ErrUnknownExecution` is the 404 twin of a read for an id the core does not know. *(evidence: `status.go` · `web/render/render.go · Fail`)*
- The live read: the running box counts the runs running right now and the queued box the runs in the queue right now (firing the same workflow several times counts each run — no deduplication by workflow); the failed box counts the workflows whose latest run is error ∪ cancelled · a latest success with no live run in no box · one `ListWorkflows` (`WithFilterSortDesc` · `LoadInput(false)` · `LoadOutput(false)`). *(evidence: `live.go · LiveCounts`)*
- The running-runs read: `Living` returns one row per running run (`LiveRun`: `Name` · `ID` · `State` · `CreatedAt` RFC3339) — no dedup by workflow name (two fires of the same workflow = two rows) · the SDK's desc order (newest first · no re-sort) · `State` is always `running` (the list filters to it) · not up → `boot.ErrNotLaunched` (the guard, like every other read). *(evidence: `living.go · LiveRun` · `Living`)*
- The queued-runs read: `Queued` returns one row per queued run (`LiveRun`: `Name` · `ID` · `State` · `CreatedAt` RFC3339) — `State` is always `queued` (the list filters to it: `ENQUEUED` ∪ `DELAYED`) · no dedup by workflow name · the SDK's desc order · not up → `boot.ErrNotLaunched` (the guard, like every other read). *(evidence: `queued.go · Queued` · `queued_test.go`)*

## Facts (validated runtime)

| Fact | Evidence | Validated |
|---|---|---|
| The wait-row names are the SDK-internal step names: `DBOS.getResult` · `DBOS.select` · `DBOS.recv` · `DBOS.sleep`. | `steps.go · kindOf` | 2026-11-03 |
| `ListWorkflows` rows carry the workflow `Name` — grouping by `Name` yields one row per workflow, first row = the latest run. | go.mod (SDK v1.7.0) · `dbos/internal/models/workflow_status.go` (`WorkflowStatus`) · the green `TestLiveRunningRunCountedUntilCancelled` · `TestLiveQueuedRunCountedUntilCancelled` | author · 2026-11-04 · SDK v1.7.0 |
| A run enqueued on a queue registered with `WithWorkerConcurrency(0)` stays ENQUEUED (no executor claims it) — the frozen-queue seam for the queued bucket. | `core/actions/cancel_test.go` · `TestCancelQueuedRun` (green) · the SDK `WithWorkerConcurrency` | author · 2026-11-04 |
| The live tests own `live-test` and end with its latest run in SUCCESS (start state = end state — the live-test contribution is zero), so the per-type counts hold under `-count=2 -shuffle=on`. | `live_test.go` | author · 2026-11-04 |
| The no-dedup contract: the rows are runs, not workflows — the same workflow running twice is two rows (no dedup by workflow name); the strict count is sound: the main queue is registered with no worker limit (`RegisterQueue` without `WithWorkerConcurrency`), so the executor claims every fire at once | `living.go · Living` · `living_test.go · TestLivingNoDedupByWorkflow` · `core/boot/lifecycle.go · Launch` | 2026-11-04 · env suite green |
