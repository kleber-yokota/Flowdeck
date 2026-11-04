# agent-write-task — the task author

The agent that receives this file is a **task author**. It writes the task
files (`tasks/*.md`) the task executor runs. The executor reads exactly the
task files and the files they reference — nothing else — so the author puts
everything the executor needs into them: the task files (the spec, the whole
commit, the green state, the ownership) and `tasks/fact.md` (the verified
facts the set depends on).

## Inputs (three, in order)

1. **Spec** — the design HTMLs of the feature (under `diagrams/improve/`).
   They say *what to build*.
2. **Contacts** — `diagrams/package-map.html` (the boundary map) and the
   folder pages (`index.md` + the folder `README.md`s). The map says *how it
   is wired* (frontiers · groups · suites · gates); the folder pages hold
   the verified facts and rules, each with its evidence. The author reads
   the map for every set — and first after every compaction: names come
   from the map and the pages, never from a re-read of the source.
3. **Code** — the files the map sections name, read in ONE batched dump:
   a single `bash` call that prints them all (`for f in …; do echo "== $f";
   cat "$f"; done`, or the named line ranges via `sed -n`). The map is a
   guide; the code is the source of truth for **names and shapes**. Grep
   confirms a digest row; it does not build the digest.

The diagrams show "today" and "target". They do not show the intermediate
states, and they do not name the tests. The green state of each step comes
from the code, not from a diagram.

The context is clean: only these three inputs — the files in their current
state (no git history, no archaeology, no previous conversation).

## `package-map.html` + the folder pages — how to read them

One map, no facts: the map is navigation, the folder pages are the source
of truth for the "what" and the "why".

**The map** (`diagrams/package-map.html`) — the boundary view: the packages
that cross a group border, and the group-level contacts.

| Section | What it holds | The question it answers |
|---|---|---|
| `#frontiers` | the group-to-group edges | Who touches whom, at the group level? |
| `#groups` | the 4 groups + their packages + their pages | Which package is in which group, and where is its page? |
| `#suites` | the green baseline (the suites) | What must be green before and after the set? |
| `#gates` | the structural checks | What cheap check keeps the architecture honest? |

**The folder pages** — the facts and rules:

- `index.md` (repo root) is the **page index**: one row per page with a
  `covers` keyword set. Read it first; open only the pages whose `covers`
  intersect the set's touch set. It holds no facts.
- A folder `README.md` (e.g. `core/reads/README.md`) is the **source of
  truth** for that folder: its rules and facts, each a `Rule`/`Fact` with
  `Evidence` (file · line · symbol) and `Validated` (who · when · how). No
  `README.md` in a folder means nothing to know about that folder.

For every step and every symbol it changes, the map + the touched pages +
the grep answer:

- which **file · function** calls it (the production call sites);
- which **test file · function** asserts it (the test call sites);
- which **frontier** the touch crosses (the invariant to keep).

When the pages and the code disagree, the code wins. The author does not
edit the map or the pages mid-set: the stale row is fixed by the set's last
task (the promotion), together with the promoted facts.

## Trust (anti-reread — the rules that keep the authoring short)

- **Facts are trusted.** A fact row (a folder page via `index.md`: Fact ·
  Evidence · Validated) is used as-is — no re-reading the SDK source, no
  re-running the spike — except when its diff-check fails: the files the
  evidence names changed since the validated date, or the SDK version in
  `go.mod` moved. Then re-validate **that fact only** (one targeted grep or
  spike) and note the refresh.
- **Reads are scoped.** Read the map sections the task touches; their named
  code files enter through the batched dump (input 3). The contacts rows
  are the impact analysis.
- **The author seeds `tasks/fact.md`.** Every fact the set depends on
  becomes a row — Fact · Evidence (spike name / source line) · Validated
  (author · date · sdk version) — copied from the folder pages (via
  `index.md`) or produced by a bounded spike. Names and values are not
  facts: they live in `tasks/symbols.md` (the symbol digest, below).
- **Spikes are bounded.** One spike per unknown, and only for a runtime
  value no code shows (row shapes, status transitions, queue behavior): a
  temporary test file, deleted before the commit, the result recorded as a
  row of `tasks/fact.md` (value · evidence · how verified).
- **Decide once.** A value the diagram does not pin: pick one, record it
  under "Notes" (value + one line of why), do not revisit it. A new value
  belongs in the diagram — not the task.

## The symbol digest (`tasks/symbols.md`)

`fact.md` holds the runtime facts; the map holds the names; `tasks/symbols.md`
holds the **names and values of the current code** — the half a compaction
wipes that the other two do not keep. The task files demand exact names and
values ("no approximations"), and the post-compaction summary does not carry
them: the digest is the on-disk copy.

**Structure** — one section per touched surface, one line per symbol. The
surfaces are known before the first task file: the touched packages (the
map's `#groups` + `index.md` name them) + the spec HTMLs + the SDK.

| Section | The row holds |
|---|---|
| production file | the functions and their shapes — guards, signatures, the key constants **with values** (ex. a signature, a guard, `name = value`) |
| test file | the test function names + the pinned assertion values (ex. the canvas `W×H`, the `N` rows of a workflow run) |
| template | the section order + the conditions (ex. a button shows on one status only) |
| the spec HTMLs (`diagrams/improve/`) | the pinned spec values the task cites (ex. a px size, a hex color, the 404/500 mapping) |
| the SDK (the `go.mod` version) | the verified signatures + behaviors (ex. the states a function acts on, the states where it is a no-op) |

**Rules:**
- The author writes the digest **before the first task file**, from the
  batched dump (input 3). It costs exactly one dump — not zero (the shapes
  live in no other input) and not more.
- It is **ephemeral**: the last task (the promotion) deletes it with
  `tasks/`, like the improve diagrams.
- After a compaction: read the map (the names) + the digest (the values) +
  the written task files; open the source only for a symbol missing from
  both — one batched dump (input 3).
- Digest and code disagree: **the code wins** — fix the digest row in place
  (the same rule as the map and code disagree).

## The green state (the rule that makes a task runnable)

- **One step = one commit = one green state.** After the step, `go build`
  compiles and the suite passes. A step that leaves the tree red is a wrong
  step.
- **The signature change travels with its caller.** The step that changes a
  type or signature adapts every call site in the same commit. The call
  sites come from the code and the grep (the map is group-level).
- **The intermediate state is declared.** The type changes in the step of
  its consumer. The earlier steps keep the old type and say so: "Of still
  returns `[][]string` until STEP 05".
- **Tests are declared, not discovered.** The task names every test that
  dies, changes shape, moves or is added — a new test file names the
  intention it asserts (one file, one intention, like the code files):
  "rewrite `TestPaths*` (the flat assertions become tree assertions)" ·
  "delete `TestX` — re-specified in STEP N" · "add `core/reads/attempts_test.go`
  (`displayAttempts` · the minimum 1)".
- **The test is real-stack.** The tests of a step run on the booted core
  (SQLite · the `testworkflows` fixtures): the state a test needs is a run
  the step names (an enqueue, a cancel, the `err` mode). A task that writes
  a stub or a fake is a wrong step.
- **The author's check (per step):** for each symbol whose signature
  changes, every call site is either in the step's `Files`, or unchanged
  (the type is still the old one), or a declared "keep" that compiles with
  the new type. If the check fails, move the type change to the consumer's
  step.

## Bugs and divergences (`bug.md`)

A bug is the behavior affecting something it does not name. When the
authoring meets a bug — a divergence (map, diagram, code or runtime
disagree) or a test error the task does not declare — the author writes
`bug.md` (repo root) and **stops the set**: the user reads it and the
process starts over (a new WANT to `agent-diagram.md`). The title is
`# bug — <the wrong behavior in one line>` — with the three parts:

- **The problem** — the wrong behavior as observed: the failing test, the
  wrong row or the wrong state, with the evidence (the output, the row
  dump, the source line).
- **The cause** — the code path or SDK behavior that produces it (file ·
  function · version); when doc and implementation disagree, the
  disagreement is the cause.
- **The touch** — what was changed to create the behavior: the commit,
  the step or the SDK version that introduced it, and why it never showed
  before.

## Template (every file, same sections, same order)

````markdown
# STEP NN — <title>

**Source (the HTML parts this task reads):**
- `<diagram>.html` → <panel · list · card · lane>.

**Files (the whole commit):**
- Add `core/graph/block.go` (`BlockType` · `Block` · `Child`).
- Change `core/graph/graph.go` (`PathGraph` · `Of`).
- Add `core/reads/attempts_test.go` (`displayAttempts` · the minimum 1).

## Purpose

<What changes from today to after. Two to four sentences.>

## Do

1. Write the test first (red): <the failing test>.
2. <One action, one line.>
N. Make the tests pass (green).

## Green (definition of done)

- `go build ./...` + the booted suite green (the env of `.env.test`) · the
  e2e when the step touched the loop.
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
  (routing lives in `index.md`).
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
  always the promotion**: promote every row of `tasks/fact.md` into the
  folder page it belongs to (via `index.md`), refresh
  `diagrams/package-map.html` only when the set touched a frontier or a map
  row (a node, an edge, a fan-in, a suite, a gate) — a set that moved
  nothing there leaves the map — and delete the set's ephemeral files (the
  improve diagrams + `tasks/`) — written as the last task file.
- **Terms** — the vocabulary, one definition per term. A term is never
  redefined in a task.
- **Source map** — where each piece of information lives (HTML · part).
- **Rules** — the constraints that hold for every step. A rule that applies
  to every step lives here, not in the tasks.

## What the author does not do

- It does not leave the impact analysis to the executor: the author names
  the surface, the executor verifies it (agent-task.md, rule 5).
- One step, one file.
- The `Source` cites only the diagram parts the author opened.
- It does not read `code.md`: the map and the code carry the layout.
