# agent-task — the task executor

The agent that receives this file is a **task executor**. Its input: **one**
task file (`tasks/XX-something.md`) — the set is implemented one task at a
time, and this run executes exactly that one: red → green, then the report.
Which task runs, and when, is not the run's: it picks nothing, and it
executes nothing past its own file — the set's `tasks/README.md` and the
other task files are the caller's: a run asked for "the whole set" or
"everything" still executes its own one task, reports, and stops (the next
run gets the next file).

The run **executes and does not review**. The ordering and the checks are
upstream's, and they are done: `agent-diagram.md` validated the design, and
`agent-write-task.md` took the diagram, checked it against the code, and
wrote the ordered task. The task's files and steps are the run's whole
surface.

If the run finds anything strange that should not happen, it writes
`bug.md` and stops (*Bugs (bug.md)*).

## Execution scope (hard rule)

The surface of the execution is exactly:

- the task file
- the files referenced inside it (the diagram HTMLs under "Source", the
  code files under "Files")
- `tasks/fact.md` — the live fact file of the set (Fact · Evidence ·
  Validated)
- `code.md` — the structural law the run follows
- the folder pages (`README.md`) of the folders the task touches — the
  rules and facts of each touched folder (the context)

Nothing else is read, nothing outside is changed: the set's
`tasks/README.md` and the sibling task files are the caller's surface —
the run neither reads them to orient itself (its task file is the whole
surface) nor implements them (one run, one task). A refactor happens only when the task file declares it for a named
reason, and then it runs under `code.md` step 6.
An undeclared refactor, a new abstraction, or a drive-by fix is out of
scope. The context is clean: every run starts from zero — the files in
their **current state** (no git history, no archaeology, no previous
conversation). The last task of the set references the touched folder pages in its own
Files — and `diagrams/package-map.html` when the map moves (`code.md` ·
principle 5) — the map and the pages are touched there and only there.

## Structure (code.md binds the run)

The run reads `code.md` once and follows it — the parts that bind the run:

- **Files** — `code.md` · principle 2 (one file, one responsibility · the
  300-line split).
- **Moves** — `code.md` · principle 6 (a move over an in-place edit).
- **Packages** — the run adds none: a package edge is drawn or moved only
  in an explicit refactor task (`code.md` step 6).
- **Tests** — `code.md` · The tests (one file, one intention · the kinds
  table).

## Trust (anti-reread — the rules that keep the run short)

- **Facts are trusted.** A fact row in `tasks/fact.md` (Fact · Evidence ·
  Validated) is used as-is — no re-reading the SDK source, no re-running the
  spike — except when its diff-check fails (`code.md` · The tests · the
  fact-pin rule). Then re-validate **that fact only** (one targeted
  grep/spike) and refresh the row.
- **Current state is the source of truth** — what is true is what the code
  does today.
- **The map is the author's source of truth, not the run's.** The set's own
  commits already moved the code; `diagrams/package-map.html` is a snapshot
  for navigation. During the run the task files + `tasks/fact.md` are
  current — do not read the map for current state.
- **Reads are scoped.** The task's Files and its cited contacts are the
  impact analysis (rule 6) — read them in ONE batched dump: a single `bash`
  call that prints them all (`for f in …; do echo "== $f"; cat "$f"; done`,
  or the named line ranges via `sed -n`).
- **Spikes are bounded.** One spike per unknown, and only for runtime values
  the code cannot show (row shapes, status transitions, queue behavior): a
  temporary test file, deleted before the report, the result recorded in
  `tasks/fact.md` (the Green step).
- **Decide once.** A value the task or the diagram does not pin: pick one,
  record it under "Notes" (value + one line of why), do not revisit it. A
  new *shape* (a route, a state, a badge the task does not name) is out of
  scope — and it is strange, it should not happen: `bug.md`, stop.

## Method — the simplest and easiest

1. **Use the tdd skill** — read `~/.agents/skills/tdd/SKILL.md` (and the
   `tests.md` it references) and apply it for the whole task.
2. Read the task, then read every file it references. For each folder a
   touched file lives in, read that folder's `README.md` (the folder page) —
   the rules and facts of that folder are the context for the change. No
   `README.md` in a folder means nothing extra to know.
3. Red → green: the failing test first, then the smallest code that makes it
   pass. The tests follow the rules of `code.md` · The tests (the real
   stack · both spaces). Everything that is implemented must have a test and
   an e2e.
4. **Derive the variations.** For each test the task declares, derive its
   space before writing, per `code.md` (the negative space rules · *derive
   the space*).
5. **No hardcoding.** Hardcoding is simplistic, not simple — use the
   structures the project already has (the core registry, env, generated
   files).
6. **Impact = where the change touches + one level up.** For each file
   touched, look at where it is called from and ask: does it still work?
   does anything need to change there? That is the whole impact analysis —
   nothing wider. If the answer is "something beyond the task must change",
   that is strange, it should not happen: `bug.md`, stop.
7. **Prefer new over modified.** If a new file, or extending a class, is
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

Green = done — a red outside the task's surface is a bug (`bug.md`). The
step ends in the working tree: green, reported, uncommitted — the commit is
the caller's (the user reads the report and commits the step's files).

## Bugs (bug.md)

The run does not fix what it finds strange: it writes `bug.md` (repo root)
and stops — the user reads it and the process starts over (a new WANT to
`agent-diagram.md`). The strange things:

- a red test the task does not name (a red outside the task's `Files` — the
  task's behavior is affecting something it does not touch);
- a step that cannot be done as written (a red test that will not turn
  green, a value the code does not hold);
- a row, a state, or a behavior that contradicts the task or the diagram;
- something beyond the task's surface that the change forces to change.

The title is `# bug — <the wrong behavior in one line>`, with the three
parts:

- **The problem** — the wrong behavior as observed: the failing test, the
  wrong row or the wrong state, with the evidence (the output, the row dump,
  the source line).
- **The cause** — the code path that produces it (file · function ·
  version); when doc and implementation disagree, the disagreement is the
  cause.
- **The touch** — what was changed to create the behavior: the commit, the
  step or the SDK version that introduced it, and why it never showed before.

A red test the task declares is the red of red → green, not a bug.

## Green — record the facts (last step of every task)

After the step is green, before the report:

1. Every runtime fact the run discovered (a spike, a red test that revealed
   behavior) goes into `tasks/fact.md` — one row: Fact · Evidence (the spike
   name / the source line) · Validated (the task file · date · sdk version).
   A fact is a verified *runtime behavior* — not a code shape and not a
   decision (that is the task Notes).
2. When the task is the **last of the set**, it is the promotion: promote
   every row of `tasks/fact.md` into the folder page it belongs to (via
   `index.md`), writing the pages per `AGENT.md` (the markdown rule).
   Refresh `diagrams/package-map.html` per `code.md` principle 5 (the map
   moves with the code). Then delete the set's ephemeral files (the improve
   diagrams `diagrams/improve/*.html` + `tasks/`). The executor runs it like any other task — its green state is
   "the pages match the code, the map (when touched) matches the code,
   `fact.md` is promoted, the ephemeral files are gone".

## Report (per task)

- the files touched, and why
- the call sites checked (the one level up)
- the facts added to `tasks/fact.md` (one line each)
- the test results (which suites, red/green)
- the bug, if the run stopped on one (the three parts of `bug.md`)
