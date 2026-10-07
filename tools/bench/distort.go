package bench

import (
	"image"
	"image/color"
	"math"
	"math/rand"
)

// Distortion describes how a camera sees a flat printed symbol.
type Distortion struct {
	Tilt        float64 // angle between the symbol plane and the image plane, degrees
	TiltAxis    float64 // direction of the tilt axis in the symbol plane, degrees
	Rotate      float64 // in-plane rotation, degrees
	PxPerModule float64 // module pitch at the symbol center, pixels
	Blur        float64 // Gaussian blur sigma, pixels
	Noise       float64 // additive Gaussian noise sigma, gray levels
	K1          float64 // radial lens distortion; negative is barrel
}

// cameraDistance is the camera's distance from the symbol in units of the
// symbol's width, about what a phone held to fill the frame sees.
const cameraDistance = 1.8

// Distort renders a symbol as a pinhole camera would see it. src is the
// symbol rendered with qz modules of quiet zone at scale pixels per module.
func Distort(src *image.Gray, scale int, d Distortion, rng *rand.Rand) *image.Gray {
	img, _ := DistortWithTruth(src, scale, 0, d, rng)
	return img
}

// DistortWithTruth is Distort that also returns where module coordinates
// (u, v) of the symbol land in the image, ignoring lens distortion. qz is
// the quiet zone of src in modules; with qz 0 the function maps
// coordinates that include the quiet zone.
func DistortWithTruth(src *image.Gray, scale, qz int, d Distortion, rng *rand.Rand) (*image.Gray, func(u, v float64) (float64, float64)) {
	side := float64(src.Bounds().Dx()) / float64(scale) // modules, including quiet zone
	h := cameraHomography(d, side)
	inv, ok := invert3(h)
	if !ok {
		panic("bench: singular camera homography")
	}

	// Bounding box of the projected symbol, plus a margin.
	minX, minY, maxX, maxY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, c := range [][2]float64{{-side / 2, -side / 2}, {side / 2, -side / 2}, {side / 2, side / 2}, {-side / 2, side / 2}} {
		x, y := apply3(h, c[0], c[1])
		minX, minY = math.Min(minX, x), math.Min(minY, y)
		maxX, maxY = math.Max(maxX, x), math.Max(maxY, y)
	}
	const margin = 12
	w := int(maxX-minX) + 2*margin
	ht := int(maxY-minY) + 2*margin
	ox, oy := minX-margin, minY-margin
	cx, cy := (minX+maxX)/2, (minY+maxY)/2
	norm := math.Max(maxX-minX, maxY-minY) / 2

	dst := image.NewGray(image.Rect(0, 0, w, ht))
	for y := 0; y < ht; y++ {
		for x := 0; x < w; x++ {
			px, py := float64(x)+0.5+ox, float64(y)+0.5+oy
			if d.K1 != 0 {
				// The lens moves an ideal point at radius r to r·(1+k1·r²);
				// to first order, the pixel at radius r shows the ideal point
				// at r·(1−k1·r²).
				rx, ry := (px-cx)/norm, (py-cy)/norm
				f := 1 - d.K1*(rx*rx+ry*ry)
				px, py = cx+rx*f*norm, cy+ry*f*norm
			}
			mx, my := apply3(inv, px, py) // symbol-plane coordinates in modules
			dst.Pix[y*dst.Stride+x] = sampleBilinear(src, (mx+side/2)*float64(scale), (my+side/2)*float64(scale))
		}
	}
	if d.Blur > 0 {
		dst = gaussianBlur(dst, d.Blur)
	}
	if d.Noise > 0 {
		for i, v := range dst.Pix {
			dst.Pix[i] = clamp8(float64(v) + rng.NormFloat64()*d.Noise)
		}
	}
	truth := func(u, v float64) (float64, float64) {
		x, y := apply3(h, u+float64(qz)-side/2, v+float64(qz)-side/2)
		return x - ox, y - oy
	}
	return dst, truth
}

// cameraHomography maps symbol-plane coordinates (modules, origin at the
// center) to image pixels for the camera described by d.
func cameraHomography(d Distortion, side float64) [9]float64 {
	rad := math.Pi / 180
	// The plane is rotated in-plane, then tilted about an axis lying in the
	// plane at angle TiltAxis.
	a, t, r := d.TiltAxis*rad, d.Tilt*rad, d.Rotate*rad
	ux, uy := math.Cos(a), math.Sin(a) // tilt axis
	ct, st := math.Cos(t), math.Sin(t)
	// Rodrigues rotation about (ux, uy, 0) by t.
	R := [9]float64{
		ct + ux*ux*(1-ct), ux * uy * (1 - ct), uy * st,
		uy * ux * (1 - ct), ct + uy*uy*(1-ct), -ux * st,
		-uy * st, ux * st, ct,
	}
	cr, sr := math.Cos(r), math.Sin(r)
	Rz := [9]float64{cr, -sr, 0, sr, cr, 0, 0, 0, 1}
	R = mul3(R, Rz)
	// Camera at distance D looking at the symbol center, focal length D so
	// that one module at the center maps to PxPerModule pixels.
	dist := cameraDistance * side
	f := dist * d.PxPerModule
	return [9]float64{
		f * R[0], f * R[1], 0,
		f * R[3], f * R[4], 0,
		R[6], R[7], dist,
	}
}

func apply3(h [9]float64, x, y float64) (float64, float64) {
	w := h[6]*x + h[7]*y + h[8]
	return (h[0]*x + h[1]*y + h[2]) / w, (h[3]*x + h[4]*y + h[5]) / w
}

func mul3(a, b [9]float64) [9]float64 {
	var c [9]float64
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			for k := 0; k < 3; k++ {
				c[i*3+j] += a[i*3+k] * b[k*3+j]
			}
		}
	}
	return c
}

func invert3(m [9]float64) ([9]float64, bool) {
	det := m[0]*(m[4]*m[8]-m[5]*m[7]) - m[1]*(m[3]*m[8]-m[5]*m[6]) + m[2]*(m[3]*m[7]-m[4]*m[6])
	if math.Abs(det) < 1e-12 {
		return [9]float64{}, false
	}
	return [9]float64{
		(m[4]*m[8] - m[5]*m[7]) / det, (m[2]*m[7] - m[1]*m[8]) / det, (m[1]*m[5] - m[2]*m[4]) / det,
		(m[5]*m[6] - m[3]*m[8]) / det, (m[0]*m[8] - m[2]*m[6]) / det, (m[2]*m[3] - m[0]*m[5]) / det,
		(m[3]*m[7] - m[4]*m[6]) / det, (m[1]*m[6] - m[0]*m[7]) / det, (m[0]*m[4] - m[1]*m[3]) / det,
	}, true
}

// sampleBilinear reads src at pixel coordinates (x, y), white outside.
func sampleBilinear(src *image.Gray, x, y float64) uint8 {
	x, y = x-0.5, y-0.5
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	fx, fy := x-float64(x0), y-float64(y0)
	at := func(x, y int) float64 {
		if x < 0 || y < 0 || x >= w || y >= h {
			return 255
		}
		return float64(src.Pix[y*src.Stride+x])
	}
	top := at(x0, y0)*(1-fx) + at(x0+1, y0)*fx
	bot := at(x0, y0+1)*(1-fx) + at(x0+1, y0+1)*fx
	return clamp8(top*(1-fy) + bot*fy)
}

func gaussianBlur(src *image.Gray, sigma float64) *image.Gray {
	r := int(math.Ceil(3 * sigma))
	k := make([]float64, 2*r+1)
	sum := 0.0
	for i := range k {
		x := float64(i - r)
		k[i] = math.Exp(-x * x / (2 * sigma * sigma))
		sum += k[i]
	}
	for i := range k {
		k[i] /= sum
	}
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	tmp := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := 0.0
			for i, kv := range k {
				xx := min(max(x+i-r, 0), w-1)
				v += kv * float64(src.Pix[y*src.Stride+xx])
			}
			tmp[y*w+x] = v
		}
	}
	dst := image.NewGray(src.Bounds())
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := 0.0
			for i, kv := range k {
				yy := min(max(y+i-r, 0), h-1)
				v += kv * tmp[yy*w+x]
			}
			dst.Pix[y*dst.Stride+x] = clamp8(v)
		}
	}
	return dst
}

func clamp8(v float64) uint8 {
	return uint8(math.Max(0, math.Min(255, math.Round(v))))
}

// grayOf converts any image to *image.Gray.
func grayOf(img image.Image) *image.Gray {
	if g, ok := img.(*image.Gray); ok {
		return g
	}
	b := img.Bounds()
	g := image.NewGray(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			g.Set(x, y, color.GrayModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)))
		}
	}
	return g
}
