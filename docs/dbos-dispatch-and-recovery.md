# DBOS dispatch and recovery

## 1. Purpose

This document records the verified behaviors of the DBOS Go runtime.
The project uses these facts for dispatch and recovery decisions.

## 2. Source

The facts come from the module `github.com/dbos-inc/dbos-transact-golang` v1.7.0.
The workflow and step state lives in the DBOS system database (Postgres).
The table lists the investigated files.

| File | Content |
|---|---|
| `dbos/workflow.go` | `RunWorkflow`, `Enqueue`, `Go`, `Select` |
| `dbos/queue.go` | the queue runner |
| `dbos/recovery.go` | the restart recovery |
| `dbos/internal/sysdb/system_database.go` | the recovery SQL |
| `dbos/internal/models/workflow_status.go` | the workflow statuses |

## 3. Defined terms

| Term | Definition |
|---|---|
| checkpoint | The committed output of a step, stored in `operation_outputs`. |
| child workflow | A workflow that the parent starts with `RunWorkflow` without a queue. |
| enqueued workflow | A workflow that the runtime inserts into a queue with `Enqueue`. |
| executor | A DBOS process instance, identified by an executor ID. |
| queue runner | The component that polls a queue and executes enqueued workflows. |
| replay | The execution of a workflow that re-reads existing checkpoints. |
| step | A durable unit of work with a deterministic step ID. |

## 4. Dispatch

- `RunWorkflow` without a queue starts a child workflow.
- `Enqueue` starts an enqueued workflow.
- `RunWorkflow` with `WithQueue` uses the `Enqueue` path.
- The queue is the distinguishing axis, not the function name.

## 5. Child workflows

- The parent executor inserts the child row with status `PENDING`.
- The child ID is deterministic: `parentID-stepID`.
- The runtime records the parent-child pair.
- The parent executor runs the child in the same process.
- The parent waits with `GetResult`.
- A step cannot start a child workflow.

## 6. Enqueued workflows

- The runtime inserts the child row with status `ENQUEUED`.
- A delayed enqueue inserts the row with status `DELAYED`.
- The row stores the parent ID in `parent_workflow_id`.
- The queue runner executes the child, not the parent.
- Within a workflow, the runtime checkpoints the call as the step `DBOS.enqueue`.
- The checkpoint stores the child ID as the step output.
- The parent receives a polling handle.
- A step cannot call `Enqueue`.

## 7. Parent failure

### 7.1 Child workflows

- If the parent is `PENDING` when the process dies, then the restart recovery re-runs the parent.
- The replay re-reads each child step from `operation_outputs`.
- The replay re-derives the same child ID.
- The replay does not create a second child.
- If the child is `PENDING` when the process dies, then the recovery re-runs the child.
- The child replay skips the completed steps and runs the first missing step.

### 7.2 Enqueued workflows

- If the parent dies, then the enqueued child remains in Postgres.
- The queue runner executes the child on the next poll.
- The parent death does not cancel the child.
- Only `CancelWorkflow` cancels the child.
- If the process dies, then the queue runner re-polls after the restart.
- The child executes once.
- The parent replay returns the recorded child ID from the `DBOS.enqueue` step.
- The replay does not enqueue a second child.

### 7.3 Async steps (`Go`)

- `Go` runs a step in a goroutine.
- `Go` returns a channel to the caller.
- The step has a deterministic step ID.
- The step output commits to `operation_outputs`.
- If the parent dies before the commit, then the replay re-runs the step.
- If the parent dies after the commit, then the replay returns the stored output.
- The goroutine dies with the process.
- The async step is a part of the parent, not a separate workflow.
- `Select` joins channels with a durable checkpoint.

## 8. Recovery (restart)

- The recovery runs at `Launch` for the current executor ID.
- The default executor ID is `local`.
- The ID is stable across restarts unless `DBOS__VMID` overrides it.
- The recovery sets the `PENDING` rows to `ENQUEUED` on the internal recovery queue.
- The replay re-reads the completed steps.
- The replay re-runs the first missing step.
- Each claim increments `recovery_attempts`.
- If the attempts exceed the limit, then the status becomes `MAX_RECOVERY_ATTEMPTS_EXCEEDED`.

### 8.1 Resume (`ResumeWorkflow`)

- The call re-enqueues the workflow on the internal queue.
- With `WithResumeQueue`, the call targets a named queue instead.
- The call resets `recovery_attempts` to 0.
- The call clears the owner, the start time, the completion time, the deadline, and the deduplication ID.
- The call does not touch the checkpoints.
- The workflow resumes from its last completed step.
- The call is a no-op for `SUCCESS` and `ERROR` workflows.
- The call resumes `CANCELLED` and `MAX_RECOVERY_ATTEMPTS_EXCEEDED` workflows.
- The call starts a `DELAYED` workflow immediately.
- Within a workflow, the runtime checkpoints the call as the step `DBOS.resumeWorkflow`.

## 9. Comparison

| Mode | Executor | Survives parent death | Re-runs on recovery |
|---|---|---|---|
| Child workflow (`RunWorkflow`) | the parent executor | yes, through the replay | yes, if `PENDING` |
| Enqueued workflow (`Enqueue`) | the queue runner | yes, the row remains in Postgres | no, the child executes once |
| Async step (`Go`) | the parent, in a goroutine | only the committed output | yes, if uncommitted |

## 10. Implications for FlowDeck

- FlowDeck dispatches with `Enqueue` on a single queue.
- The dispatch is durable because the row is in Postgres.
- The web request can end before the workflow runs.
- The front polls the status every 2 s.
- A re-dispatch from the web creates a new workflow ID.
- A re-dispatch does not resume the old execution.
- The old execution continues through the queue, or recovers if `PENDING`.

## 11. Open items

- The diagram `paths-blocks.html` does not mark `RunWorkflow` with `WithQueue` as off-path.
