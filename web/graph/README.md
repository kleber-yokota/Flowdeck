# web/graph

The shared geometry and svg emission of the two graph emitters. The metrics (the paddings, gaps, char width and box widths) and the svg (one scale, one arrow marker, the number format) live here, imported by the two emitters: `executed/` (the executed path) and `block/` (the possible-paths block DAG). Pure layout: no DB, no runtime state.

## Rules (decisions · pinned values)

- The shared geometry: 16px monospace text, char width 11px, inner padding 14px, every box 48px tall, column gap 44px, row gap 24px, the child/wait symbol zone 20px. *(evidence: `metrics.go`)*
- Both emitters emit at the 1.25× readability scale (the canvas is the content canvas × 1.25, everything inside the scale group). *(evidence: `svg.go · GraphScale`)*
- The arrow marker is written once here and referenced by both emitters' edge lines via `marker-end`. *(evidence: `svg.go · ArrowDefs`)*
