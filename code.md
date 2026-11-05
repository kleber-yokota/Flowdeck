# code.md — the code is a change index

The code is planned against the *changes*, not the *domain*: folders and
files are a **change index** — the layout that makes each planned change
cheap, nameable, and verifiable.

## The load path (index → page → files)

The index is a load path: how an agent — or a human — gets from "I'm
changing X" to the exact files, loading only what the change needs.

1. **File = one intention** (principle 2). Named for the behavior it owns.
2. **Folder = a set of intentions** (principle 3).
3. **Page = the folder's `README.md`.** The loadable unit for that set:
   its rules and validated facts, each with `Evidence` and `Validated`.
4. **Index = `index.md`.** The retrieval index (the RAG): read first, open
   only the pages whose `covers` intersect the touch set, load only those
   pages + the files they name. It holds no facts — navigation, not
   knowledge.

The load is proportional to the task: index (small) → page(s) → files,
never the whole repo. The agent and the human share the path — a folder
name and its page say what that part does, with no archaeology.

The path only holds because of the intention discipline (principles 2 · 3):
a file with two intentions can't be loaded in isolation, and a folder with
unrelated things can't be routed to.

**The separation metric (routing precision of the page index).** A folder
is a page; `index.md` routes a task to the pages whose `covers` intersect
its touch set. A folder is correctly separated when that routing is
precise: its `covers` set names one set of intentions, and no backticked
`covers` key appears in two pages. Two signals drive the split — the page
is **too broad** (`covers` no longer names one set: a task touching part
of the folder loads the rest) → split into subfolders, each naming one
set; or the routing is **ambiguous** (a backticked `covers` key appears in
two pages) → move the intention to the one page that owns it. The load-cost
guard is the folder twin of the 300-line rule: a page must be loadable in
isolation. The metric says *when* to split; the grain (principle 3) says
*where*. Measured by `scripts/page-index-metric.py`.

## Principles

1. **Packages are contracts, files are the working unit.**
   Packages (modules imported by others) are drawn once, kept few, kept
   cycle-free. Every package edge is a permanent tax: it is read by every
   future change and by every doc line that describes it. Files inside a
   package are rearranged for free — a combine or a split changes nothing
   outside the package. Split files freely; create packages reluctantly —
   a refactor may still draw, move, or redraw one (step 6). A package
   split into subfolders groups them by purpose — the conversation each
   subfolder owns (an execution, a catalog, a boot) — never by technical
   role (controller, service, model): the top-level package is the
   technical boundary (HTTP, DB, SVG), the subfolder is the index of the
   purpose.
   The rule descends to the file level: references between files are
   unidirectional against a named owner — the shared thing (state, data,
   model) lives in the one file that owns it, and the other files reference
   the owner. A mutual reference between two files is debt in waiting, not
   a layout: it is broken by extracting the shared part to its owner
   (step 6), and it blocks the package draw the pair would otherwise
   receive (step 1).

2. **One file, one responsibility, named for the behavior it owns.**
   Files are named for the thing they own (the handler, the entity, the
   state, the command). The test is named for the intention it
   asserts (*The tests*). "Where is X?" must be answerable from name +
   index, without a search.

3. **A folder is a set of intentions.**
   A file owns one intention (principle 2). Files whose intentions are
   about the same thing form a *set*, and a folder, subfolder, or package
   is the name of the set it owns. Two grains of draw, both a structure
   change (step 6) and a design decision the run never invents: a
   **subfolder** is drawn when a package's files have grown into more than
   one set of intentions — it names the set it owns; a **package** (a root
   folder) appears when a second consumer of the contract appears (step 1).
4. **One recipe per entity kind.**
   Every entity of the same kind (endpoint, workflow, migration, tool) has
   exactly one layout: where the file goes, how the test looks, the single
   wiring point (router, registry, generated file). Two recipes = a decision
   every change of that kind re-justifies.

5. **The map moves with the code.**
   The project keeps a boundary map (frontiers · groups · fan-in · suites ·
   gates) and the folder pages (the rules and the verified facts, indexed by
   `index.md`). The map is refreshed *by* the change that moved a frontier
   (or a map row), as its last step — never as a separate chore, and a
   change that moved nothing there leaves the map untouched. A map
   maintained apart from the code is a map that lies. The map's SVG is a
   diagram: an update that touches the drawing
   (nodes, edges, labels, layout) loads the `diagram-design` skill first and
   lets it guide the edit.

6. **Changes are sequences of green steps.**
   Each step leaves the system compiling and tested. Small files are what
   make small steps possible. Prefer a move over an in-place edit: when
   moving to a smaller surface simplifies the change, the move is its own
   step — a pure move is trivially green.

7. **The reader is the same as the writer.**
   The read path is planned like the write path: index → named files →
   nothing else. A clean-context actor (a fresh agent session, a new hire)
   must be able to start a change from the structure alone. If finding
   "where to start" needs archaeology, the layout failed.

## The tests (one file, one intention)

The test file asserts **one intention**, and is named for that intention —
never for the production file it lives next to. The map row declares the
intention ("asserts X · Y"). The mirror is at the *intention* level, not the
file level: a source file with N behaviors has N test files (`spec.go` →
`default_test.go` + `sanitize_test.go`). A parser that also extracts the
spec is two intentions: the spec tests live in the spec's files.

| Kind | What it proves | Named for | Lives as long as |
| --- | --- | --- | --- |
| Unit | one function, in isolation (no env, no core) | the behavior: `sanitize_test.go` | the function |
| Contract | the package's public behavior (env-booted, via the public surface) | the package: `core_test.go` | the contract |
| E2E | the full loop (build tag, the real DB) | `e2e_test.go` | the loop |
| Harness | the fixtures: the `TestMain` boot · the `testworkflows` fixtures · the HTTP helpers | the package: `web_test.go` | the contract it mirrors |
| Spike | one runtime value no code shows | `zz_spike_test.go` (temporary) | the commit — deleted, promoted to a fact |
| Fact-pin | a pinned runtime behavior (an SDK fact) | the behavior: `TestResumeOfFailedRun` | the fact — only a re-validation changes it |

Rules:

- **The harness is the test's boot** — the `TestMain` per package (the
  `testworkflows` registration · the workflows source) and the shared
  helpers (`web/internal/webtest`). A harness change is a cross-cutting
  change: it is named in every task that touches the contract it mirrors.
- **The test runs on the real stack** — no mock: the per-behavior tests run
  on the booted core (SQLite · the `testworkflows` fixtures); the state a
  test needs is a real run (an enqueue, a cancel, the `err` mode). The
  not-launched port (`core.NewPort()`) proves the core-down path.
- **Negative space.** The set of states a contract must never hold. The
  term is from the art: the empty space around a figure pins the figure's
  shape — the negative defines the positive. In code the same: a contract
  is pinned by the states it must hold *and* the states it must never hold.
- **The contract pins both spaces.** A contract test asserts the contract's
  whole space: the positive (each state that rows · the multi-row case, no
  dedup) and the negative (each state that must never row — the empty case,
  the queued run, the terminal run). A negative assertion is as strict as a
  positive one (the empty case is `len(rows) == 0`, the count is the exact
  row count), and every pinned space is a fact row.
- **Derive the space (the variations).** For each declared test, ask the
  space before writing: more than one running (the second run · another
  workflow · no dedup), none (the empty case), and each state that must
  never row (the negative space). The variations are tests of the same
  intention — the same test file — each pinned as a fact row.
- **A step adds a test file for a new intention** — one file, one
  intention, like the code files; a test file that outgrew its intention
  splits, as its own green step.
- **The test is declared, not discovered** — a task names every test that
  dies, changes shape, moves or is added, with its kind. A red test outside
  a task's declared surface is a bug, not a discovery.
- **The fact-pin changes only with the fact** — its evidence is a `Fact`
  row of the folder page; the diff-check (evidence files, SDK version)
  decides re-validation, not taste.

## The intentions

| Intention | Mechanism that delivers it |
| --- | --- |
| The cost of a change is proportional to its size | file granularity + green steps: a small change touches a small, nameable surface |
| Navigation is a lookup, not a search | routing index + behavior naming + uniform recipes |
| Docs cannot drift from the code | the map is a byproduct of the change, at the same granularity as the files |
| Errors stay local | one responsibility per file + test mirror: a red test or a missed call site names exactly one file |
| A contract bends only where the change repairs it | every draw and break is named for the intention it serves, repaired at every break point in the same green sequence |
| The N+1th change costs the same as the Nth | recipes amortize: the layout decision is made once, at the kind level |
| Structure replaces memory | a clean-context actor acts correctly from the index — no history, no tribal knowledge |

The meta-intention: the layout is the first line of the change budget — the
way tests are the first line of the correctness budget.

## The decision procedure (per change)

1. Which package owns this behavior? If none, that is a design decision —
   stop and decide it; do not invent a folder. Name the module split, don't
   infer it: a new package appears when a second consumer of the contract
   appears (a new import edge with its own needs), not when a file holds two
   responsibilities — intent splits are file splits (principle 1 · one file,
   one intention · the 300-line rule). Before any draw, read the reference
   DAG of the cluster: a pair of files that references each other is not
   drawable — the cycle break (step 6) is the preceding green step, and the
   draw is the one that follows it.
2. Does an existing file own it? Extend the file.
3. No file owns it? Create a file named for the behavior + its test.
4. A file now holds two responsibilities? Split it — as its own green step.
5. Does the index know where this lives? Update it — as the last step of the
   same change.
6. An explicit refactor is a structure change: it combines and splits
   files, breaks mutual file references (the shared part is extracted to
   its owner, so references stay unidirectional), draws or moves folders
   and subfolders (packages), redraws or moves boundaries (a package edge,
   the Port surface, a read model), and improves performance (a batched
   query, a state that rides on the read).
   Its reasons, and only these: performance · the ease of the code (it
   reads easier, it changes easier) · the intention (every file is
   oriented to the intention it owns — a boundary out of step with the
   intention is redrawn). The rules of the structure change: each draw,
   break, and move names the intention it serves, and every break point —
   call sites, tests, the map — is repaired in the same green sequence
   (principle 6). The goal is the improvement, not the move: a break left
   unfixed is not green, it is debt. A change that is *not* about structure
   adds no package, no folder layer, no naming convention.

## What this forbids

- Package sprawl (a permanent tax per edge, paid without a named
  intention).
- A break without its repair (a contract moved, and a call site, a
  test, or a map row left at the old address).
- Catch-all layer folders (`utils/`, `misc/`, `helpers/`).
- Bucket files (N unrelated things in one file).
- A package drawn over a cycle (the mutual pair breaks first — step 1).
- Doc updates as a separate chore.
- Two layout recipes for one kind of entity.
- A test file that asserts two intentions (the parser and the spec in one
  file).
- A Go file over 300 lines (a file this long has outgrown one responsibility
  — split it).

## What this is not

This is **behavior-first**, not layer-first. A file is not named for its
technical role (controller, service, model) — it is named for the behavior it
owns. The technical grouping (HTTP, DB, SVG, state) still exists — it is the
*package* boundary, drawn once and kept small. The layer name is the reader's
*filter*, not the writer's *address*: in a layer layout, "where is X" is a
grep; in a behavior layout, it is a name.

## Where this lives in this repo

| Piece | Where |
| --- | --- |
| The map (frontiers · groups · fan-in · suites · gates) | `diagrams/package-map.html` |
| The verified facts and rules | the folder `README.md` pages · the index: `index.md` |
| The markdown rule (writing the pages) | `AGENT.md` (the `writing-great-skills` skill applied to the edit) |
| The routing index (task → where to start) | `index.md` (one row per page · the `covers` keywords) |
| The green steps | `tasks/*.md` (one step = one commit = one green state) |
| The last-step promotion | the last task of every set (facts into the folder pages · the map refreshes only when a frontier or a map row moved · the SVG goes through the `diagram-design` skill) |
| The clean-context actors | `agent-diagram.md` · `agent-write-task.md` · `agent-task.md` |
| The uniform recipe (workflow) | `workflows/<name>/` + `go generate ./workflows` |
| The load lines (complexity · nesting · cycles · import contract) | `scripts/structure-gate.sh` + `.goimportmaps.yaml` |
