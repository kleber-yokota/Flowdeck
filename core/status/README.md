# core/status

The run state machine of the dashboard: the five states a run moves through and the terminal rule. Pure value object — the DBOS read that produces a `State` lives in `core/reads`.

## Rules (decisions · pinned values)

- The five states, in the order the run moves through them: `queued → running → success | error | cancelled`. *(evidence: `state.go · State`)*
- Terminal rule: `success | error | cancelled` end the run — the 2s badge poll stops and the UI shows a dot (no poll). *(evidence: `state.go · (State).Terminal` · `web/executions/badge.go · handleBadge`)*
- The badge colors are the contract: `queued` #7cc0ff · `running` #ff9142 · `success` #46d19a · `error` #ff5450 · `cancelled` #8f96a3. *(evidence: `state.go` · `web/render/templates/badge.html`)*
