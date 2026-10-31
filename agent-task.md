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
- `diagrams/package-map.html` — the source of truth for the contacts and
  the verified facts

Nothing else is read, nothing outside is changed. No refactors, no new
abstractions, no drive-by fixes.

## Trust (anti-reread — the rules that keep the run short)

- **Facts are trusted.** A fact row in the map (Fact · Evidence · Validated)
  is used as-is — no re-reading the SDK source, no re-running the spike —
  except when its diff-check fails: the files the evidence names changed
  since the validated date, or the SDK version in `go.mod` moved. Then
  re-validate **that fact only** (one targeted grep/spike) and refresh the
  row.
- **Reads are scoped.** The map's contacts rows are the impact analysis —
  do not inventory the package. Read the task, then the files it names;
  research reads of 3+ files go through one batched round-trip, not one
  read per file.
- **Spikes are bounded.** One spike per unknown, and only for runtime values
  the code cannot show (row shapes, status transitions, queue behavior): a
  temporary test file, deleted before the commit, the result recorded under
  "Notes" in the task file (fact · evidence · how verified).
- **Decide once.** A value the task or the diagram does not pin: pick one,
  record it under "Notes" (value + one line of why), do not revisit it. A
  new *shape* (a route, a state, a badge the task does not name) is out of
  scope — stop and report.

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

## Green — promote the facts (last step of every task)

The map is the source of truth the next task runs from; it must not lag the
code. After the step is green, before the report:

1. The task file's "Notes" now holds the facts discovered during the run
   (and the ones the author seeded).
2. **Promote them into `diagrams/package-map.html`** — the facts table
   (`#facts`), one row per fact: Fact · Evidence (the spike name / the
   source line) · Validated (the task file · date · sdk version). A fact is
   a verified *runtime behavior* — not a code shape (that is the contact
   rows) and not a decision (that is the task Notes).
3. **Refresh the contact rows the step touched** (a new route, a new Port
   member, a new function) and the test rows that now assert it.
4. The map update goes into **the task's commit** — one task, one commit:
   code + tests + map together.

New fact ⇒ the diagram is updated, in the same commit.

## Report (per task)

- the files touched, and why
- the call sites checked (the one level up)
- the facts promoted to the map (one line each)
- the test results (which suites, red/green)
