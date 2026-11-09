# core/actions

The write side of the core: `Enqueue` (the dispatch — the run goes onto the DBOS queue), `Resume` (a failed or cancelled run re-enqueues by id), `Cancel` (a live run cancels by id).

## Rules (decisions · pinned values)

- Every dispatch and resume goes through the queue picked by workflow name (`queueNameFor`): the serial queue (`boot.SerialQueueName`, concurrency 1) for the serial dispatches (the `serial` table — today the `queue` workflow), the main queue (`boot.QueueName`) for the rest (the queues: `core/boot`). *(evidence: `dispatch.go · queueNameFor` · `resume.go · Resume`)*
- The ACK is the row read back (`reads.ListByID`), not the SDK call — dispatch, resume and cancel each read their row back before returning. *(evidence: `dispatch.go · dispatch` · `resume.go · resumeAck` · `cancel.go · cancelAck`)*
- `Enqueue` takes the clean input: the web sanitized the payload against the spec before the call — the core trusts the caller. A nil payload is the zero payload (the SDK stores `"null"` and decodes it back into the zero value at run time). *(evidence: `dispatch.go · Enqueue`)*
- The resume guard is `error | cancelled`; anything else is `ErrNotResumable`. *(evidence: `resume.go · Resume`)*
- The cancel guard is `queued | running` (a terminal run is not cancellable); anything else is `ErrNotCancellable`. *(evidence: `cancel.go · Cancel`)*
- The action errors (`ErrNotResumable` · `ErrNotCancellable`) are owned here; the web maps them to statuses in `render.Fail`. *(evidence: `resume.go` · `cancel.go` · `web/render/render.go · Fail`)*

## Facts (validated runtime)

| Fact | Evidence | Validated |
|---|---|---|
| SDK v1.7.0 · a **cancelled** run is resumed with `dbos.ResumeWorkflow`: the SDK SQL re-enqueues `CANCELLED` rows and resumes after the last completed step. | `resume.go · Resume` · SDK `SysDB.ResumeWorkflows` | 2026-11-03 |
| SDK v1.7.0 · a **failed** run is resumed with `dbos.RewindWorkflow`: the SDK deletes the step rows from the cut onward (`function_id >= cut`), keeps the same id and queue, and sets `recovery_attempts = 0`. The SDK resume is a silent no-op on `ERROR` rows (its SQL excludes `SUCCESS` and `ERROR`), which is why the two paths differ. | `resume.go · Resume` · SDK `SysDB.RewindWorkflow` | 2026-11-03 |
| The rewind cut is 0-based: the index of the first `error` row in `reads.Steps` (the rows come back `ORDER BY function_id ASC`); when no error row is found, the end of the path is the defensive default. | `resume.go · rewindCut` · SDK `SysDB.GetWorkflowSteps` | 2026-11-03 |
| `recovery_attempts` is 0-based (the SDK column), incremented by the SDK on each recovery, capped at 3 per registration (`WithMaxRecoveryAttempts(3)`); `RewindWorkflow`/`ResumeWorkflow` reset it to 0. The display is `attempts − 1` (see `core/reads` · `web/executions`). | `workflows/*/<name>.go · init` · `resume.go` · `core/reads/detail.go · Detail` | 2026-11-03 |
| The resume re-enqueues on the dispatch's queue (`queueNameFor`): a cancelled run of the `queue` workflow comes back on the serial queue, not the main queue. | `workflows/queue · TestQueueResumeStaysSerial` · SDK `SysDB.ListWorkflows` (the `QueueName` column) | 2026-11-08 |
