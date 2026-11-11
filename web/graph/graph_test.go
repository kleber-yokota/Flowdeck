package graph

import (
	"testing"

	"flowdeck/core/graph"
)

// baseBoxW is a step box's width: the text at the char width plus the
// inner padding on both sides.
func TestBaseBoxW(t *testing.T) {
	for name, want := range map[string]float64{
		"":        2 * GraphPadX,
		"a":       graphCharW + 2*GraphPadX,
		"step":    4 * graphCharW + 2*GraphPadX,
		"fetch":   5 * graphCharW + 2*GraphPadX,
	} {
		if got := baseBoxW(name); got != want {
			t.Errorf("baseBoxW(%q): got %v want %v", name, got, want)
		}
	}
}

// NodeBoxW is the node box's width: the step width for the plain kinds
// (step, async), the step width plus the symbol zone for the child and
// wait kinds (the child/wait symbols on the box's right).
func TestNodeBoxW(t *testing.T) {
	base := baseBoxW("step")
	for kind, want := range map[graph.NodeKind]float64{
		graph.NodeStep:  base,
		graph.NodeAsync: base,
		graph.NodeChild: base + GraphSymW,
		graph.NodeWait:  base + GraphSymW,
	} {
		if got := NodeBoxW("step", kind); got != want {
			t.Errorf("NodeBoxW(%q, %v): got %v want %v", "step", kind, got, want)
		}
	}
}

// Num formats a layout coordinate: one decimal (enough for px), rounded
// to it (the svg keeps the content units — the viewer zooms).
func TestNum(t *testing.T) {
	for f, want := range map[float64]string{
		0:      "0.0",
		12:     "12.0",
		12.34:  "12.3",
		12.36:  "12.4",
		48:     "48.0",
		0.05:   "0.1",
		-3.26:  "-3.3",
	} {
		if got := Num(f); got != want {
			t.Errorf("Num(%v): got %q want %q", f, got, want)
		}
	}
}
