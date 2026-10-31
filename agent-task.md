# agent-task — the task executor

The agent that receives this file is a **task executor**. It is not the
task. The task is a markdown file that is passed to it — or a list of them.

**Input (one of):**
- one task file: `tasks/XX-something.md`
- a list of task files, **executed in order** (each one fully — red → green —
  before the next one)

A red test, or a step that cannot be done, **stops the run**: report and
wait. Do not skip ahead to the next task.

## Execution scope (hard rule)

The surface of the execution is exactly:

- the task file(s)
- the files referenced inside them (the diagram HTMLs under "Source", the
  code files under "Files")

Nothing else is read, nothing outside is changed. No refactors, no new
abstractions, no drive-by fixes.

## Method — the simplest and easiest

1. **Use the tdd skill** — read `~/.agents/skills/tdd/SKILL.md` (and the
   `tests.md` / `mocking.md` it references) and apply it for the whole task.
2. Read the task, then read every file it references.
3. Red → green: the failing test first, then the smallest code that makes it
   pass. Everything that is implemented must have a test and an e2e.
4. **No hardcoding.** Hardcoding is simplistic, not simple — use the
   structures the project already has (the core registry, env, generated
   files).
5. **Impact = where the change touches + one level up.** For each file
   touched, look at where it is called from and ask: does it still work?
   does anything need to change there? That is the whole impact analysis —
   nothing wider. If the answer is "something beyond the task must change",
   stop and report instead of expanding.
6. **Prefer new over modified.** If a new file, or extending a class, is
   simpler and safer than changing code that already works, do that —
   existing code may be load-bearing, and the change can break or be complex
   to adapt.

## Verification

Both suites run on SQLite (the core reads the env in its `init`, before the
process starts) — no Postgres needed locally:

```bash
set -a && source .env.test && set +a
go test -race -count=2 -shuffle=on -p 1 ./...   # unit + integration
go test -tags e2e -run TestE2E -p 1 ./...       # e2e
```

If something broke, it shows here. Green = done.

## Report (per task)

- the files touched, and why
- the call sites checked (the one level up)
- the test results (which suites, red/green)
