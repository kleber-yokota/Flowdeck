# core/testcore

The shared setup of the `core` · `web` · `workflows` test binaries: the per-test boot (`BootTest` — one explicit boot per test, on a fresh sqlite file), the shared `TestMain` body (`TestMain`), and the shared poll helpers. There is no env boot and no skip: the default state is not-booted, and every booted test boots and shuts down its own core.

## Rules (decisions · pinned values)

- `BootTest(t, dir, register…)` = `boot.Boot` (app `flowdeck_test`, DSN `sqlite:` + `t.TempDir()/test.db`) → `boot.SetWorkflowsSource` (the `dir`, relative to the test binary's directory) → the registration → `boot.Launch`, with the `Shutdown` in a `t.Cleanup` (the `TempDir` removal is the framework's). The register funcs, when given, replace the full registration with a named subset on top of the fixture queues. The returned context is live for the test and nil after the cleanup. *(evidence: `testcore.go · BootTest` · `TestBootTestNamedSubset`)*
- The default registration (no register funcs) is the ten test-only fixtures + the frozen queues of `core/testworkflows` (their separation from the production registry: that page); the register route is `testworkflows.RegisterQueues` + the named funcs. *(evidence: `testcore.go · BootTest` · `testworkflows.go · RegisterTestWorkflows · RegisterQueues`)*
- `TestMain(m, dir)` is the shared `TestMain` body: it sets the workflows source (the `dir`, relative to the test binary's directory) and runs the tests — the source dir only, no boot, no env: the bootless tests need no launch, and every booted test boots per-test via `BootTest`. *(evidence: `testcore.go · TestMain` · `core/core_test.go · TestMain`)*
- The package is exported (no `internal`): the `web` and `workflows` test binaries import it too — the three suites share the one boot, the one `TestMain` body and the poll helpers. *(evidence: `web/web_test.go` · `workflows/workflows_test.go`)*
- `PollState` polls the badge state until the wanted one — a `success` stops the poll too (the run finished, just not with the wanted state). 20s deadline, 200ms tick. *(evidence: `testcore.go · PollState`)*

## Facts (validated runtime)

| Fact | Evidence | Validated |
|---|---|---|
| The boot→Launch→Shutdown cycle is repeatable in one process: a fresh sqlite file per boot, and `Ctx()` is nil after the `Shutdown` — no shared file, no `-p 1`. | `TestBootTestBootsAndCleans` · the race suite (`go test -race -count=2 -shuffle=on ./...`) | 2026-11-08 · dbos v1.7.0 |
| The default `BootTest` route registers the ten fixtures + the frozen queues (`frozen` · `live-frozen`) on every boot. | `testcore.go · BootTest` · `testworkflows.go · RegisterTestWorkflows` · `TestBootTestBootsAndCleans` | 2026-11-08 · dbos v1.7.0 |
