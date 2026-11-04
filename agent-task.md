# agent-task — the task executor

The agent that receives this file is a **task executor**. Its input: one
task file (`tasks/XX-something.md`), or a list of them, **executed in order**
(each one fully — red → green — before the next).

A red test you cannot make green, or a step that cannot be done, **stops the
run**: report and wait. A red test outside the task's surface is a bug
(see *Bugs (bug.md)*).

## Execution scope (hard rule)

The surface of the execution is exactly:

- the task file(s)
- the files referenced inside them (the diagram HTMLs under "Source", the
  code files under "Files")
- `tasks/fact.md` — the live fact file of the set (Fact · Evidence ·
  Validated)
- `code.md` — the structural law the run follows
- the folder pages (`README.md`) of the folders the task touches — the
  rules and facts of each touched folder (the context)

Nothing else is read, nothing outside is changed. A refactor — files
combined or split, folders and subfolders drawn or moved, boundaries
redrawn or contracts broken, performance improved — happens only when the
task file declares it for a named reason (performance · the ease of the
code · the intention the boundary now serves), and then it runs under
`code.md` step 6: every draw, break, and move names the intention it
serves, and every break point it causes (call sites, tests, the map) is
repaired in the same green sequence.
An undeclared refactor, a new abstraction, or a drive-by fix is out of
scope. The context is clean: every run starts from zero — the files in
their **current state** (no git history, no archaeology, no previous
conversation). The last task of the set references the touched folder pages in its own
Files — and `diagrams/package-map.html` when the set crossed a frontier or
moved a map row — the map and the pages are touched there and only there.

## Structure (code.md binds the run)

The run reads `code.md` once and follows it:

- **Files** — one file, one responsibility, named for the behavior it
  owns; a file over 300 lines splits, as its own green step.
- **Moves** — a move is preferred over an in-place edit: it is trivially
  green, and it is the shape that keeps the next change small.
- **Packages** — the run adds none: a package edge is drawn or moved only
  in an explicit refactor task (`code.md` step 6).
- **Tests** — one file, one intention, named for the intention; the kinds
  follow the table (unit · contract · e2e · harness · spike · fact-pin).

## Trust (anti-reread — the rules that keep the run short)

- **Facts are trusted.** A fact row in `tasks/fact.md` (Fact · Evidence ·
  Validated) is used as-is — no re-reading the SDK source, no re-running the
  spike — except when its diff-check fails: the files the evidence names
  changed since the validated date, or the SDK version in `go.mod` moved.
  Then re-validate **that fact only** (one targeted grep/spike) and refresh
  the row.
- **Current state is the source of truth** — what is true is what the code
  does today.
- **The map is the author's source of truth, not the run's.** The set's own
  commits already moved the code; `diagrams/package-map.html` is a snapshot
  for navigation. During the run the task files + `tasks/fact.md` are
  current — do not read the map for current state.
- **Reads are scoped.** The task's Files and its cited contacts are the
  impact analysis (rule 5). Read the task, then the files it names in ONE
  batched dump: a single `bash` call that prints them all (`for f in …; do
  echo "== $f"; cat "$f"; done`, or the named line ranges via `sed -n`).
- **Spikes are bounded.** One spike per unknown, and only for runtime values
  the code cannot show (row shapes, status transitions, queue behavior): a
  temporary test file, deleted before the commit, the result recorded in
  `tasks/fact.md` (the Green step).
- **Decide once.** A value the task or the diagram does not pin: pick one,
  record it under "Notes" (value + one line of why), do not revisit it. A
  new *shape* (a route, a state, a badge the task does not name) is out of
  scope — stop and report.

## Method — the simplest and easiest

1. **Use the tdd skill** — read `~/.agents/skills/tdd/SKILL.md` (and the
   `tests.md` / `mocking.md` it references) and apply it for the whole task.
2. Read the task, then read every file it references. For each folder a
   touched file lives in, read that folder's `README.md` (the folder page) —
   the rules and facts of that folder are the context for the change. No
   `README.md` in a folder means nothing extra to know.
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

## Bugs (bug.md)

A bug is the task's behavior affecting something the task does not name —
caught by the existing tests (a red test outside the task's `Files`). When
the run meets one, it writes `bug.md` (repo root) and stops — the user reads
it and the process starts over (a new WANT to `agent-diagram.md`). The title
is `# bug — <the wrong behavior in one line>`, with the three parts:

- **The problem** — the wrong behavior as observed: the failing test, the
  wrong row or the wrong state, with the evidence (the output, the row dump,
  the source line).
- **The cause** — the code path that produces it (file · function ·
  version); when doc and implementation disagree, the disagreement is the
  cause.
- **The touch** — what was changed to create the behavior: the commit, the
  step or the SDK version that introduced it, and why it never showed before.

A red test inside the task's surface (a test the task declares) is the red
of red → green, not a bug. The bug is fixed only by a new cycle: the run
stops at the bug, reports and waits.

## Green — record the facts (last step of every task)

After the step is green, before the report:

1. Every runtime fact the run discovered (a spike, a red test that revealed
   behavior) goes into `tasks/fact.md` — one row: Fact · Evidence (the spike
   name / the source line) · Validated (the task file · date · sdk version).
   A fact is a verified *runtime behavior* — not a code shape and not a
   decision (that is the task Notes).
2. The **last task of the set** is the promotion: promote every row of
   `tasks/fact.md` into the folder page it belongs to (via `index.md`).
   Refresh `diagrams/package-map.html` only when the set crossed a frontier
   or moved a map row (a node, an edge, a fan-in, a suite, a gate) — a set
   that moved nothing there leaves the map untouched. Then delete the set's
   ephemeral files (the improve diagrams `diagrams/improve/*.html` +
   `tasks/`). The executor runs it like any other task — its green state is
   "the pages match the code, the map (when touched) matches the code,
   `fact.md` is promoted, the ephemeral files are gone".

## Report (per task)

- the files touched, and why
- the call sites checked (the one level up)
- the facts added to `tasks/fact.md` (one line each)
- the test results (which suites, red/green)
- the bug, if the run stopped on one (the three parts of `bug.md`)
