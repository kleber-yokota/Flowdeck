# web/graph/executed

The executed-path SVG emitter: `ExecutedPathToSVG` draws the steps that ran, grouped and top-down, each box carrying its state color. Imports the shared geometry and svg emission from `web/graph`. Pure layout: no DB, no runtime state.

## Rules (decisions · pinned values)

- The grouping pass (`groupPath`): consecutive rows repeating the step name are a **loop group** (one box, ×N, loop arc); consecutive rows whose `[StartedAt, CompletedAt]` intervals overlap are a **parallel group** (the previous box forks the invisible group node — the continuation hangs off the last member; a parallel group at the start of the path is the root); the **wait rows** are the continuation of the steps they wait on (never parallel members — the fanout's select hangs below the go group); a parallel group of more than 4 members **collapses** into one annotated box (×N, the full member list in the box's `<title>`). *(evidence: `graph.go · groupPath` · `buildRow` · `loopGroupEnd` · `parallelGroupEnd`)*
- The layout is top-down: depth → y, the leaves → columns, a parent centered on its children; the invisible group nodes consume no depth of their own. *(evidence: `layout.go · newGraphLayout` · `depthStats`)*
- The executed path is the only emitter with state colors (success/error) on the boxes. *(evidence: `layout_svg.go · the state class` · `graph.go · pathNode`)*
