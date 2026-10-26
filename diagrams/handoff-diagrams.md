# Handoff — pending adjustments

Changes discussed and agreed in principle, **not yet applied**. Each item lists the
decision, the files to touch (with the current line anchors) and the open questions.
Apply the code change and the diagram in the same commit (the diagram is the spec).

## 1. Flow change — sanitize in the web, not the core

**Current (implemented, phase 5):** the web forwards the raw form values and the
core sanitizes them against the parsed spec, before the DBOS enqueue:

```
form → formValues (web/web.go:146) → Port.Enqueue(name, values)
     → sanitize (core/spec.go:44) → dispatch → DBOS queue
```

**Proposed:** the web already asks the core for the spec (`Port.Spec`,
`handleWorkflowPage` — the form on top). If the web sanitizes the values against
that spec **before** calling `Enqueue`, the core receives a clean input and the
`Enqueue` contract becomes "clean input in".

**Files to change:**
- `web/web.go` — `handleRunDispatch` (:115): after `formValues`, call `p.Spec(name)`
  and sanitize against the spec before `p.Enqueue`; unknown field / bad int → 400 directly
- `core/spec.go` — `sanitize` (:44) / `convertInput` (:64): move the rules to the web
  (or expose them through the `Port`); `ErrInvalidInput` (:7) stops being a core error
- `core/core.go` — `Enqueue` (:247-270): drop `specOf`/`sanitize`; trust the caller
  (or keep a defense — see open question)
- `core/port.go` — `Enqueue` contract (:30-34): raw values → clean input
- `web/render.go` — `ErrInvalidInput` mapping (:23): no longer needed (web raises it)
- tests — `core/sanitize_test.go` → `web/sanitize_test.go`; `core/core_test.go`
  dispatch tests enqueue clean values
- `sequence-phase-5.html` — the `SANITIZE (SPEC)` label (line 129, CORE lifeline)
  moves to the WEB lifeline; the desc (lines 19, 206): "the web forwards the raw
  values and the core sanitizes them" → "the web sanitizes the values against the
  spec and forwards the clean input"
- `package-map.html` — `web → core/graph` contact (Spec + the rules), `core` file
  table (`spec.go` without `sanitize`)

**Open question — who guards the core's boundary?** if `Enqueue` stops sanitizing,
every future caller (tests, a future CLI, anything that bypasses the web) must
sanitize on its own: the guarantee moves from the boundary to the callers. The
`failing-test` dispatch (no parsed spec) uses the nil path either way. Decide:
(a) web sanitizes, core trusts — simplest, matches this flow; (b) web sanitizes,
core re-checks — duplicated rules; (c) keep the core as the single gate (status quo).

**Not affected by the change:** resume (the stored input is immutable per
workflow_id), the 2s polling, the ACK, "input written once".

## 2. Resume guard condition — `sequence-phase-5.html`

**Current:** the RESUME frame guard (line 268) reads `[after FAILED run]`.

**Agreed rule** (implemented + tested — `core/resume_test.go`, `core/resume.go`):
resume is allowed only when the badge state is `failed`; `queued` / `running` /
`success` → 400 `ErrNotResumable`.

**Adjustment:** make the guard express the full state rule, e.g.
`[state = failed · queued / running / success → 400]`.
