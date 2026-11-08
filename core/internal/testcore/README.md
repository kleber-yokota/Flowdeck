# core/internal/testcore

The shared setup of the core test binaries: the per-test boot (`BootTest` — one explicit boot per test, on a fresh sqlite file) and the shared poll helpers. There is no env boot and no skip: the default state is not-booted, and every booted test boots and shuts down its own core.

## Rules (decisions · pinned values)

- `BootTest(t, dirOrNames)` = `boot.Boot` (app `flowdeck_test`, DSN `sqlite:` + `t.TempDir()/test.db`) → the registration → `boot.Launch`, with the `Shutdown` in a `t.Cleanup` (the `TempDir` removal is the framework's). The argument is generic: a `string` is the `workflowsDir` (`boot.SetWorkflowsSource(os.DirFS(dir))` + `testworkflows.RegisterTestWorkflows` — the paths graph and the full registration); a `[]string` is the names of the test workflows to register (a subset of the ten — an unknown name fails the test — plus the fixture queues, no source dir, so no paths graph). The returned context is live for the test and nil after the cleanup. *(evidence: `testcore.go · BootTest` · `TestBootTestNamedSubset`)*
- The test-only workflows live in `core/testworkflows` (registered, never part of the `workflows` package) — so they do not appear in the production registry. *(evidence: `../testworkflows/testworkflows.go · ByName` · `boot/lifecycle.go · warnUnregistered`)*
- `PollState` polls the badge state until the wanted one — a `success` stops the poll too (the run finished, just not with the wanted state). 20s deadline, 200ms tick. *(evidence: `testcore.go · PollState`)*

## Facts (validated runtime)

| Fact | Evidence | Validated |
|---|---|---|
| The boot→Launch→Shutdown cycle is repeatable in one process: the second boot on a fresh sqlite file works, and after the `Shutdown` `Ctx()` is nil — so the per-test boot needs no shared file and no `-p 1`. | `TestBootTestBootsAndCleans` · the race suite (`go test -race -count=2 -shuffle=on ./...`) | 2026-11-07 |
