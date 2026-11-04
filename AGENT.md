# AGENT.md — the markdown rule

Any change that writes a markdown inside a folder (a folder `README.md`
page, the `index.md` rows, the `diagrams/README.md` registrations) applies
the **writing-great-skills** skill to the edit: read
`~/.agents/skills/writing-great-skills/SKILL.md` (+ `GLOSSARY.md`) before
the first edit — the skill is the home of the lens; this file is the
trigger and the page-specific criteria.

The edit is done when every line of the touched page passes the tests:

- **One meaning, one home** — the page that owns the behavior states the
  rule or the fact; every other page points to it (`the queue: core/boot`).
  A pointer is a name + a path, never a restatement.
- **Every row carries its proof** — a `Rule`/`Fact` row has `Evidence`
  (file · symbol) and `Validated` (who · when · how); a row without them
  stays in `tasks/fact.md` until it has.
- **The no-op test** — a line that adds nothing against the code it cites
  is deleted, not trimmed.
- **No temporal sediment** — the page says what *is*: no "this round", no
  task or phase names, no "next round".
- **Numbers point** — a number that lives in the map or in the code is
  cited by pointer, never restated.
- **Negation pairs** — a prohibition ships with the positive it guards.

The intro of a page is its leading word: the folder in one sentence; the
rules are the home, the facts the evidence.
