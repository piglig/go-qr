package go_qr

import (
	"fmt"
	"strconv"
	"strings"
)

func (q *QrCode) toSvgOptimizedString(config *QrCodeImgConfig, lightColor, darkColor string) string {
	scale := config.scale
	border := config.border
	sb := strings.Builder{}
	sb.Grow(1024)
	sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	sb.WriteString("<!DOCTYPE svg PUBLIC \"-//W3C//DTD SVG 1.1//EN\" \"http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd\">\n")
	n := q.GetSize()*scale + border*2
	sb.WriteString(fmt.Sprintf("<svg xmlns=\"http://www.w3.org/2000/svg\" version=\"1.1\" viewBox=\"0 0 %d %d\" stroke=\"none\">\n",
		n, n))
	if lightColor != "" {
		sb.WriteString("\t<rect width=\"100%\" height=\"100%\" fill=\"" + lightColor + "\"/>\n")
	}

	graph := q.assembleBorderGraph()
	sb.WriteString("\t<path d=\"")
	graph.writePath(&sb, border, scale)
	sb.WriteString("\" fill=\"" + darkColor + "\" fill-rule=\"evenodd\"/>\n")
	sb.WriteString("</svg>\n")

	return sb.String()
}

// node represents one node of the svg path creating the QR code.
// x,y are the grid coordinates of the node in the QR code.
// top is set to true if these coordinates can belong to two distinct nodes
// (the diagonal-touching modules case, see docs below) and this node belongs
// to the path around the upper of the two modules.
//
//	 _ _             _ _
//	|   |           |   |
//	|_ _|_ _    _ _ |_ _|
//	    |   |  |   |
//	    |_ _|  |_ _|
type node struct {
	x, y int
	top  bool
}

func (n node) imageXY(border, scale int) (int, int) {
	return n.x*scale + border, n.y*scale + border
}

// edges holds the two line segments incident on a node. A closed border always
// gives exactly two per node.
type edges struct {
	first, second node
}

func (e edges) formCorner() bool {
	return e.first.x != e.second.x && e.first.y != e.second.y
}

// borderGraph is a deterministic, array-backed replacement for the previous
// map[node]edges. Grid points live on (n+1) × (n+1); the `top` flag doubles
// that for the diagonal-touch disambiguation. Iteration order is fixed, which
// makes output byte-stable and enables golden-file regression tests.
type borderGraph struct {
	stride int // n + 1
	// Flat slice indexed by ((x*stride+y)<<1) | topBit. Presence is tracked
	// separately because a zero-valued edges struct is a legal value (edge to
	// node{0,0,false}).
	data   []edges
	exists []bool
}

func newBorderGraph(n int) *borderGraph {
	stride := n + 1
	size := stride * stride * 2
	return &borderGraph{
		stride: stride,
		data:   make([]edges, size),
		exists: make([]bool, size),
	}
}

func (g *borderGraph) index(x, y int, top bool) int {
	i := (x*g.stride + y) << 1
	if top {
		i |= 1
	}
	return i
}

func (g *borderGraph) add(from, to node) {
	i := g.index(from.x, from.y, from.top)
	if !g.exists[i] {
		g.data[i] = edges{first: to}
		g.exists[i] = true
	} else {
		g.data[i].second = to
	}
}

func (g *borderGraph) get(n node) edges {
	return g.data[g.index(n.x, n.y, n.top)]
}

// writePath walks the graph in deterministic index order and writes the SVG
// path `d` attribute data for every closed loop.
func (g *borderGraph) writePath(sb *strings.Builder, border, scale int) {
	visited := make([]bool, len(g.exists))

	for idx := range g.exists {
		if !g.exists[idx] || visited[idx] {
			continue
		}
		startNode := g.nodeAt(idx)
		startEdges := g.data[idx]
		if !startEdges.formCorner() {
			continue
		}

		startX, startY := startNode.imageXY(border, scale)
		sb.WriteByte('M')
		writeInt(sb, startX)
		sb.WriteByte(',')
		writeInt(sb, startY)

		prev := startNode
		cur := startNode
		next := startEdges.first
		for next != startNode {
			prev = cur
			cur = next
			curEdges := g.get(cur)
			if curEdges.first == prev {
				next = curEdges.second
			} else {
				next = curEdges.first
			}
			if curEdges.formCorner() {
				if prev.x == cur.x {
					_, y := cur.imageXY(border, scale)
					sb.WriteByte('V')
					writeInt(sb, y)
				} else {
					x, _ := cur.imageXY(border, scale)
					sb.WriteByte('H')
					writeInt(sb, x)
				}
			}
			visited[g.index(cur.x, cur.y, cur.top)] = true
		}
		sb.WriteByte('L')
		writeInt(sb, startX)
		sb.WriteByte(',')
		writeInt(sb, startY)
		visited[idx] = true
	}
}

func (g *borderGraph) nodeAt(idx int) node {
	top := idx&1 == 1
	cell := idx >> 1
	return node{x: cell / g.stride, y: cell % g.stride, top: top}
}

func writeInt(sb *strings.Builder, v int) {
	sb.WriteString(strconv.Itoa(v))
}

// assembleBorderGraph builds the border graph of all connected filled regions
// in the QR code. Borders between two adjacent filled modules are omitted.
func (q *QrCode) assembleBorderGraph() *borderGraph {
	n := q.GetSize()
	g := newBorderGraph(n)
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			if !q.GetModule(x, y) {
				continue
			}
			top := y == 0 || !q.GetModule(x, y-1)
			right := x == n-1 || !q.GetModule(x+1, y)
			bottom := y == n-1 || !q.GetModule(x, y+1)
			left := x == 0 || !q.GetModule(x-1, y)

			if top {
				l := node{x: x, y: y}
				r := node{x: x + 1, y: y}
				g.add(l, r)
				g.add(r, l)
			}
			if left {
				t := node{x: x, y: y}
				b := node{x: x, y: y + 1, top: bottom}
				g.add(t, b)
				g.add(b, t)
			}
			if bottom {
				l := node{x: x, y: y + 1, top: left}
				r := node{x: x + 1, y: y + 1, top: right}
				g.add(l, r)
				g.add(r, l)
			}
			if right {
				t := node{x: x + 1, y: y}
				b := node{x: x + 1, y: y + 1, top: bottom}
				g.add(t, b)
				g.add(b, t)
			}
		}
	}
	return g
}
