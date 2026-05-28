package go_qr

import (
	"fmt"
	"image"
)

// SegmentInfo describes one decoded segment in a DecodeResult.
type SegmentInfo struct {
	Mode     int    // 4-bit mode indicator (Numeric/Alphanumeric/Byte/...)
	NumChars int    // character count from the segment header
	Bytes    []byte // decoded bytes contributed by this segment
}

// DecodeResult is the structured output of DecodeDetailed.
type DecodeResult struct {
	Text     string
	Version  int
	Ecc      Ecc
	Mask     int
	Segments []SegmentInfo
}

type decodeConfig struct {
	fastPathOnly bool
}

// DecodeOption configures Decode / DecodeDetailed.
type DecodeOption func(*decodeConfig)

// WithFastPathOnly disables the (not-yet-implemented) robust fallback and
// requires the crisp, axis-aligned fast path to succeed. Use in verify/CI
// where the input is known to be freshly rendered.
func WithFastPathOnly() DecodeOption {
	return func(c *decodeConfig) { c.fastPathOnly = true }
}

// Decode reads the first QR code found in img and returns its decoded text.
//
// It first tries a fast path that assumes a crisp, axis-aligned image (the
// shape this library's renderers produce). If that fails it falls back to a
// robust path that detects the finder patterns and corrects for rotation and
// noise via an affine transform. Pass WithFastPathOnly to disable the fallback.
func Decode(img image.Image) (string, error) {
	res, err := DecodeDetailed(img)
	if err != nil {
		return "", err
	}
	return res.Text, nil
}

// DecodeDetailed is the full-fidelity entry point.
func DecodeDetailed(img image.Image, opts ...DecodeOption) (*DecodeResult, error) {
	cfg := decodeConfig{}
	for _, o := range opts {
		o(&cfg)
	}

	modules, err := fastSample(img)
	if err != nil {
		if cfg.fastPathOnly {
			return nil, err
		}
		// Robust path: finder detection + affine sampling for rotated/noisy
		// images the fast path can't handle.
		modules, err = robustSample(img)
		if err != nil {
			return nil, err
		}
	}

	data, ver, ecl, mask, err := decodeMatrix(modules)
	if err != nil {
		return nil, err
	}

	text, segs, err := parseBitstream(data, ver)
	if err != nil {
		return nil, err
	}
	return &DecodeResult{Text: text, Version: ver, Ecc: ecl, Mask: mask, Segments: segs}, nil
}

// fastSample binarizes a crisp, axis-aligned image and samples it into a module
// grid. It locates the symbol by trimming the light quiet zone to the dark
// bounding box (whose extent equals the matrix because finder patterns occupy
// three corners), derives the module pitch from the top-left finder's 7-module
// edge run, and samples each module center.
func fastSample(img image.Image) ([][]bool, error) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w < 21 || h < 21 {
		return nil, fmt.Errorf("%w: image too small (%dx%d)", ErrNoQRCode, w, h)
	}

	// One typed pass over the pixel buffer into a dark bitmap. Avoids boxing a
	// color.Color per pixel (img.At), which otherwise dominates allocations.
	bitmap := binarizeFast(img, b, w, h)
	dark := func(x, y int) bool { return bitmap[y*w+x] }

	minX, minY, maxX, maxY := w, h, -1, -1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if dark(x, y) {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	if maxX < 0 {
		return nil, fmt.Errorf("%w: no dark pixels", ErrNoQRCode)
	}

	boxW := maxX - minX + 1
	boxH := maxY - minY + 1

	// Module pitch from the top-left finder's top edge: a run of 7 dark modules
	// starting at the top-left corner of the bounding box.
	run := 0
	for x := minX; x <= maxX && dark(x, minY); x++ {
		run++
	}
	if run < 7 {
		return nil, fmt.Errorf("%w: no finder edge at top-left", ErrNoQRCode)
	}
	pitch := float64(run) / 7.0

	size := int(float64(boxW)/pitch + 0.5)
	if size < 21 || (size-17)%4 != 0 {
		return nil, fmt.Errorf("%w: inferred size %d not a valid QR size", ErrNoQRCode, size)
	}
	// Consistency check against the vertical extent.
	if vsize := int(float64(boxH)/pitch + 0.5); vsize != size {
		return nil, fmt.Errorf("%w: non-square module grid (%d vs %d)", ErrNoQRCode, size, vsize)
	}

	px := float64(boxW) / float64(size)
	py := float64(boxH) / float64(size)
	modules := make([][]bool, size)
	grid := make([]bool, size*size) // one backing allocation for all rows
	for row := 0; row < size; row++ {
		modules[row] = grid[row*size : (row+1)*size]
		cy := minY + int((float64(row)+0.5)*py)
		for col := 0; col < size; col++ {
			cx := minX + int((float64(col)+0.5)*px)
			modules[row][col] = dark(cx, cy)
		}
	}
	return modules, nil
}

// binarizeFast produces a dark/light bitmap (true == dark) via a single pass
// over the image's concrete pixel buffer where possible, falling back to At.
// Threshold is mid-gray, which is exact for the pure black/white images this
// library's renderers emit.
func binarizeFast(img image.Image, b image.Rectangle, w, h int) []bool {
	out := make([]bool, w*h)
	switch im := img.(type) {
	case *image.RGBA:
		for y := 0; y < h; y++ {
			row := (b.Min.Y+y-im.Rect.Min.Y)*im.Stride + (b.Min.X-im.Rect.Min.X)*4
			for x := 0; x < w; x++ {
				p := row + x*4
				luma := 299*int(im.Pix[p]) + 587*int(im.Pix[p+1]) + 114*int(im.Pix[p+2])
				out[y*w+x] = luma < 128*1000
			}
		}
	case *image.NRGBA:
		for y := 0; y < h; y++ {
			row := (b.Min.Y+y-im.Rect.Min.Y)*im.Stride + (b.Min.X-im.Rect.Min.X)*4
			for x := 0; x < w; x++ {
				p := row + x*4
				luma := 299*int(im.Pix[p]) + 587*int(im.Pix[p+1]) + 114*int(im.Pix[p+2])
				out[y*w+x] = luma < 128*1000
			}
		}
	case *image.Gray:
		for y := 0; y < h; y++ {
			row := (b.Min.Y+y-im.Rect.Min.Y)*im.Stride + (b.Min.X - im.Rect.Min.X)
			for x := 0; x < w; x++ {
				out[y*w+x] = im.Pix[row+x] < 128
			}
		}
	default:
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				r, g, bl, _ := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
				luma := (299*r + 587*g + 114*bl) / 1000
				out[y*w+x] = luma < 0x8000
			}
		}
	}
	return out
}
