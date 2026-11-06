// svg.go is the shared svg emission of the two graph emitters (the
// executed path and the block DAG): one scale, one number format, one
// arrow marker.
package graph

import (
	"strconv"
)

// GraphScale is the fixed svg scale (readability): the svg is emitted 1.25×
// the content canvas (width/height are content units × the scale). One
// scale, one unit (content units) — the zoom is the svg viewer's job.
const GraphScale = 1.25

// ArrowDefs is the svg's arrow marker, the single source both emitters
// write: the edge lines reference it via marker-end.
const ArrowDefs = `<defs><marker id="flowdeck-arrow" markerWidth="10" markerHeight="10" refX="9" refY="5" orient="auto" markerUnits="userSpaceOnUse"><path class="g-arrow" d="M0,0 L10,5 L0,10 Z"/></marker></defs>`

// Num formats a layout coordinate (one decimal is enough for px).
func Num(f float64) string {
	return strconv.FormatFloat(f, 'f', 1, 64)
}
