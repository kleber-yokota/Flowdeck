# index — the page index (read this first)

Navigation, not knowledge: one row per page. The `covers` column is the keyword set — open only the pages whose covers intersect your touch set. This index holds **no facts** (no values, no rules): the facts live in the pages, each with `Evidence` and `Validated`. No page = nothing to know about that folder.

## pages

| page | covers |
|---|---|
| `core/README.md` | the Port (the contract) · fan-in · the web never touches DBOS |
| `core/actions/README.md` | Enqueue · Resume · Cancel · the guards · the single queue · the ACK · recovery attempts · rewind cut · `WithMaxRecoveryAttempts` · `recovery_attempts` |
| `core/boot/README.md` | Launch · Shutdown · Ready · `ErrNotLaunched` · the explicit Boot · `SetWorkflowsSource` · `warnUnregistered` · the paths |
| `core/registry/README.md` | List (the registered workflows) · Spec · `SpecOf` · `ErrUnknownWorkflow` · the empty state |
| `core/reads/README.md` | Executions · Status · Steps · Detail · MapState · the attempts mapping · the wait-row names · the child rows · `ErrUnknownExecution` |
| `core/status/README.md` | the five states · the terminal rule · the badge colors |
| `core/graph/README.md` | the possible-paths graph · the block model · the spec from source · the default constants · Sanitize · `ErrInvalidInput` |
| `core/internal/testcore/README.md` | the test setup · `BootTest` · `PollState` · `MustStepNames` · the per-test boot |
| `core/testworkflows/README.md` | the test-workflows fixtures · `RegisterTestWorkflows` · the registration lists · the payloads of `live-test` · the block step |
| `web/README.md` | the router · healthz · the middlewares · the static assets · the Port + the ready seam |
| `web/catalog/README.md` | the dashboard · the workflow pages · the spec form · the possible-paths SVG · the empty state |
| `web/executions/README.md` | the dispatch · the badge · the detail · the steps · the resume · the cancel · the poll · the attempts display · `retriesFor` |
| `web/graph/README.md` | the SVG · the grouping (loop · parallel · collapse) · the layout · the geometry · the scale |
| `web/render/README.md` | the templates · Page vs Partial · Fail (the error→status map) |
| `web/internal/webtest/README.md` | the HTTP helpers · `Dispatch` · `Ack` · `Detail` |

## structure

- Boundaries and contacts (who imports whom, at the group level): `diagrams/package-map.html` (the boundary map)
- The green baseline (the suites) and the structural gates: `diagrams/package-map.html#suites` · `#gates`
- The agent briefs: `agent-task.md` · `agent-write-task.md` · `agent-diagram.md` · `AGENT.md` (the markdown rule)

## scope

The pages cover `core/` and `web/` (and their subfolders). The entry group (`main` · `workflows` · `cmd/` · the dbos surface) has no pages yet — the `package-map.html` boundary map is read for it until its pages exist. A folder that gains something to say (a rule, a fact, a gate) gets a page in that same change.
