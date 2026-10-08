package qr

import "math"

// perspective maps module coordinates (u, v) to image coordinates through a
// homography:
//
//	x = (a·u + b·v + c) / (g·u + h·v + 1)
//	y = (d·u + e·v + f) / (g·u + h·v + 1)
type perspective struct {
	a, b, c, d, e, f, g, h float64
}

// quadToQuad returns the homography that maps the four source points to the
// four destination points, both in the order p0, p1, p2, p3 around the
// quadrilateral. It reports false for degenerate input.
func quadToQuad(src, dst [4][2]float64) (perspective, bool) {
	s, ok := squareToQuad(src)
	if !ok {
		return perspective{}, false
	}
	d, ok := squareToQuad(dst)
	if !ok {
		return perspective{}, false
	}
	inv, ok := s.inverse()
	if !ok {
		return perspective{}, false
	}
	return d.compose(inv), true
}

// squareToQuad maps the unit square (0,0), (1,0), (1,1), (0,1) to q
// (Heckbert, "Fundamentals of Texture Mapping and Image Warping", 1989).
func squareToQuad(q [4][2]float64) (perspective, bool) {
	x0, y0, x1, y1 := q[0][0], q[0][1], q[1][0], q[1][1]
	x2, y2, x3, y3 := q[2][0], q[2][1], q[3][0], q[3][1]
	dx3, dy3 := x0-x1+x2-x3, y0-y1+y2-y3
	if dx3 == 0 && dy3 == 0 {
		return perspective{x1 - x0, x2 - x1, x0, y1 - y0, y2 - y1, y0, 0, 0}, true
	}
	dx1, dx2, dy1, dy2 := x1-x2, x3-x2, y1-y2, y3-y2
	den := dx1*dy2 - dx2*dy1
	if den == 0 {
		return perspective{}, false
	}
	g := (dx3*dy2 - dx2*dy3) / den
	h := (dx1*dy3 - dx3*dy1) / den
	return perspective{
		x1 - x0 + g*x1, x3 - x0 + h*x3, x0,
		y1 - y0 + g*y1, y3 - y0 + h*y3, y0,
		g, h,
	}, true
}

// matrix returns the homography as a row-major 3×3 matrix.
func (p perspective) matrix() [9]float64 {
	return [9]float64{p.a, p.b, p.c, p.d, p.e, p.f, p.g, p.h, 1}
}

func fromMatrix(m [9]float64) (perspective, bool) {
	if m[8] == 0 || math.IsNaN(m[8]) {
		return perspective{}, false
	}
	k := 1 / m[8]
	return perspective{m[0] * k, m[1] * k, m[2] * k, m[3] * k, m[4] * k, m[5] * k, m[6] * k, m[7] * k}, true
}

func (p perspective) inverse() (perspective, bool) {
	m := p.matrix()
	// The adjugate is the inverse up to scale, which a homography ignores.
	adj := [9]float64{
		m[4]*m[8] - m[5]*m[7], m[2]*m[7] - m[1]*m[8], m[1]*m[5] - m[2]*m[4],
		m[5]*m[6] - m[3]*m[8], m[0]*m[8] - m[2]*m[6], m[2]*m[3] - m[0]*m[5],
		m[3]*m[7] - m[4]*m[6], m[1]*m[6] - m[0]*m[7], m[0]*m[4] - m[1]*m[3],
	}
	return fromMatrix(adj)
}

// compose returns the homography that applies q, then p.
func (p perspective) compose(q perspective) perspective {
	a, b := p.matrix(), q.matrix()
	var c [9]float64
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			for k := 0; k < 3; k++ {
				c[i*3+j] += a[i*3+k] * b[k*3+j]
			}
		}
	}
	r, _ := fromMatrix(c)
	return r
}

func (p perspective) apply(u, v float64) (float64, float64) {
	w := p.g*u + p.h*v + 1
	return (p.a*u + p.b*v + p.c) / w, (p.d*u + p.e*v + p.f) / w
}

// line is a 2D line through a point along a unit direction.
type line struct {
	x, y, dx, dy float64
}

// fitLine fits a line to points by total least squares. It reports false
// for fewer than two distinct points.
func fitLine(pts [][2]float64) (line, bool) {
	if len(pts) < 2 {
		return line{}, false
	}
	var mx, my float64
	for _, p := range pts {
		mx += p[0]
		my += p[1]
	}
	n := float64(len(pts))
	mx, my = mx/n, my/n
	var sxx, sxy, syy float64
	for _, p := range pts {
		dx, dy := p[0]-mx, p[1]-my
		sxx += dx * dx
		sxy += dx * dy
		syy += dy * dy
	}
	if sxx+syy == 0 {
		return line{}, false
	}
	// Principal direction of the scatter matrix.
	theta := 0.5 * math.Atan2(2*sxy, sxx-syy)
	return line{mx, my, math.Cos(theta), math.Sin(theta)}, true
}

// rmsDistance returns the root-mean-square distance of pts from l.
func (l line) rmsDistance(pts [][2]float64) float64 {
	sum := 0.0
	for _, p := range pts {
		d := (p[0]-l.x)*l.dy - (p[1]-l.y)*l.dx
		sum += d * d
	}
	return math.Sqrt(sum / float64(len(pts)))
}

// intersect returns the intersection of two lines, reporting false when they
// are nearly parallel.
func (l line) intersect(m line) (float64, float64, bool) {
	den := l.dx*m.dy - l.dy*m.dx
	if math.Abs(den) < 1e-6 {
		return 0, 0, false
	}
	t := ((m.x-l.x)*m.dy - (m.y-l.y)*m.dx) / den
	return l.x + t*l.dx, l.y + t*l.dy, true
}
