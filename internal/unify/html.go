// Copyright 2025 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package unify

import (
	"fmt"
	"html"
	"io"
	"strings"
)

func (t *tracer) writeHTML(w io.Writer) {
	if !t.saveTree {
		panic("writeHTML called without tracer.saveTree")
	}

	fmt.Fprintf(w, "<html><head><style>%s</style></head>", htmlCSS)
	for _, root := range t.trees {
		dot := newDotEncoder()
		html := htmlTracer{w: w, dot: dot}
		html.writeTree(root)
	}
	fmt.Fprintf(w, "</html>\n")
}

const htmlCSS = `
.unify {
	display: grid;
	grid-auto-columns: min-content;
	text-align: center;
}

.header {
	grid-row: 1;
	font-weight: bold;
	padding: 0.25em;
	position: sticky;
	top: 0;
	background: white;
}

.envFactor {
	display: grid;
	grid-auto-rows: min-content;
	grid-template-columns: subgrid;
	text-align: center;
}
`

type htmlTracer struct {
	w    io.Writer
	dot  *dotEncoder
	svgs map[*Value]string
}

func (t *htmlTracer) writeTree(node *traceTree) {
	// TODO: This could be really nice.
	//
	// - Put nodes that were unified on the same rank with {rank=same; a; b}
	//
	// - On hover, highlight nodes that node was unified with and the result. If
	// it's a variable, highlight it in the environment, too.
	//
	// - On click, show the details of unifying that node.
	//
	// This could be the only way to navigate, without necessarily needing the
	// whole nest of <detail> nodes.

	// TODO: It might be possible to write this out on the fly.

	t.emit([]*Value{node.v, node.w}, []string{"v", "w"}, node.envIn)

	// Render children.
	for i, child := range node.children {
		if i >= 10 {
			fmt.Fprintf(t.w, `<div style="margin-left: 4em">...</div>`)
			break
		}
		fmt.Fprintf(t.w, `<details style="margin-left: 4em"><summary>%s</summary>`, html.EscapeString(child.label))
		t.writeTree(child)
		fmt.Fprintf(t.w, "</details>\n")
	}

	// Render result.
	if node.err != nil {
		fmt.Fprintf(t.w, "Error: %s\n", html.EscapeString(node.err.Error()))
	} else {
		t.emit([]*Value{node.res}, []string{"res"}, node.env)
	}
}

func (t *htmlTracer) svg(v *Value) string {
	if s, ok := t.svgs[v]; ok {
		return s
	}
	var buf strings.Builder
	t.dot.subgraph(v)
	t.dot.writeSvg(&buf)
	t.dot.clear()
	svg := buf.String()
	if t.svgs == nil {
		t.svgs = make(map[*Value]string)
	}
	t.svgs[v] = svg
	buf.Reset()
	return svg
}

func (t *htmlTracer) emit(vs []*Value, labels []string, env nonDetEnv) {
	fmt.Fprintf(t.w, `<div class="unify">`)
	for i, v := range vs {
		fmt.Fprintf(t.w, `<div class="header" style="grid-column: %d">%s</div>`, i+1, html.EscapeString(labels[i]))
		fmt.Fprintf(t.w, `<div style="grid-area: 2 / %d">%s</div>`, i+1, t.svg(v))
	}

	t.emitEnv(env, len(vs))

	fmt.Fprintf(t.w, `</div>`)
}

func (t *htmlTracer) emitEnv(env nonDetEnv, colStart int) {
	if env.isBottom() {
		fmt.Fprintf(t.w, `<div class="header" style="grid-column: %d">_|_</div>`, colStart+1)
		return
	}

	colLimit := 10
	col := colStart
	for i, f := range env.factors {
		if i > 0 {
			// Print * between each factor.
			fmt.Fprintf(t.w, `<div class="header" style="grid-column: %d">&times;</div>`, col+1)
			col++
		}

		var idCols []int
		for i, id := range f.ids {
			var str string
			if i == 0 && len(f.ids) > 1 {
				str = "("
			}
			if colLimit <= 0 {
				str += "..."
			} else {
				str += html.EscapeString(t.dot.idp.unique(id))
			}
			if (i == len(f.ids)-1 || colLimit <= 0) && len(f.ids) > 1 {
				str += ")"
			}

			fmt.Fprintf(t.w, `<div class="header" style="grid-column: %d">%s</div>`, col+1, str)
			idCols = append(idCols, col)

			col++
			if colLimit <= 0 {
				break
			}
			colLimit--
		}

		fmt.Fprintf(t.w, `<div class="envFactor" style="grid-area: 2 / %d / 3 / %d">`, idCols[0]+1, col+1)
		rowLimit := 10
		row := 0
		for _, term := range f.terms {
			// TODO: Print + between rows? With some horizontal something to
			// make it clear what it applies across?

			for i, val := range term.vals {
				fmt.Fprintf(t.w, `<div style="grid-area: %d / %d">`, row+1, idCols[i]-idCols[0]+1)
				if i < len(term.vals)-1 && i == len(idCols)-1 {
					fmt.Fprintf(t.w, `...</div>`)
					break
				} else if rowLimit <= 0 {
					fmt.Fprintf(t.w, `...</div>`)
				} else {
					fmt.Fprintf(t.w, `%s</div>`, t.svg(val))
				}
			}

			row++
			if rowLimit <= 0 {
				break
			}
			rowLimit--
		}
		fmt.Fprintf(t.w, `</div>`)
	}
}
