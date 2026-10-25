// Package status is the run state machine of the dashboard: the four
// states a run moves through and the terminal rule (success/failed end
// the 2s badge poll). Pure value object — the DBOS read that produces a
// State lives in the core package.
package status

// State is the UI state of a workflow run — one of the four badge colors
// of the dashboard.
type State string

// The four execution states, in the order the run moves through them:
//
//	queued -> running -> success | error
const (
	Queued  State = "queued"  // in queue  (#7cc0ff)
	Running State = "running" // running   (#ff9142)
	Success State = "success" // success   (#46d19a)
	Error   State = "error"   // failed    (#ff5450)
)

// Terminal reports whether the state is terminal: success and failed end
// the run — the 2s badge poll stops and the UI shows a dot.
func (s State) Terminal() bool {
	return s == Success || s == Error
}
