# web/graph/block

The possible-paths block-DAG SVG emitter: `BlockToSVG` draws the block DAG of the graph (the paths a run can take), laid out by the block's nesting, no state colors (no step has run). Imports the shared geometry and svg emission from `web/graph`. Pure layout: no DB, no runtime state.

## Rules (decisions · pinned values)

- The block DAG is laid out by the block's nesting: a sequence is a column, a fork is a row of branches, a loop is an outline, and the root (L0) outline clears the inner outlines. *(evidence: `block_layout.go · layoutColumn` · `layoutIf` · `block.go · blockLayout`)*
- The possible-paths map has no state colors (no step has run) — the executed path is the only emitter that carries them. *(evidence: `block_svg.go · BlockToSVG`)*
