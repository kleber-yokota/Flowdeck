// Package status is the run state machine of the dashboard: the five
// states a run moves through and the terminal rule (success/failed/
// cancelled end the 2s badge poll). Pure value object — the DBOS read
// that produces a State lives in the core package.
package status

// State is the UI state of a workflow run — one of the five badge colors
// of the dashboard.
type State string

// The five execution states, in the order the run moves through them:
//
//	queued -> running -> success | error | cancelled
const (
	Queued    State = "queued"    // in queue  (#7cc0ff)
	Running   State = "running"   // running   (#ff9142)
	Success   State = "success"   // success   (#46d19a)
	Error     State = "error"     // failed    (#ff5450)
	Cancelled State = "cancelled" // cancelled (#8f96a3)
)

// Terminal reports whether the state is terminal: success, failed and
// cancelled end the run — the 2s badge poll stops and the UI shows a dot.
func (s State) Terminal() bool {
	return s == Success || s == Error || s == Cancelled
}
