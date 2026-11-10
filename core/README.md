# core

The DBOS side of FlowDeck. The `Port` is the single contract the web talks to; the web never touches DBOS/Postgres (forbidden import, enforced by the gate).

## Rules (decisions · pinned values)

- The `Port` (the four embedded contracts: `Catalog` — `List · Graph · Spec` · `Runs` — `Status · Steps · Detail · StepDetail · Executions` · `Dispatch` — `Enqueue · Resume · Cancel` · `LiveBoard` — `Counts · Living · Queued · Failed`) is the only domain contact between web and core. Core health is a different contact: `boot.Ready`. *(evidence: `core/port.go · Port` · `NewPort`)*
- `Catalog.Graph()` resolves `boot.Paths()` per call — the graph follows boot: after boot the possible-paths graph, before boot (and after shutdown) the empty graph (never nil). *(evidence: `core/port.go · catalogAdapter · Graph` · `core/boot/lifecycle.go · Paths`)*
- The Port is the contract: changing it changes every consumer (fan-in — the map's `#frontiers` counts them). *(evidence: `core/port.go · Port` · `diagrams/package-map.html#frontiers`)*
- The read side (`reads` · `board`) observes durable state only — nothing is executed there; the actions (`actions`) are the only mutations. *(evidence: `core/reads/status.go · Status` · `core/board/live.go · LiveCounts` · `core/actions/dispatch.go · Enqueue`)*
