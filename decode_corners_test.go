package qr

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"testing"
)

// symbolBox returns the outer corners of a symbol rendered at scale s with
// quiet zone qz, top-left first and clockwise.
func symbolBox(code *Code, s, qz int) [4][2]float64 {
	a, b := float64(qz*s), float64((qz+code.Size())*s)
	return [4][2]float64{{a, a}, {b, a}, {b, b}, {a, b}}
}

func assertCorners(t *testing.T, want [4][2]float64, got [4]image.Point, tol float64) {
	t.Helper()
	for i, w := range want {
		if d := math.Hypot(float64(got[i].X)-w[0], float64(got[i].Y)-w[1]); d > tol {
			t.Errorf("corner %d = %v, want (%.1f, %.1f) within %.1f", i, got[i], w[0], w[1], tol)
		}
	}
}

func mustDecode(t *testing.T, img image.Image, opts ...DecodeOption) *DecodeResult {
	t.Helper()
	res, err := Decode(img, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func TestDecodeCornersAxisAligned(t *testing.T) {
	code := mustEncode(t, "corners")
	img := mustImage(t, code, WithScale(6), WithQuietZone(3))
	want := symbolBox(code, 6, 3)
	for _, opts := range [][]DecodeOption{nil, {WithFastPathOnly()}} {
		res := mustDecode(t, img, opts...)
		assertCorners(t, want, res.Corners, 0)
		assertFalse(t, res.Inverted)
	}
}

// TestDecodeCornersImageBounds decodes a sub-image whose bounds do not start
// at the origin and expects corners in the image's own coordinates.
func TestDecodeCornersImageBounds(t *testing.T) {
	code := mustEncode(t, "offset")
	sym := mustImage(t, code, WithScale(5))
	canvas := image.NewRGBA(image.Rect(0, 0, 400, 400))
	draw.Draw(canvas, canvas.Bounds(), image.White, image.Point{}, draw.Src)
	off := image.Pt(97, 61)
	draw.Draw(canvas, sym.Bounds().Add(off), sym, image.Point{}, draw.Src)
	sub := canvas.SubImage(image.Rect(40, 30, 400, 400))

	want := symbolBox(code, 5, defaultQuietZone)
	for i := range want {
		want[i][0] += float64(off.X)
		want[i][1] += float64(off.Y)
	}
	assertCorners(t, want, mustDecode(t, sub).Corners, 0)
}

func TestDecodeCornersRotated(t *testing.T) {
	code := mustEncode(t, "rotated corners")
	img := mustImage(t, code, WithScale(6))
	const theta = 0.3
	rot := rotateGray(img, theta)
	// rotateGray turns the image by theta about its center.
	c := float64(img.Bounds().Dx()) / 2
	sin, cos := math.Sin(theta), math.Cos(theta)
	var want [4][2]float64
	for i, p := range symbolBox(code, 6, defaultQuietZone) {
		dx, dy := p[0]-c, p[1]-c
		want[i] = [2]float64{cos*dx - sin*dy + c, sin*dx + cos*dy + c}
	}
	assertCorners(t, want, mustDecode(t, rot).Corners, 2)
}

func TestDecodeCornersPerspective(t *testing.T) {
	code := mustEncode(t, "perspective corners")
	img := mustImage(t, code, WithScale(6))
	w := float64(img.Bounds().Dx())
	q := [4][2]float64{{0.08 * w, 0.1 * w}, {0.92 * w, 0.04 * w}, {0.96 * w, 0.96 * w}, {0.04 * w, 0.9 * w}}
	warped := warpPerspective(img, q, int(w), int(w))
	p, _ := quadToQuad([4][2]float64{{0, 0}, {w, 0}, {w, w}, {0, w}}, q)
	var want [4][2]float64
	for i, c := range symbolBox(code, 6, defaultQuietZone) {
		want[i][0], want[i][1] = p.apply(c[0], c[1])
	}
	assertCorners(t, want, mustDecode(t, warped).Corners, 2)
}

// TestDecodeCornersMirrored expects the corners of a mirrored symbol in its
// own orientation: its top-left corner, mirrored, is at the image's right.
func TestDecodeCornersMirrored(t *testing.T) {
	code := mustEncode(t, "mirrored corners")
	img := mustImage(t, code, WithScale(5))
	res := mustDecode(t, mirror(img))
	assertTrue(t, res.Mirrored)
	w := float64(img.Bounds().Dx())
	var want [4][2]float64
	for i, c := range symbolBox(code, 5, defaultQuietZone) {
		want[i] = [2]float64{w - c[0], c[1]}
	}
	assertCorners(t, want, res.Corners, 2)
}

func TestDecodeCornersInverted(t *testing.T) {
	code := mustEncode(t, "light on dark")
	img := mustImage(t, code, WithScale(5), WithForeground(color.White), WithBackground(color.Black))
	for _, opts := range [][]DecodeOption{nil, {WithFastPathOnly()}} {
		res := mustDecode(t, img, opts...)
		assertTrue(t, res.Inverted)
		assertCorners(t, symbolBox(code, 5, defaultQuietZone), res.Corners, 0)
	}
	res := mustDecode(t, rotateGray(img, 0.1))
	assertTrue(t, res.Inverted)
}

// TestDecodeCornersCoarseScale locates a symbol in a downsampled image, as
// Decode does for screens and very large modules, and expects corners at
// full resolution.
func TestDecodeCornersCoarseScale(t *testing.T) {
	code := mustEncode(t, "coarse")
	img := rotateGray(mustImage(t, code, WithScale(12)), 0.2)
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	l := toLuma(img)
	small, sw, sh := halve(l, w, h)
	res, err := robustDecode(hybridThresholds(small, sw, sh), false, 2, l, w, h, decodeGrid)
	if err != nil {
		t.Fatal(err)
	}
	full := mustDecode(t, img)
	assertCorners(t, [4][2]float64{
		{float64(full.Corners[0].X), float64(full.Corners[0].Y)},
		{float64(full.Corners[1].X), float64(full.Corners[1].Y)},
		{float64(full.Corners[2].X), float64(full.Corners[2].Y)},
		{float64(full.Corners[3].X), float64(full.Corners[3].Y)},
	}, res.Corners, 3)
}
