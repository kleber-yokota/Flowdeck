# core/reads

The read side of the core: `Executions` (list the runs of a workflow), `Status` (the 2s badge poll), `Steps` (the executed path), `Detail` (the execution detail page in one read). Observes durable state — nothing is executed here.

## Rules (decisions · pinned values)

- `MapState` is the total contract: `ENQUEUED/DELAYED → queued` · `PENDING → running` · `SUCCESS → success` · `CANCELLED → cancelled` · `ERROR | MAX_RECOVERY_ATTEMPTS_EXCEEDED → error`. Unknown DBOS states are rejected, not guessed. *(evidence: `status.go · MapState`)*
- `Attempts = recovery_attempts + 1`: the SDK column is 0-based, so a run on its first attempt is 1; the display renders `attempts − 1` recovered rows (the final attempt is the badge, not a row). *(evidence: `detail.go · Detail` · `web/executions/detail.go · retriesFor`)*
- The stored input is written once at enqueue — immutable per `workflow_id`; the detail page shows the stored values, never the form values. *(evidence: `detail.go · Detail` · `inputValues`)*
- Step rows come back in `step_id` order (the branch the run took). Row kind: the wait names are checked **before** the child id — a `DBOS.getResult` row carries its child's id but is the wait, not the child. *(evidence: `steps.go · kindOf` · `stepRows`)*
- The child rows are one batched query by the child ids, issued only when the run dispatched a child. *(evidence: `steps.go · stepRows` · `childRows`)*
- `ListByID` loads no input/output — the 2s badge poll must stay cheap. *(evidence: `status.go · ListByID`)*
- `ErrUnknownExecution` is the 404 twin of a read for an id the core does not know. *(evidence: `status.go` · `web/render/render.go · Fail`)*

## Facts (validated runtime)

| Fact | Evidence | Validated |
|---|---|---|
| The wait-row names are the SDK-internal step names: `DBOS.getResult` · `DBOS.select` · `DBOS.recv` · `DBOS.sleep`. | `steps.go · kindOf` | 2026-11-03 |
