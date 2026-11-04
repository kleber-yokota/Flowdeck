# core/internal/testcore

The shared setup of the core test binaries: the env-driven lifecycle (the paths graph from the workflows tree on disk, and — when the env context exists — the test workflows and the executor) plus the shared poll helpers. Every core test binary calls `Setup` from its `TestMain`.

## Rules (decisions · pinned values)

- `Setup` = the paths graph from the workflows tree on disk (`os.DirFS`, relative to the test binary's directory) + — when the env context exists — `RegisterTestWorkflows` + `Launch`. The env-var absence is not an error: `NeedCore` skips the test (the mock tests run without env). *(evidence: `testcore.go · Setup` · `NeedCore`)*
- The test-only workflows (`failing-test` · `resume-test`, with `WithMaxRecoveryAttempts(3)`) are registered in testcore, never in the workflows package — so they do not appear in the production registry. *(evidence: `testworkflows.go · RegisterTestWorkflows` · `boot/lifecycle.go · warnUnregistered`)*
- `PollState` polls the badge state until the wanted one — a `success` stops the poll too (the run finished, just not with the wanted state). 20s deadline, 200ms tick. *(evidence: `testcore.go · PollState`)*

## Facts (validated runtime)

| Fact | Evidence | Validated |
|---|---|---|
| The test env: `.env.test` sets `DBOS_APP_NAME=flowdeck_test` and `DBOS_SYSTEM_DATABASE_URL` = sqlite on disk (the env suite: the map's `#suites`). | `.env.test` · the suites table in `diagrams/package-map.html#suites` | 2026-11-03 |
