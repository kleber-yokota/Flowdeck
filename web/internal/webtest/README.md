# web/internal/webtest

The fake `Port` and the HTTP helpers of the web handler tests: the web never tests against the real core — a test overrides only the one operation under test.

## Rules (decisions · pinned values)

- `TestPort` builds the base stub port: every core operation is a stub (the empty listing, the empty graph, the clean rows below) so a test can override only the one operation under test. *(evidence: `webtest.go · TestPort`)*
- The ok-fakes return canned rows: `AckEnqueuer` (a fixed id · queued · a fixed created_at) · `okStatuser` (queued) · `okSteps` (a terminal success path with one step) · `okDetail` (the unknown execution — the 404 twin of the booted detail tests) · `okResumer`/`okCanceller` (the ACK rows) · `okLive` (1 · 2 · 3). *(evidence: `webtest.go`)*
- The HTTP helpers (`Get` · `Post` · `PostForm` · `ReadAll`) hit the test server; the handler tests use them to assert fragments and statuses. *(evidence: `webtest.go`)*
