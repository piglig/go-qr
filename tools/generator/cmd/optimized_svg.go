package cmd

import (
	"fmt"
	"strings"

	go_qr "github.com/piglig/go-qr"
)

func toSvgOptimizedString(qr *go_qr.QrCode, border uint, scale uint, lightColor, darkColor string) string {
	// Write the header of the svg.
	sb := strings.Builder{}
	sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	sb.WriteString("<!DOCTYPE svg PUBLIC \"-//W3C//DTD SVG 1.1//EN\" \"http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd\">\n")
	// Determine the size of the svg.
	n := qr.GetSize()*int(scale) + int(border*2)
	sb.WriteString(fmt.Sprintf("<svg xmlns=\"http://www.w3.org/2000/svg\" version=\"1.1\" viewBox=\"0 0 %d %d\" stroke=\"none\" style=\"fill-rule:evenodd;clip-rule:evenodd\">\n",
		n, n))
	// If light color is set, the background layer is omitted to yield a
	// transparent background.
	if lightColor != "" {
		sb.WriteString("\t<rect width=\"100%\" height=\"100%\" fill=\"" + lightColor + "\"/>\n")
	}

	// Create a graph representing the border of all areas with filled modules.
	nodes := assembleBorderGraph(qr)

	// Create a path consisting of several closed loops, which connect all the
	// just determined nodes along their edges.
	sb.WriteString("\t<path d=\"")
	connectedNodes := make(map[node]bool, len(nodes))
	for startNode, edges := range nodes {
		// Skip the node if it is already connected to a drawn path.
		if connected, ok := connectedNodes[startNode]; ok && connected {
			continue
		}
		// Skip the node if it is not in a corner. The starting nodes should
		// always be in a corner, as nodes on straight lines do not have to be
		// drawn anyway.
		if !edges.formCorner() {
			continue
		}
		// Move cursor to staring node.
		startX, startY := startNode.imageXY(border, scale)
		sb.WriteString(fmt.Sprintf("M%d,%d", startX, startY))
		// Move along edges until the starting node is reached.
		prevNode := startNode
		curNode := startNode
		nextNode := edges.first
		for nextNode != startNode {
			// The next node is set to be the current node.
			prevNode = curNode
			curNode = nextNode
			// Get the edges of the current node.
			curEdges := nodes[curNode]
			// Select the edge of the current node, which does not connect back
			// to the previous node.
			if curEdges.first == prevNode {
				nextNode = curEdges.second
			} else {
				nextNode = curEdges.first
			}
			// Only draw a line if the current node is at a corner,
			// otherwise it is skipped.
			if curEdges.formCorner() {
				// Draw a line to the current node.
				if prevNode.x == curNode.x {
					_, y := curNode.imageXY(border, scale)
					// Previous and current node are on a vertical line.
					sb.WriteString(fmt.Sprintf("V%d", y))
				} else {
					// Previous and current node are on a horizontal line.
					x, _ := curNode.imageXY(border, scale)
					sb.WriteString(fmt.Sprintf("H%d", x))
				}
			}
			// Mark the current node as being connected to the path.
			connectedNodes[curNode] = true
		}
		// Draw the final line to the start.
		sb.WriteString(fmt.Sprintf("L%d,%d", startX, startY))
		// Mark the start node as being connected to the path.
		connectedNodes[startNode] = true

	}
	sb.WriteString("\" style=\"fill:" + darkColor + "\"/>\n")
	sb.WriteString("</svg>\n")

	return sb.String()
}

// Node represents one node of the svg path creating the QR code.
// x,y are the pixel coordinates of the node in the path.
// top is set to true, if these coordinates can belong to two nodes and this
// node belongs to the path around the upper of these two nodes.
// This can happen if the top right corner of a module coincides with the bottom
// left corner of another module or the top left corner of a module coincides
// with the bottom right corner of another module, as shown below.
//
//	 _ _             _ _
//	|   |           |   |
//	|_ _|_ _    _ _ |_ _|
//	    |   |  |   |
//	    |_ _|  |_ _|
//
// To distinguish both nodes in this case, the node belonging to the path around
// the upper module, will have top set to true.
type node struct {
	x, y int
	top  bool
}

// imageXY converts the coordinates of a node in the QR code to the coordinates
// of a point in the svg image taking the border around the QR code and the
// scale applied to the code into account.
func (n node) imageXY(border uint, scale uint) (x int, y int) {
	x = n.x*int(scale) + int(border)
	y = n.y*int(scale) + int(border)
	return
}

// edges are the two line segments originating from one node in the svg path.
// As the full path is a set of closed loops, each node will always have two
// edges attached to it. For the edges only the nodes to which they connect are
// stored.
type edges struct {
	first  node
	second node
}

// formCorner yields true if the edges of of a node form a corner and not a
// straight line. As the drawn svg path only consists of lines, which are
// parallel to the x or y axis, this is case if and only if the two nodes to
// to which a node is connected differ in both coordinates.
func (e edges) formCorner() bool {
	return e.first.x != e.second.x && e.first.y != e.second.y
}

// addEdge adds an edge to toNode to the set of edges of fromNode.
func addEdge(nodes map[node]edges, fromNode node, toNode node) {
	// Check if an edge for fromNode has already been added.
	fromEdges, ok := nodes[fromNode]
	if !ok {
		// If not add an edge to toNode for it as its first edge.
		nodes[fromNode] = edges{first: toNode}
	} else {
		// Otherwise add an edge to toNode for it as its second edge.
		nodes[fromNode] = edges{first: fromEdges.first, second: toNode}
	}
}

// assembleBorderGraph create a graph data structure representing the border of
// all connected areas in the QR code with filled modules. The borders between
// two filled and adjacent modules are all removed.
func assembleBorderGraph(qr *go_qr.QrCode) map[node]edges {
	nodes := make(map[node]edges)
	n := qr.GetSize()
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			if qr.GetModule(x, y) {
				// Select which edges of the module have to be in the svg path.
				// These are all edges which are not adjacent to another filled
				// module.
				top := y == 0 || !qr.GetModule(x, y-1)
				right := x == n-1 || !qr.GetModule(x+1, y)
				bottom := y == n-1 || !qr.GetModule(x, y+1)
				left := x == 0 || !qr.GetModule(x-1, y)
				// Store edges in both directions.
				if top {
					leftNode := node{x: x, y: y}
					rightNode := node{x: x + 1, y: y}
					addEdge(nodes, leftNode, rightNode)
					addEdge(nodes, rightNode, leftNode)
				}
				// If both edges of a bottom corner of the module have to be
				// drawn, the coordinates could coincided with the a corner
				// of another module. To distinguish the corners, this node
				// is marked as belonging to the upper module.
				if left {
					topNode := node{x: x, y: y}
					bottomNode := node{x: x, y: y + 1, top: bottom}
					addEdge(nodes, topNode, bottomNode)
					addEdge(nodes, bottomNode, topNode)
				}
				if bottom {
					leftNode := node{x: x, y: y + 1, top: left}
					rightNode := node{x: x + 1, y: y + 1, top: right}
					addEdge(nodes, leftNode, rightNode)
					addEdge(nodes, rightNode, leftNode)
				}
				if right {
					topNode := node{x: x + 1, y: y}
					bottomNode := node{x: x + 1, y: y + 1, top: bottom}
					addEdge(nodes, topNode, bottomNode)
					addEdge(nodes, bottomNode, topNode)
				}
			}
		}
	}
	return nodes
}
