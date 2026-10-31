# agent-write-task — the task author

The agent that receives this file is a **task author**. It writes the task
files (`tasks/*.md`) that the task executor (`agent-task.md`) runs. The
executor reads exactly the task file and the files it references — nothing
else (the hard rule of `agent-task.md`). The author puts everything the
executor needs into the files: the task files (the spec, the whole commit,
the green state, the ownership) and `tasks/fact.md` (the verified facts the
set depends on).

## Inputs (three, in order)

1. **Spec** — the design HTMLs of the feature (under `diagrams/`). They say
   *what to build*.
2. **Contacts** — `diagrams/package-map.html`. They say *how it works*:
   which file, function and class touches which identifier — and they hold
   the **verified facts** (`#facts`) of runtime behavior (SDK internals,
   Postgres rows, queue behavior), each with its evidence. The map is the
   author's source of truth for navigation. The author reads this file for
   every task set, always.
3. **Code** — the call sites and the test files. The map is a guide; the
   code is the source of truth for **names and shapes**. The author greps
   only the symbols a step changes.

The diagrams show "today" and "target". They do not show the intermediate
states, and they do not name the tests. The green state of each step comes
from the code, not from a diagram.

## `package-map.html` — how to read it

One file, two readers: the SVG is the human view, the sections are the
machine-readable map of the same data.

| Section | What it holds | The question it answers |
|---|---|---|
| `#tasks` | the "Task → Read first" table | Where does a common task start? |
| `#contacts` | "Edge · Where (file · function) · What (identifiers)" | Which file · function touches which identifier, per package edge? |
| `#package-*` | "File · Responsibility · Functions" + the contact rows | Which files, functions and test files live in the package? |
| `#facts` | "Fact · Evidence · Validated" | What runtime behavior is already validated (and by what proof)? |

The package sections are: `main · core · workflows · gen · status · graph ·
web · webgraph · dbos`. The contact rows read `X →` (this package touches
X) and `X ←` (X touches this package).

For every step and every symbol it changes, the map + the grep answer:

- which **file · function** calls it (the production call sites);
- which **test file · function** asserts it (the test call sites);
- which **package edge** the touch crosses (the invariant to keep).

When the map and the code disagree, the code wins for the current state.
The author does not edit the map mid-set: the stale row is fixed by the
set's last task (the map update), together with the promoted facts.

## Trust (anti-reread — the rules that keep the authoring short)

- **Facts are trusted.** A fact row (`#facts`: Fact · Evidence · Validated)
  is used as-is — no re-reading the SDK source, no re-running the spike —
  except when its diff-check fails: the files the evidence names changed
  since the validated date, or the SDK version in `go.mod` moved. Then
  re-validate **that fact only** (one targeted grep or spike) and note the
  refresh.
- **Reads are scoped.** Read the map sections the task touches, then the
  code files they name. Do not inventory the package: the contacts rows are
  the impact analysis. Research reads of 3+ files go through one batched
  round-trip, not one read per file.
- **The author seeds `tasks/fact.md`.** Every fact the set depends on
  becomes a row of the set's fact file — Fact · Evidence (spike name /
  source line) · Validated (author · date · sdk version) — copied from
  `#facts` or produced by the bounded spike. The executor appends the facts
  it discovers during the run; the set's last task promotes the rows into
  the map.
- **Spikes are bounded.** One spike per unknown, and only for a runtime
  value no code shows (row shapes, status transitions, queue behavior): a
  temporary test file, deleted before the commit, the result recorded as a
  row of `tasks/fact.md` (value · evidence · how verified).
- **Decide once.** A value the diagram does not pin: pick one, record it
  under "Notes" (value + one line of why), do not revisit it. A new value
  belongs in the diagram — not the task.

## The green state (the rule that makes a task runnable)

- **One step = one commit = one green state.** After the step, `go build`
  compiles and the suite passes. A step that leaves the tree red is a wrong
  step.
- **The signature change travels with its caller.** The step that changes a
  type or signature adapts every call site in the same commit. The call
  sites come from `#contacts` and the grep.
- **The intermediate state is declared.** The type changes in the step of
  its consumer. The earlier steps keep the old type and say so: "Of still
  returns `[][]string` until STEP 05".
- **Tests are declared, not discovered.** The task names every test that
  dies, changes shape or moves: "rewrite `TestPaths*` (the flat assertions
  become tree assertions)" · "delete `TestX` — re-specified in STEP N".
- **The author's check (per step):** for each symbol whose signature
  changes, every call site is either in the step's `Files`, or unchanged
  (the type is still the old one), or a declared "keep" that compiles with
  the new type. If the check fails, move the type change to the consumer's
  step.

## Template (every file, same sections, same order)

````markdown
# STEP NN — <title>

**Source (the HTML parts this task reads):**
- `<diagram>.html` → <panel · list · card · lane>.

**Files (the whole commit):**
- Add `core/graph/block.go` (`BlockType` · `Block` · `Child`).
- Change `core/graph/graph.go` (`PathGraph` · `Of`).
- Adapt `web/web_test.go` (`fakeGraph` takes a `Block`).

## Purpose

<What changes from today to after. Two to four sentences.>

## Do

1. Write the test first (red): <the failing test>.
2. <One action, one line.>
N. Make the tests pass (green).

## Green (definition of done)

- `go build ./...` + the unit suite green (the env of `.env.test`).
- <The specific assertions that prove the step.>

## Out of scope (owned by other steps)

- `core/graph/parse.go` — the flat walk is replaced in STEP 02.

## Notes (decisions)

- <Decision>: <the reason, one line>.
````

Rules of the fields:

- `Files` is the **whole commit**: the owned files, the call sites to
  adapt, the tests that change shape. Nothing the commit touches is outside
  the list.
- Every entry names the **file and the function or class**: "Change
  `core/graph/graph.go` (`PathGraph` · `Of`)". A file without a function is
  not a task.
- `Source` cites the exact parts, so the executor opens only what it needs
  (the reading rule of `diagrams/README.md`).
- `Out of scope` names the file and the owning step. The executor does not
  read the other task files to learn the ownership.
- `Notes` carries the decisions (value + the why). The runtime facts the
  task depends on live in `tasks/fact.md` — the task cites the row it needs.
  A fact without evidence is a guess.

## ASD-STE100 (as applied to the task files)

- One idea per sentence. One action per line.
- Active voice. Imperative verbs: Add · Change · Adapt · Keep · Write ·
  Run.
- A defined term keeps its definition. The terms live in the README
  ("Terms") and the tasks use them unchanged.
- The values are the diagram's values: hex colors, levels, ids, step
  names. No approximations.
- The sections are the same, in the same order, in every file.
- The task says *what* to do. The diagrams say *why*. No motivation, no
  alternatives, no "consider" in a task file.

## The set (`tasks/README.md`)

The README holds, once for the whole set:

- The goal (today → target, two sentences).
- **Steps** — the table of the files and the step names. The **last step is
  always the map update**: promote `tasks/fact.md` into the package-map
  (`#facts`) and refresh the contact rows the set touched — written as the
  last task file.
- **Terms** — the vocabulary, one definition per term. A term is never
  redefined in a task.
- **Source map** — where each piece of information lives (HTML · part).
- **Rules** — the constraints that hold for every step. A rule that applies
  to every step lives here, not in the tasks.

## What the author does not do

- It does not leave the impact analysis to the executor. The author names
  the surface. The executor verifies it (agent-task.md, rule 5).
- It does not create a step that is not green on its own.
- It does not put two steps in one file, or one step in two files.
- It does not cite a diagram part it did not open.
- It does not edit `diagrams/package-map.html` mid-set: the map is the last
  task's surface.
- It does not re-verify a fact that passes its diff-check (the Trust rule).
