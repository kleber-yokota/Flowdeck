# core/graph

The possible-paths graph of the registered workflows: workflow name → the stored block model, plus the parameter spec of each workflow. Computed once at boot from the workflow source (AST parse), served by plain map lookup — no re-parse, no DB, no executor state.

## Rules (decisions · pinned values)

- The source is the single source of truth for the possible paths and the specs: `New` walks the whole FS tree (subpackages included, test files excluded) and parses every `.go` file. *(evidence: `graph.go · New`)*
- Read-only after the build: the web holds the graph through the Port and only reads it (`Of` · `Specs`). *(evidence: `graph.go · PathGraph` · `Of`)*
- The boot map carries the source file of each registered workflow (`Files`: name → the file path as walked by `New`, subpackage paths kept as `dir/file`) — the page's breadcrumb. *(evidence: `graph.go · PathGraph.Files` · `parse.go · pathsFromSources`)*
- The inputs are parsed from the signature: the struct input (the fields of the first non-context parameter, declared in the package) or the single simple parameter. `Context` and the blank `_` parameter (the Go SDK's required input argument) are not inputs. *(evidence: `spec.go · inputsOf` · `paramInput`)*
- Each input carries its default: the package constant `default<NamePascal>` (e.g. `start_date` → `defaultStartDate`); a step's default is the literal the source passes to the step call — a step that takes the workflow's input has no default. *(evidence: `spec.go · paramInput` · `Pascal` · `StepSpec`)*
- `Sanitize` maps raw form values onto the clean input: no extra fields, each value trimmed and converted to the input's type (only `string` and `int` are understood), missing values fall back to the defaults; a nil values map is the zero payload. `ErrInvalidInput` is raised as a 400 by the web. *(evidence: `sanitize.go · Sanitize` · `convert` · `web/executions/dispatch.go · handleRunDispatch`)*
- Independent of the DBOS core: no Postgres, no executor state — the graph is built from source alone. *(evidence: `graph.go · New` · `cmd/svgdump/main.go`)*
## Facts (validated runtime)
| Fact | Evidence | Validated |
|---|---|---|
| `TestGraphBuild` is the graph-build gate's test — it builds the graph from the production workflows tree on disk (no boot, no DB) and asserts the parse covers the spec, the steps, the child, the loop and the wait. | `build_test.go · TestGraphBuild` · `diagrams/package-map.html#gates` | 2026-11-09 |
| A production tree can have no step (`RunAsStep`) leaves: the fanout tree's leaves are the asyncs (`alpha` · `beta`) and the wait (`select`) — the read-back is one path (`alpha` · `beta` · `select`). "At least one step node" holds only as a leaf of any kind: the tree's `Steps` are all its leaves. | `workflows/fanout/fanout.go` (no `RunAsStep`) · `paths.go · stepsOf` (the leaf node names, in build order) | 2026-11-09 · the core AST parse (no SDK) |
