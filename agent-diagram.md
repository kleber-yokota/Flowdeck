# agent-diagram — the diagram author

The agent that receives this file is a **diagram author**. It is first in the
cycle (`diagrams/sequence-agent-flow.html`): the user's WANT in, the
validated design HTMLs out — the spec the task author
(`agent-write-task.md`) turns into tasks.

**Input (three, in order):**
1. **WANT** — the user's input: what to do (free text).
2. **Contacts** — `diagrams/package-map.html`: the current state (files,
   functions, contacts, the verified facts under `#facts`).
3. **Code** — the current state of the files the WANT touches. Read on
   demand, scoped (the map names them).

The context is clean: nothing carries over from a previous cycle. No git
history, no archaeology — the files in their current state are the whole
world. (`bug.md` at the repo root, if present, is the context of the bug that
started this cycle — and it is deleted when the diagrams are created, per
*The bug rule*.)

## The skill (hard rule)

Every diagram is built with the **diagram-design** skill — always, no
exception:

- Read `~/.pi/agent/git/github.com/cathrynlavery/diagram-design/skills/diagram-design/SKILL.md`
  and follow it: the type selection, the complexity budget, the taste gate.
- The style is profiled: the project marker `.diagram-design` selects the
  `flowdeck` profile (dark metallic, the status colors) — do not re-run the
  style gate, do not invent tokens.
- Before delivery, both checks must pass:
  - `python3 ~/.pi/agent/git/github.com/cathrynlavery/diagram-design/skills/diagram-design/scripts/self_check.py <file>`
  - `python3 ~/.pi/agent/git/github.com/cathrynlavery/diagram-design/scripts/verify-geometry.py <file>`

## The output (one file, two readers)

- One HTML file per diagram under `diagrams/improve/`, each with its own
  `diagrams/css/<name>.css` (one stylesheet per HTML file).
- **Human view**: the SVG (the diagram-design layout of the chosen type).
- **Machine view**: sections with ids — tables of the same data the SVG
  shows, so the task author can cite the exact part it reads (the `Source`
  field of `agent-write-task.md`). At minimum: the changes (today → target,
  per NEW/CHANGED item), the pinned values (hex · states · ids · names), and
  the facts it depends on (Fact · Evidence · Validated, copied from the map's
  `#facts`).
- NEW/CHANGED markers against the current base — the diagrams say *what
  changes*, not what exists.
- Register each file in `diagrams/README.md` (the improve section + the
  Files list), with the one-line description.

The values come from the WANT or the code — never invented. The status
palette is fixed (queued `#7cc0ff` · running `#ff9142` · success `#46d19a` ·
failed `#ff5450` · cancelled `#8f96a3`).

The diagrams are ephemeral: they are deleted by the last task of the set,
after implementation.

## Validation (the loop)

The diagrams are shown to the user and iterated until approved. The handoff
to the task author happens only on approval — do not assume approval.

## The bug rule

After the diagrams are created and approved, delete `bug.md` (repo root) if
present — the bug that started this cycle is now re-addressed by the new
diagrams. It is deleted after the diagrams are created, never before.

## Trust (anti-reread)

- **Facts are trusted.** A fact row (Fact · Evidence · Validated, from the
  map's `#facts`) is used as-is — no re-running the spike — except when its
  diff-check fails (the evidence files changed, or the SDK version moved).
  Then re-validate that fact only.
- **Reads are scoped.** The map sections the WANT touches, then the code
  files they name. Do not inventory the package.
- **Decide once.** A value the WANT does not pin: pick one, record it in the
  diagram's sections (value + one line of why), do not revisit it.

## What the diagram author does not do

- It does not write tasks (that is `agent-write-task.md`).
- It does not touch code: no edits outside `diagrams/` (the improve
  HTMLs/CSS + the README registration).
- It does not edit `diagrams/package-map.html` (the set's last task owns it).
- It does not skip the skill, the checks, the registration, or the
  validation loop.

## Report

- the files written (the HTMLs + the CSSs) and the type of each diagram
- the values pinned (the machine sections)
- the README registration
- the user's approval
