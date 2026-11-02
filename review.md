# review.md — the readings of the structure signals

`scripts/structure-gate.sh` prints the numbers (the gates and the signals).
This file holds what the numbers cannot say: what is **ignored** in the
analysis, and how to read what is **in attention**. When the density or
coupling output changes, the agent updates the numbers here in the same
change.

## Ignored (the signal does not apply)

| Package | Why |
| --- | --- |
| `workflows/` + the subpackages | The uniform recipe: one file per workflow. The parent `workflows` (2 files × 12 imports) is the registration wiring — `zz_generated.go` pulls the subpackages in. By design, not a god package. |
| `cmd/` (workflowgen · svgdump) | The build tool and the diagnostic main: one file importing the layers is structural. |
| `main` (the root package) | The boot: env → launch → port → web. Its imports are the wiring, not responsibility. |

## In attention (read the numbers)

| Package | Numbers (baseline 2026-11-02) | The reading |
| --- | --- | --- |
| `core` | 12 files × 15 imports | The density ratio is balanced, but both sides are high: responsibility is spread over the folder. The skew is the tell — `lifecycle.go` (10 imports) and `detail.go` (7) carry most of the fan-out; the other ten files carry 4 or fewer. |
| `web` | 12 files × 12 imports | The same shape: `render.go` (6) · `workflow.go` (5) · `web.go` (5) against the rest at 1–4. |

The rule: when files and imports are both high, do not stop at the ratio —
check which files carry the imports. Many files with few imports next to
one or two files with many imports means the folder is carrying more
responsibility than its density says; the file with the many imports is
where the next intention should be extracted.
