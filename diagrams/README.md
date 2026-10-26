# FlowDeck — Diagrams

Style: dark metallic skin (background `#0b0c0e`, steel borders, orange highlight) with the status colors
`queued` (light blue), `running` (orange), `success` (green) and `failed` (red).

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

## Package map (navigable)

- Ranked package graph (7 nodes, 9 edges, zero cycles): `main` → `web` → `core` + `web/graph`
  → `core/status` + `core/graph` + `dbos` (third); edge labels = the functions that
  contact across modules;
- `core` is the convergence point (fan-in 3: `main`, `web`, `web/graph`); the `web`↔`core`
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

## Files

- `css/` — one stylesheet per HTML file (`css/<name>.css`; `package-map.css` is exclusive)
- `architecture.html`
- `workflow-states.html`
- `package-map.html`
- `sequence-phase-1.html` (listing)
- `sequence-phase-2.html` (dispatch, run, track)
- `sequence-phase-3.html` (runs of a workflow)
- `sequence-phase-4.html` (paths graph)
