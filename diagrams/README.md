# FlowDeck — Diagrams

Style: dark metallic skin (background `#0b0c0e`, steel borders, orange highlight) with the status colors
`queued` (light blue), `running` (orange), `success` (green) and `failed` (red).

**Reading rule**: open only the HTML file(s) your task actually needs — route via the phase sections below
or `package-map.html`; do not read the other `.html` files without a reason or unless explicitly asked.

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

## Agent process (the pipeline)

- How this project is built: `sequence-agent-flow.html` — WANT (the user's input) → agent-diagram
  writes `improve/*.html` (the user validates; a previous `bug.md` is deleted) → agent-write-task writes
  `tasks/*.md` + `tasks/fact.md` → agent-task implements red → green, in order → green deletes the
  ephemeral files; a bug (an existing test failing outside the task surface) writes `bug.md` and stops —
  the user reads it and starts the process over;
- Every agent enters with a **clean context**: only the handoff files, in their current state — no git
  tree, no history (machine view: `#handoffs` · `#artifacts` · `#rules` · `#terms` of the same file).

## Package map (navigable)

- Ranked package graph (10 nodes, 17 edges, zero cycles): `main` → `web` → `core` + `web/graph`
  → `core/status` + `core/graph` + `dbos` (third); edge labels = the functions that
  contact across modules; `cmd/workflowgen` (build) and `cmd/svgdump` (diagnosis) are
  main packages that are never imported;
- `core` is the convergence point (fan-in 4: `main`, `web`, `web/graph`, `workflows`); the `web`↔`core`
  contact is the single `Port`; `core/status` and `core/graph` are pure;
- **Clickable nodes**: clicking a package shows its files, functions and contacts
  — navigate without reading the code;
- Navigation without JavaScript (anchors + `:target`): deep-link by hash, keyboard and
  back button work; each detail section has drill-down links to the contact packages.
- **One file, two readers**: the `#contacts` section (where the modules touch: per edge,
  file/function + identifiers) and the `<section id="package-*">` blocks
  (file | responsibility | functions) are the machine-readable map (AGENTS.md points to
  them); the SVG is the human view of the same data. No separate map exists — if you
  update it, it is here.

## Graph subsystem (deep dive)

One folder — `graph/current/` = the state of the code today (the block model is built and
stored at boot; `web/graph` renders it):

- `graph/current/graph-flow.html` — data flow: who owns what in the graph subsystem and how the
  data moves (embedded source → `core/graph` parse → `PathGraph` → `Port` → `web/graph`
  render, durable steps from Postgres feeding the executed path);
- `graph/current/graph-contract.html` — the two path-diagram pipelines, reflecting the current code:
  the block-model map (boot · AST: `core/graph.New(fsys)` → `Port.Graph` →
  `handleWorkflowPage` → `BlockToSVG` → workflow page) and the executed path (runtime ·
  Postgres: `core.Steps` → `Port.Steps` → `handleExecutionSteps` (2s poll) /
  `handleExecutionDetail` → `ExecutedPathToSVG` → steps fragment), both emitted at 1.25×
  through the shared SVG machinery (`num` · the marker defs · `<g transform="scale(1.25)">`);
- `graph/current/executed-path-flow.html` — the executed-path (runtime · Postgres) data flow:
  the execution page and its 2s poll (`GET /executions/{id}` · `GET /executions/{id}/steps`)
  go through the web handlers into the core Port (`Steps` · `Detail` · `Status`), the core reads
  the status row and the step rows from Postgres (step_id asc, `row.Error` mapped to failed) and
  hands the durable steps to `web/graph.ExecutedPathToSVG`, which renders the executed-path SVG
  into the steps fragment that HTMX swaps every 2s until the run is terminal;
- `graph/current/executed-path-dependencies.html` — how the executed-path dependencies are
  generated: the Postgres step rows (step_id asc) → `core.Steps` maps each row to a
  `Step{Name, State}` (`row.Error != nil` → failed, else success) → `ExecutedPathToSVG` wires the
  dependency (`nodes[i-1].Children = []pathNode{nodes[i]}` — each step → the next step that ran)
  and keeps only the first step as the single root → the shared layout (measure · place ·
  collect) emits the linear SVG (one rect and arrow per step, colored by the step state);
- `graph/current/paths-blocks.html` — taxonomy of "what's inside what": the Go syntax blocks a
  workflow body can use (the containers: block, if/else-if/else, for, for range) × the DBOS
  constructs that can be put inside any of them (the contents: `RunAsStep`,
  `RunAsTransaction`, `RunWorkflow`, `Go`, `Select`, `Sleep`, `Recv`, `Enqueue`,
  `Send`/`SetEvent`), each with its path effect (sequence / fork / loop) and the node type
  it produces (step / child workflow / async / wait / off-path) — the reference for the
  AST extraction; the parser recognizes constructs by function name, groups a run of async
  children right before a channel wait (`select`/`recv`) into a join block, and filters
  empty guard blocks;
- `graph/current/paths-block-levels.html` — the possible-paths structure as a **tree of blocks by level**: every
  block has a level (depth — root = L0, larger = more internal), a type (sequence · loop · if ·
  join) and ordered children; independent blocks share a level (siblings — two sequential loops
  are both L1); the tree is stored and read back, not recomputed; the example is a workflow with
  two sequential loops + steps, with the flattened execution order;
- `graph/current/webgraph-block-treatment.html` — the flow of how `web/graph` receives and treats the
  block model: `core/graph` produces the stored block model (blocks · levels · types) at boot;
  `web/graph` receives it, then treats it (top → bottom) — (1) receive, (2) assign the block
  level (L0/L1/L2) to each block's outline, (3) represent each block type (sequence · async ·
  if · loop · join), (4) layout (positions · arrows · symbols · nesting by level), (5) emit
  the SVG at 1.25×;
- `graph/current/webgraph-block-representation.html` — the block representation key: how each block type
  is drawn (sequence = boxes in order, async = side-by-side boxes + the ∥ mark, if/else-if/else =
  a fork whose branches arrow to the next dependency, loop = the open-arc symbol, join = the
  async group + the wait node, child workflow ◉ and wait = distinct box symbols). Two distinct
  palettes: status (badges: queued/running/success/failed) and block level (outlines: L0
  `#ddd6fe`, L1 `#a78bfa`, L2 `#7c3aed`; L3+ reuses L2); sequence flows top → bottom.

Second folder — `graph/improve/` = proposed improvements over the current flow (NEW/CHANGED
markers against the `current/` base; DBOS v1.7.0 facts: `operation_outputs` rows are written
when a step completes — no step-level running state, running is the workflow-level `PENDING`
badge — and each row already carries `child_workflow_id` + `started_at`/`completed_at` and its
SDK-internal name (`DBOS.enqueue` · `getResult` · `select` · `recv` · `sleep` — stable across
app versions, unlike the boot AST, which is only valid for the current
`application_version`); `workflow_status` carries the child's `name`, `status`, `attempts`,
`queue` via `WithFilterParentWorkflowID`):

- `graph/improve/executed-path-dependencies.html` — the improved executed-path dependency
generation (Postgres only, app-version safe): the read also fetches the child workflows
(`workflow_status` by `parent_workflow_id`); `core.Steps` keeps the fields it drops today
(`ChildWorkflowID`, `StartedAt`, `CompletedAt`) and marks the kind from the row name; the new
decision `ChildWorkflowID != ''?` splits child steps (child name + badge shown on the step)
from plain steps; async Go steps have no Postgres flag (only the boot AST could mark them,
and only when the execution's `application_version` matches); the wiring, single root and
shared layout are unchanged; the SVG shows state · child name per step.

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
- `sequence-agent-flow.html` (agent flow — the pipeline: WANT → `improve/*.html` → `tasks/*.md` → red → green; bug → `bug.md` → restart; clean context per agent)
- `improve/` — proposed improvements over the current flow (NEW/CHANGED against the current base)
  - `improve/sequence-cancel.html` (improve — cancel a run: POST /cancel/{id} → `dbos.CancelWorkflow` → CANCELLED row → the new fifth badge (cancelled · steel `#8f96a3`) via the 2s poll; mapState + CANCELLED → cancelled)
  - `improve/sequence-resume.html` (improve — resume a run: POST /resume/{id} → guard {error, cancelled} → failed via `dbos.RewindWorkflow(id, queue, cut)` (bug.md — the SDK `ResumeWorkflow` is a silent no-op on ERROR rows) · cancelled via `dbos.ResumeWorkflow` → ACK enqueued · badge back to queued · web error mapping 404/500)
  - `improve/executed-path-improvements.html` (improve — the executed path for /executions/{id}: Postgres rows → core `Steps`/`Detail` (+Children) → `groupPath` (runtime parallel by times overlap · loop by repeated name) → top-down layout with larger boxes (36→48px · font 14→16 · scale 1.25) → SVG (∥ outline · loop arc ×N · collapse >4 → one box ×N + `<title>` · child name + live badge) → UI (child id list); discovered DBOS states · row facts · app-version safety)
  - `improve/execution-detail-map.html` (improve — the run lifecycle with the fifth badge (cancelled · `#8f96a3`), the cancel/resume guards and edges (failed → rewind · cancelled → resume), the UI surface per state, and the where-applied ledger (file · function per improvement))
- `graph/` — graph subsystem deep dive (own `graph/css/`, one stylesheet per HTML file)
  - `graph/current/` — the code today (as-is)
  - `graph/improve/` — proposed improvements over the current flow (NEW/CHANGED against the current base)
    - `graph/current/graph-flow.html` (dependency and flow)
    - `graph/current/graph-contract.html` (the two path-diagram pipelines: block model + executed path)
    - `graph/current/paths-blocks.html` (Go blocks × DBOS constructs — what's inside what)
    - `graph/current/paths-block-levels.html` (possible paths as a tree of blocks by level)
    - `graph/current/webgraph-block-treatment.html` (how `web/graph` receives and treats the block model)
    - `graph/current/webgraph-block-representation.html` (how each block type is represented + the two palettes: status and block level)
    - `graph/current/executed-path-flow.html` (executed path — flow and fetch: page + 2s poll → handlers → Port → Postgres step rows → `ExecutedPathToSVG` → steps fragment)
    - `graph/current/executed-path-dependencies.html` (executed path — dependency generation: step rows → Step mapping → chain wiring → single root → layout → SVG)
  - `graph/improve/` — the proposed improvements
    - `graph/improve/executed-path-dependencies.html` (executed path (improve) — dependency generation, Postgres-only and app-version safe: + child workflows by `parent_workflow_id`, + kept row fields, kind from the SDK-internal row name)
