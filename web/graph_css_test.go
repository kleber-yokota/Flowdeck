package web

// TestGraphCSSCoversEmittedPaint pins the stylesheet contract: every painted
// element of the block-DAG and executed-path svgs — the real stack (the
// production tree, the nested-loop tree, real runs of the booted core) —
// carries an explicit fill or stroke from style.css, no browser default.

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"flowdeck/core"
	"flowdeck/core/graph"
	"flowdeck/core/status"
	"flowdeck/web/graph/block"
	"flowdeck/web/graph/executed"
)

// svgElement: one painted element, its ancestor chain innermost first.
type svgElement struct {
	tag       string
	classes   map[string]bool
	ancestors []svgElement
}

// paintedTags: the element tags the emitters paint (containers are not painted).
var paintedTags = map[string]bool{"rect": true, "text": true, "circle": true, "polygon": true, "path": true, "line": true}

// svgElements walks one emitted svg and returns its painted elements.
func svgElements(svg string) []svgElement {
	var out, stack []svgElement
	dec := xml.NewDecoder(strings.NewReader(svg))
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			el := svgElement{tag: tok.Name.Local}
			for _, a := range tok.Attr {
				if a.Name.Local == "class" {
					el.classes = map[string]bool{}
					for _, c := range strings.Fields(a.Value) {
						el.classes[c] = true
					}
				}
			}
			el.ancestors = append([]svgElement(nil), stack...)
			stack = append(stack, el)
			if paintedTags[el.tag] {
				out = append(out, el)
			}
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		}
	}
	return out
}

// cssPart: one selector part — a tag and/or a run of classes.
type cssPart struct {
	tag     string
	classes []string
} // tag "" when class-only

// cssRule: one stylesheet rule — its descendant chain and declarations.
type cssRule struct {
	parts []cssPart
	decls map[string]string
}

var cssRuleRe = regexp.MustCompile(`([^{}]+)\{([^}]*)\}`)

// cssRules parses the stylesheet into rules (the descendant combinator, at most two parts per selector).
func cssRules(css string) []cssRule {
	var out []cssRule
	for _, m := range cssRuleRe.FindAllStringSubmatch(regexp.MustCompile(`/\*.*?\*/`).ReplaceAllString(css, ""), -1) {
		var parts []cssPart
		for _, s := range strings.Fields(m[1]) {
			if strings.HasPrefix(s, ".") {
				parts = append(parts, cssPart{classes: strings.Split(strings.TrimPrefix(s, "."), ".")})
			} else {
				parts = append(parts, cssPart{tag: s})
			}
		}
		decls := map[string]string{}
		for _, d := range strings.Split(m[2], ";") {
			if eq := strings.Index(d, ":"); eq > 0 {
				decls[strings.TrimSpace(d[:eq])] = strings.TrimSpace(d[eq+1:])
			}
		}
		out = append(out, cssRule{parts: parts, decls: decls})
	}
	return out
}

// partMatches: the part matches the element — the tag, when named, and every class.
func partMatches(p cssPart, el svgElement) bool {
	if p.tag != "" && p.tag != el.tag {
		return false
	}
	for _, c := range p.classes {
		if !el.classes[c] {
			return false
		}
	}
	return true
}

// ruleDecls: the rule that matches the element — its last part on the element, the first on an ancestor.
func ruleDecls(r cssRule, el svgElement) (map[string]string, bool) {
	if !partMatches(r.parts[len(r.parts)-1], el) {
		return nil, false
	}
	if len(r.parts) == 1 {
		return r.decls, true
	}
	for _, a := range el.ancestors {
		if partMatches(r.parts[0], a) {
			return r.decls, true
		}
	}
	return nil, false
}

// TestGraphCSSCoversEmittedPaint: every painted element of every emitted
// graph svg has an explicit fill or stroke from style.css — no browser default.
func TestGraphCSSCoversEmittedPaint(t *testing.T) {
	svgs := blockDAGSVGs(t)
	svgs = append(svgs, executedPathSVGs(t)...)
	cssBytes, err := os.ReadFile(filepath.Join("static", "style.css"))
	if err != nil {
		t.Errorf("read the stylesheet: %v", err)
		return
	}
	rules := cssRules(string(cssBytes))

	unpainted := []string{}
	for i, svg := range svgs {
		for _, el := range svgElements(svg) {
			painted := false
			for _, r := range rules {
				decls, ok := ruleDecls(r, el)
				if !ok {
					continue
				}
				if v, present := decls["fill"]; present && v != "none" {
					painted = true
				}
				if v, present := decls["stroke"]; present && v != "none" {
					painted = true
				}
			}
			if !painted {
				unpainted = append(unpainted, fmt.Sprintf("svg %d: <%s classes=%v>", i, el.tag, el.classes))
			}
		}
	}
	if len(unpainted) > 0 {
		t.Errorf("%d painted element(s) have no explicit fill or stroke in style.css:\n%s", len(unpainted), strings.Join(unpainted, "\n"))
	}
}

// blockDAGSVGs: the possible-paths block DAG of the production workflow tree, plus the nested-loop tree (the l1 · l2 loop outlines and the loop symbol the production tree does not reach).
func blockDAGSVGs(t *testing.T) []string {
	t.Helper()
	out := []string{}
	for _, dir := range []string{filepath.Join("..", "workflows"), nestedLoopTree(t)} {
		g, err := graph.New(os.DirFS(dir))
		if err != nil {
			t.Errorf("graph.New(%s): %v", dir, err)
			return nil
		}
		for name := range g.ByName {
			out = append(out, string(block.BlockToSVG(g.Of(name))))
		}
	}
	return out
}

// nestedLoopSource: the nested-loop workflow, the real parse of a real file —
// the SDK alias is immaterial (calleeName matches on the name), so the source
// stays clear of the forbidden-import grep the web gate runs.
const nestedLoopSource = `package nested

func Register(c sdk.Context) {
	sdk.RegisterWorkflow(c, Nested, sdk.WithWorkflowName("nested"))
}

func Nested(ctx sdk.Context) (string, error) {
	for range items {
		for range items {
			_, _ = sdk.RunAsStep(ctx, func(_ context.Context) (string, error) {
				return "ok", nil
			}, sdk.WithStepName("s"))
		}
	}
	return "ok", nil
}
`

// nestedLoopTree: the nested-loop tree in a temp dir (the real parse of a real file).
func nestedLoopTree(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "nested.go"), []byte(nestedLoopSource), 0o644); err != nil {
		t.Errorf("write the nested-loop tree: %v", err)
		return ""
	}
	return dir
}

// executedPathSVGs: the executed-path svg of one real run per renderable state
// — success · the retried error (the loop group) · the child · running, then cancelled.
func executedPathSVGs(t *testing.T) []string {
	t.Helper()
	bootedTestServer(t)
	p := core.NewPort()
	out := []string{}
	for _, run := range []struct {
		name    string
		payload any
		want    status.State
	}{
		{name: "noinput-test", want: status.Success},
		{name: "failing-test", payload: "boom", want: status.Error},
		{name: "child-test", payload: "hi", want: status.Success},
	} {
		id := runToState(t, p, run.name, run.payload, run.want)
		if id == "" {
			return nil
		}
		out = append(out, executedSVGForRun(t, p, id))
	}
	// The block run: Running while it blocks, Cancelled after the cancel
	// (the cancel releases the executor worker — the suite settles).
	id := runToState(t, p, "live-test", "block", status.Running)
	if id == "" {
		return nil
	}
	out = append(out, executedSVGForRun(t, p, id))
	if _, err := p.Cancel(id); err != nil {
		t.Errorf("cancel %s: %v", id, err)
		return nil
	}
	if st, err := p.Status(id); err != nil || st != status.Cancelled {
		t.Errorf("run %s: %v, want cancelled", id, st)
		return nil
	}
	out = append(out, executedSVGForRun(t, p, id))
	return out
}

// runToState: enqueues and polls until the wanted state (the 30s liveness
// bound — a deadline, not a timing assertion); "" on failure.
func runToState(t *testing.T, p core.Port, name string, payload any, want status.State) string {
	t.Helper()
	ex, err := p.Enqueue(name, payload)
	if err != nil {
		t.Errorf("enqueue %s: %v", name, err)
		return ""
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		st, err := p.Status(ex.ID)
		if err != nil {
			t.Errorf("status: %v", err)
			return ""
		}
		if st == want {
			return ex.ID
		}
		if st.Terminal() || time.Now().After(deadline) {
			t.Errorf("run %s: %s, want %s", ex.ID, st, want)
			return ""
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// executedSVGForRun: the executed-path svg of the run's real steps.
func executedSVGForRun(t *testing.T, p core.Port, id string) string {
	t.Helper()
	path, err := p.Steps(id)
	if err != nil {
		t.Errorf("steps %s: %v", id, err)
		return ""
	}
	return string(executed.ExecutedPathToSVG(path.Steps))
}
