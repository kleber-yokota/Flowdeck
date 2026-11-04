# review.md — the readings of the structure signals

`scripts/structure-gate.sh` prints the live numbers (the gates and the
signals). This file holds what the numbers cannot say: what is **ignored**
in the analysis, and how to **read** a number. It is guidance, not a
snapshot: the numbers stay where they are printed. The agent runs the
script, reads the live output against these readings — the signal points,
the agent decides.

## Ignored (the signal does not apply)

| Package | Why |
| --- | --- |
| `workflows/` + the subpackages | The uniform recipe: one file per workflow. The parent `workflows` is the registration wiring — `zz_generated.go` pulls the subpackages in. By design, not a god package. |
| `cmd/` (workflowgen · svgdump) | The build tool and the diagnostic main: one file importing the layers is structural. |
| `main` (the root package) | The boot: env → launch → port → web. Its imports are the wiring, not responsibility. |

## Readings (how to read a number)

| Shape | Reading |
| --- | --- |
| many files + few imports | The package can be split into subpackages (the folder is an index of intentions that already live apart). |
| few files + many imports | Responsibility is concentrated — the files carry more than their name says. Read the file before touching it. |
| files and imports both high | Do not stop at the ratio — check which files carry the imports. Many files with few imports next to one or two files with many imports means the folder is carrying more responsibility than its density says; the file with the many imports is where the next intention should be extracted. |

Every reading resolves in the same direction: the intention the number
points at gets drawn out — as a subpackage of the folder, or as a
package of its own.
