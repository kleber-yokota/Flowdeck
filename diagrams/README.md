# diagrams/

The map the agents read. One HTML per view + CSS (no images, no JS — static, greppable, printable).

## the maps

| file | view |
|---|---|
| `package-map.html` | The **boundary map**: who imports whom, at the group level — the frontiers · the groups · the fan-in · the suites (the green baseline) · the structural gates. The `core`/`web` nodes link to their folder pages. |
| `sequence-agent-flow.html` | The **agent pipeline**: the WANT → diagrams → tasks → green flow, plus the bug loop (read it before the agents). |

`css/` holds the CSS. The maps are navigation + contact (the folder pages: the facts and the rules, via `index.md`).

The improvement specs (ephemeral — deleted after implementation) live in `improve/` with their own README (`improve/README.md`) — not in this file.

## contracts

- **Boundary, group-level.** `package-map.html` shows the packages that cross a group border. Fan-in is counted at the group level (a node = a group of packages) — the `workflows` node fans out to its subpackages.
- **No facts in the map.** Facts and rules live in the folder pages (each `Rule`/`Fact` has `Evidence` + `Validated`); `index.md` is the page index. The map's `#frontiers` is the group-level contact list, `#suites` the green baseline, `#gates` the structural checks.
- **Clickable nodes** link to their folder page (`core/README.md` · `web/README.md`); the entry nodes have no page yet.
- The `pages` gate keeps the folder pages honest: a folder with code but no `README.md` is a failure.
- Generated from the code with the `diagram-design` skill (the `flowdeck` profile); validated with `self_check.py`.
