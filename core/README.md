# core

The DBOS side of FlowDeck. The `Port` is the single contract the web talks to; the web never touches DBOS/Postgres (forbidden import, enforced by the gate).

## Rules (decisions · pinned values)

- The `Port` (13 function fields: `List · Graph · Spec · Executions · Enqueue · Status · Steps · Detail · Resume · Cancel · Live · Living · Queued`) is the only domain contact between web and core. Core health is a different contact: `boot.Ready`. *(evidence: `core/port.go · Port` · `NewPort`)*
- `NewPort` captures the possible-paths graph at the call — wire the port after boot; before it the port carries the empty graph (never nil). *(evidence: `core/port.go · NewPort`)*
- The Port is the contract: changing it changes every consumer (fan-in — the map's `#frontiers` counts them). *(evidence: `core/port.go · Port` · `diagrams/package-map.html#frontiers`)*
- The read side (`reads`) observes durable state only — nothing is executed there; the actions (`actions`) are the only mutations. *(evidence: `core/reads/status.go · Status` · `core/actions/dispatch.go · Enqueue`)*
