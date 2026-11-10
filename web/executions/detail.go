package executions

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"flowdeck/core"
	"flowdeck/core/reads"
	"flowdeck/core/status"
	"flowdeck/web/executions/badge"
	"flowdeck/web/graph/executed"
	"flowdeck/web/render"
)

// executionDetail is the data for the execution detail full page (v6.3):
// the badge (polling while live) and the split fragment (the meta row, the
// actions, the step log with the child blocks and the dashed tail, and the
// sticky executed path) — the failure section is gone (the state is on the
// badge, the failed step is highlighted in the log, and its error is
// reachable through the expand fragment).
type executionDetail struct {
	ID    string
	Name  string
	Badge badge.Badge
	Steps stepsView
	Title string
}

// stepRowView is one row of the step log: the t+ start (one decimal), the
// name, the child mark, the state cell (done · error · the child's live
// state), the expand target id, and — on the child row — the child block.
// The dashed tail row (while live) is a stepRowView with no name (T "…").
type stepRowView struct {
	Index      int
	T          string
	Name       string
	Mark       string
	State      string
	StateClass string
	DetailID   string
	Child      *childBlockView
}

// childBlockView is the child block under its launching step: the child's
// id (the deep link target), the child's badge (polling while the child is
// live), and the child's completed step lines.
type childBlockView struct {
	ID    string
	Badge badge.Badge
	Lines []childLineView
}

// childLineView is one completed step line of the child (the name and the
// done duration — no t+: the child's created_at is not in the RunPath read
// model).
type childLineView struct {
	Name     string
	Duration string
}

// childPathsOf reads the steps of every child of the run (the steps read
// is per-run — one p.Steps per child id).
func childPathsOf(p core.Port, d reads.DetailPath) (map[string]reads.RunPath, error) {
	out := map[string]reads.RunPath{}
	for _, row := range d.RunPath.Steps {
		if row.Kind != reads.KindChild || row.ChildID == "" {
			continue
		}
		if _, ok := out[row.ChildID]; ok {
			continue
		}
		cp, err := p.Steps(row.ChildID)
		if err != nil {
			return nil, err
		}
		out[row.ChildID] = cp
	}
	return out, nil
}

// rowsFor builds the step log rows: one row per completed step (the done
// duration), the failed steps (the error state — highlighted in the log),
// and the child rows (the child block under their launching step); the
// queued/running plain rows have no row yet (the tail marks the live run).
func rowsFor(d reads.DetailPath, childPaths map[string]reads.RunPath) []stepRowView {
	var rows []stepRowView
	for i, row := range d.RunPath.Steps {
		t := fmt.Sprintf("t+%.1fs", row.StartedAt.Sub(d.CreatedAt).Seconds())
		detailID := "det-" + strconv.Itoa(i)
		switch {
		case row.Kind == reads.KindChild:
			rows = append(rows, stepRowView{
				Index:      i,
				T:          t,
				Name:       row.Name,
				Mark:       "└─ child",
				State:      string(row.ChildState),
				StateClass: string(row.ChildState),
				DetailID:   detailID,
				Child: &childBlockView{
					ID:    row.ChildID,
					Badge: badge.BadgeFor(row.ChildID, row.ChildState),
					Lines: childLinesFor(childPaths[row.ChildID]),
				},
			})
		case row.State == status.Error:
			rows = append(rows, stepRowView{
				Index:      i,
				T:          t,
				Name:       row.Name,
				State:      "error",
				StateClass: "error",
				DetailID:   detailID,
			})
		case row.State == status.Success:
			rows = append(rows, stepRowView{
				Index:      i,
				T:          t,
				Name:       row.Name,
				State:      fmt.Sprintf("done in %.1fs", row.CompletedAt.Sub(row.StartedAt).Seconds()),
				StateClass: "done",
				DetailID:   detailID,
			})
		}
	}
	if !d.RunPath.State.Terminal() {
		rows = append(rows, stepRowView{
			Index:      len(rows),
			T:          "…",
			State:      "running",
			StateClass: "running",
		})
	}
	return rows
}

// childLinesFor the child's completed step lines (the done duration, no
// t+).
func childLinesFor(cp reads.RunPath) []childLineView {
	var lines []childLineView
	for _, row := range cp.Steps {
		if row.State != status.Success {
			continue
		}
		lines = append(lines, childLineView{
			Name:     row.Name,
			Duration: fmt.Sprintf("done in %.1fs", row.CompletedAt.Sub(row.StartedAt).Seconds()),
		})
	}
	return lines
}

// newStepsView builds the steps partial of a run path (v6.3): the meta
// data (created, the live elapsed, the stored inputs, the retries count),
// the step log rows (with the child blocks and the dashed tail while
// live), and the executed-path graph (the dashed tail while live). The
// page and the 2s poll fragment share it.
func newStepsView(workflowID string, d reads.DetailPath, childPaths map[string]reads.RunPath) stepsView {
	terminal := d.RunPath.State.Terminal()
	return stepsView{
		WorkflowID: workflowID,
		Terminal:   terminal,
		State:      string(d.RunPath.State),
		CreatedAt:  d.CreatedAt,
		Elapsed:    badge.ElapsedLabel(time.Since(d.CreatedAt)),
		Inputs:     d.Inputs,
		Retries:    d.Attempts - 1,
		Rows:       rowsFor(d, childPaths),
		Graph:      executed.ExecutedPathToSVG(d.RunPath.Steps, !terminal),
	}
}

// handleExecutionDetail renders the execution detail full page (v6.3):
// the badge, and the split fragment (the meta row, the actions, the step
// log with the child blocks and the tail, the sticky executed path) — the
// failure section is gone.
func handleExecutionDetail(w http.ResponseWriter, r *http.Request, p core.Port) {
	id := r.PathValue("id")
	d, err := p.Detail(id)
	if err != nil {
		render.Fail(w, "detail", err)
		return
	}
	childPaths, err := childPathsOf(p, d)
	if err != nil {
		render.Fail(w, "detail", err)
		return
	}
	render.Page(w, "execution-detail.html", executionDetail{
		ID:    d.ID,
		Name:  d.Name,
		Badge: badge.BadgeFor(id, d.RunPath.State),
		Steps: newStepsView(id, d, childPaths),
		Title: "Execution " + id,
	})
}
