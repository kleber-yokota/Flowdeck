# FlowDeck — Diagrams

Style: dark metallic skin (background `#0b0c0e`, steel borders, orange highlight) with the status colors
`queued` (light blue), `running` (orange), `success` (green) and `failed` (red).

**Reading rule**: open only the HTML file(s) your task actually needs — route via the phase sections below
or `package-map.html` (`#tasks`); do not read the other `.html` files without a reason or unless explicitly asked.

## Phase 1 — Dynamic listing

- The browser requests the page; the core lists the registered workflows (registry, **zero hardcoding in the web**);
- the UI renders the list from the registry.

## Phase 2 — Dispatch, run and track

- HTMX POST dispatches a run (ACK + badge `queued` + visible `workflow_id`);
- the core enqueues in DBOS (Postgres); the DBOS executor runs the steps and updates the status;
- the front polls `GET /status` every 2s: `queued → running → success | failed` (badge colors: light blue, orange, green, red).

## Phase 3 — Runs of a workflow

- `GET /workflows/{name}` lists every execution (workflow_id + badge);
- click on the workflow_id → DETAIL: status, retries, error on failure, and the step that errored.

## Phase 4 — Paths graph (SVG)

- The core parses the workflow source at boot (AST: `RegisterWorkflow` → possible paths, including `if` branches);
- the `decide` page shows the generated SVG graph (overview) and, after dispatch, the SVG graph of the steps that ran (executed path);
- no hardcoded SVG: the parser finds the workflow and generates the full graph (steps, branches, final state).

## Phase 5 — Parameters, edit and stored input

- The core parses the workflow source at boot (AST) and extracts each workflow's
  input and the steps' default values — the source is the single source of truth;
- `GET /workflows/{name}` renders the input + defaults at the **top** of the page;
  the user edits the values and clicks dispatch (`POST /run/{name}` with the value);
  the web forwards the raw values and the **core** sanitizes them against the
  parsed spec (type, limits, default fallback) before the enqueue;
- the clean value is written to Postgres **once** at enqueue (`workflow_input`); on a
  failed run, `POST /resume/{id}` re-enqueues by id and re-reads the same stored
  input — the input per workflow_id is immutable (editing only affects new dispatches).

## Package map (navigable)

- Ranked package graph (9 nodes, 13 edges, zero cycles): `main` → `web` → `core` + `web/graph`
  → `core/status` + `core/graph` + `dbos` (third); edge labels = the functions that
  contact across modules;
- `core` is the convergence point (fan-in 4: `main`, `web`, `web/graph`, `workflows`); the `web`↔`core`
  contact is the single `Port`; `core/status` and `core/graph` are pure;
- **Clickable nodes**: clicking a package shows its files, functions and contacts
  — navigate without reading the code;
- Navigation without JavaScript (anchors + `:target`): deep-link by hash, keyboard and
  back button work; each detail section has drill-down links to the contact packages.
- **One file, two readers**: the `#tasks` section (task→files routing), the `#contacts`
  section (where the modules touch: per edge, file/function + identifiers) and the
  `<section id="package-*">` blocks (file | responsibility | functions) are the
  machine-readable map (AGENTS.md points to them); the SVG is the human view of the same
  data. No separate map exists — if you update it, it is here.

## Graph subsystem (deep dive)

- `graph/graph-flow.html` — data flow: who owns what in the graph subsystem and how the
  data moves (embedded source → `core/graph` parse → `PathGraph` → `Port` → `web/graph`
  render, durable steps from Postgres feeding the executed path);
- `graph/core-graph-extraction.html` — flowchart of how `core/graph` extracts
  (ParseFile · collect → `possiblePaths` → `specs` → `PathGraph`); the spec for improving
  the AST extraction;
- `graph/possible-paths.html` — detailed flowchart of the `possiblePaths` walk: the open
  paths list (starts with one empty path), `RunAsStep` appending to every open path, `if`
  forking taken/not-taken (cartesian), dedupe at the end — with the `decide` workflow trace;
- `graph/graph-contract.html` — the two path-diagram pipelines, reflecting the current code:
  the possible-paths map (boot · AST: `core/graph.New(fsys)` → `Port.Graph` →
  `handleWorkflowPage` → `PathsToSVG` → workflow page) and the executed path (runtime ·
  Postgres: `core.Steps` → `Port.Steps` → `handleExecutionSteps` (2s poll) /
  `handleExecutionDetail` → `ExecutedPathToSVG` → steps fragment), both sharing the same
  drawing code (`newGraphLayout` · `svg()` · `pathNode`);
- `graph/paths-blocks.html` — taxonomy of "what's inside what": the Go syntax blocks a
  workflow body can use (the containers: block, if/else-if/else, switch, type switch, for,
  for range, select, go, defer/return) × the DBOS constructs that can be put inside any of
  them (the contents: `RunAsStep`, `RunAsTransaction`, `RunWorkflow`, `Go`, `Select`,
  `Sleep`, `Recv`, `Enqueue`, `Send`/`SetEvent`), each with its path effect (sequence / fork
  / loop) and the node type it produces (step / child workflow / async / wait / off-path),
  plus today's `possiblePaths` coverage (handled / flattened / invisible) — the reference
  for extending the AST extraction beyond `RunAsStep` + `if`;
- `graph/paths-block-levels.html` — the possible-paths structure as a **tree of blocks by level**: every
  block has a level (depth — root = L0, larger = more internal), a type (sequence · loop · if) and
  ordered children; independent blocks share a level (siblings — two sequential loops are both L1);
  the tree is stored and read back, not recomputed; the example is a workflow with two sequential
  loops + steps, with the flattened execution order.
- `graph/webgraph-block-treatment.html` — the target-model flow of how `web/graph` receives and
  treats the block model: `core/graph` produces the block model (blocks · dependencies · levels ·
  types) at boot; `web/graph` receives it, then treats it (top → bottom) — (1) receive, (2) assign
  the block level (L0/L1/L2) to each block's outline, (3) represent each block type (async · if ·
  loop · sequence), (4) layout (positions · arrows · symbols · nesting by level), (5) emit the SVG;
- `graph/webgraph-block-representation.html` — the block representation key: how each block type
  is drawn (sequence = boxes in order, async = each box one below the last, if/else-if/else = a
  stack whose branches arrow to the next dependency, loop = a repeat symbol, child workflow and
  wait = distinct box symbols). Two distinct palettes: status (badges: queued/running/success/
  failed) and block level (outlines: L0 ink, L1 soft, L2 dark; L3+ reuses L2 + badge); sequence
  flows top → bottom.

## Files

- `handoff-diagrams.md` (pending adjustments discussed but not yet applied — not part of the spec)
- `css/` — one stylesheet per HTML file (`css/<name>.css`; `package-map.css` is exclusive)
- `architecture.html`
- `workflow-states.html`
- `package-map.html`
- `sequence-phase-1.html` (listing)
- `sequence-phase-2.html` (dispatch, run, track)
- `sequence-phase-3.html` (runs of a workflow)
- `sequence-phase-4.html` (paths graph)
- `sequence-phase-5.html` (parameters, edit, stored input)
- `graph/` — graph subsystem deep dive (own `graph/css/`, one stylesheet per HTML file)
  - `graph/graph-flow.html` (dependency and flow)
  - `graph/core-graph-extraction.html` (how `core/graph` extracts)
  - `graph/possible-paths.html` (the `possiblePaths` walk, step by step)
  - `graph/graph-contract.html` (the two path-diagram pipelines: possible-paths + executed path)
  - `graph/paths-blocks.html` (Go blocks × DBOS constructs — what's inside what)
  - `graph/paths-block-levels.html` (possible paths as a tree of blocks by level)
  - `graph/webgraph-block-treatment.html` (how `web/graph` receives and treats the block model)
  - `graph/webgraph-block-representation.html` (how each block type is represented + the two palettes: status and block level)
