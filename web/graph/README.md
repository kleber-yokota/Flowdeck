# web/graph

The SVG of the two graphs: the executed path (`ExecutedPathToSVG` — the steps that ran, grouped) and the possible-paths block diagram (`BlockToSVG` — the block DAG of the graph). Pure layout: no DB, no runtime state.

## Rules (decisions · pinned values)

- The grouping pass (`groupPath`): consecutive rows repeating the step name are a **loop group** (one box, ×N, loop arc); consecutive rows whose `[StartedAt, CompletedAt]` intervals overlap are a **parallel group** (the previous box forks the invisible group node — the continuation hangs off the last member; a parallel group at the start of the path is the root); the **wait rows** are the continuation of the steps they wait on (never parallel members — the fanout's select hangs below the go group); a parallel group of more than 4 members **collapses** into one annotated box (×N, the full member list in the box's `<title>`). *(evidence: `graph.go · groupPath` · `buildRow` · `loopGroupEnd` · `parallelGroupEnd`)*
- The layout is top-down: depth → y, the leaves → columns, a parent centered on its children; the invisible group nodes consume no depth of their own. *(evidence: `layout.go · newGraphLayout` · `depthStats`)*
- The shared geometry: 16px monospace text, char width 11px, inner padding 14px, every box 48px tall, column gap 44px, row gap 24px, the child/wait symbol zone 20px. *(evidence: `metrics.go`)*
- Both SVGs are emitted at the 1.25× readability scale (the canvas is the content canvas × 1.25, everything inside the scale group). *(evidence: `block_svg.go · BlockToSVG` · `block_test.go · TestBlockToSVGScale` · `graph_test.go · TestExecutedPathToSVGScale`)*
- The executed path carries the state colors (success/error); the possible-paths map has none (no step has run). *(evidence: `graph.go · pathNode` · `state.go`)*
