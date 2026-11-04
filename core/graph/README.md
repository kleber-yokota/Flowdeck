# core/graph

The possible-paths graph of the registered workflows: workflow name → the stored block model, plus the parameter spec of each workflow. Computed once at boot from the workflow source (AST parse), served by plain map lookup — no re-parse, no DB, no executor state.

## Rules (decisions · pinned values)

- The source is the single source of truth for the possible paths and the specs: `New` walks the whole FS tree (subpackages included, test files excluded) and parses every `.go` file. *(evidence: `graph.go · New`)*
- Read-only after the build: the web holds the graph through the Port and only reads it (`Of` · `Specs`). *(evidence: `graph.go · PathGraph` · `Of`)*
- The inputs are parsed from the signature: the struct input (the fields of the first non-context parameter, declared in the package) or the single simple parameter. `Context` and the blank `_` parameter (the Go SDK's required input argument) are not inputs. *(evidence: `spec.go · inputsOf` · `paramInput`)*
- Each input carries its default: the package constant `default<NamePascal>` (e.g. `start_date` → `defaultStartDate`); a step's default is the literal the source passes to the step call — a step that takes the workflow's input has no default. *(evidence: `spec.go · paramInput` · `Pascal` · `StepSpec`)*
- `Sanitize` maps raw form values onto the clean input: no extra fields, each value trimmed and converted to the input's type (only `string` and `int` are understood), missing values fall back to the defaults; a nil values map is the zero payload. `ErrInvalidInput` is raised as a 400 by the web. *(evidence: `sanitize.go · Sanitize` · `convert` · `web/executions/dispatch.go · handleRunDispatch`)*
- Independent of the DBOS core: no Postgres, no executor state — the graph is built from source alone. *(evidence: `graph.go · New` · `cmd/svgdump/main.go`)*
