# core/testworkflows

The fixtures of the test suite: the ten test-only workflows and the frozen queues (`frozen` · `live-frozen`, the queued-state fixtures), registered on the booted DBOS context (the per-test cores of the `core` and `web` suites, and the env context of the two `e2e` suites), and the parser's source — the suites point the workflows source at this package directory. Never part of the `workflows` package, so they do not appear in the production registry.

## Rules (decisions · pinned values)

- The ten fixtures, all with `WithMaxRecoveryAttempts(3)`: `failing-test` · `resume-test` · `live-test` · `noinput-test` · `block-test` · `fork-test` · `struct-test` · `linear-test` · `inner-test` · `child-test`. *(evidence: `testworkflows.go · ByName`)*
- `RegisterTestWorkflows` delegates to the lists: `Workflows` (the ten entries, each with `WithMaxRecoveryAttempts(3)`) · `Queues` (`frozen` · `live-frozen`, concurrency 0) — only what the lists name is registered. `RegisterQueues` runs on every route (the full boot and the subset route); `ByName` is the single source of the ten entries. *(evidence: `testworkflows.go · RegisterTestWorkflows · RegisterQueues · ByName`)*
- The frozen queues: `frozen` and `live-frozen` (the live tests' second queue), both `WithWorkerConcurrency(0)` — a run enqueued on them is never claimed, it stays ENQUEUED until the owning test cancels it. *(evidence: `testworkflows.go · Queues`)*
- `live-test`'s payload (`mode`) picks the run outcome: `"err"` → the step errors · `"block"` → the step blocks until the cancel · anything else (including the dispatch default) → success. No fixed sleep: the block mode is the deterministic Running window, released by the cancel itself. *(evidence: `testworkflows.go · liveTestWorkflow` · `liveStep`)*
- `block-test` is the single-step Running fixture: the `block` step blocks until the run is cancelled; the test that owns the run cancels it and the step unblocks (the suite settles — no leftover tail). *(evidence: `testworkflows.go · blockTestWorkflow` · `blockUntilCancelled`)*
- `blockUntilCancelled` polls the run's own row (`dbos.ListWorkflows` with the workflow id captured at the top of the workflow — constant per run, replay-safe) every 100ms and returns when the row is `CANCELLED`; the row stays `CANCELLED` (terminal) no matter what the workflow does after. *(evidence: `testworkflows.go · blockUntilCancelled` · `blockPollInterval`)*
- `noinput-test` takes no form values (the dispatch posts nil); `struct-test` takes a struct input (`Name` · `StartDate`, defaults `FlowDeck` · `2026-01-01`); `fork-test` forks by the payload (empty → `right`, non-empty → `left`); `child-test` runs `inner-test` as a child run (`dbos.RunWorkflow` — a child row in the run's steps, a node named after the child func in the paths graph). *(evidence: `testworkflows.go`)*
- `resume-test` fails its step while `FlakyFail` is on (the resume tests flip it off and resume to success). *(evidence: `testworkflows.go · flakyTestWorkflow` · `FlakyFail`)*

## Facts (validated runtime)

| Fact | Evidence | Validated |
|---|---|---|
| No test leaves an in-flight run: every block fire is cancelled by its owning test, and every frozen-queued fire is cancelled too. | the race suite | 2026-11-07 |
| The booted suites (unit · race) boot per test: a fresh sqlite file per test — no shared file, no `-p 1`. | `ci.yml · race job` · `testcore.go · BootTest` | 2026-11-07 |
| The e2e job is SQLite (`sqlite:/tmp/flowdeck-e2e.db`, app `flowdeck_e2e`) — Postgres is production-only (docker compose). | `.github/workflows/ci.yml` | 2026-11-04 |
