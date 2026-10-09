package qr

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"strconv"
	"strings"
)

// logoConfig holds a logo to draw over the center of the symbol.
type logoConfig struct {
	img   image.Image
	ratio float64
}

// WithLogo draws img over the center of the symbol, on a background-colored
// pad one module wide.
//
// ratio is the logo side as a fraction of the symbol side (quiet zone
// excluded); 0.15-0.22 is typical. The covered modules are lost, so the
// symbol depends on error correction to stay readable: rendering fails with
// ErrLogoTooLarge when the codewords under the logo would use more than 75%
// of the correction capacity of any error correction block, leaving the rest
// for real-world damage. Encode with WithECC(ECCHigh) to allow the largest
// logos.
func WithLogo(img image.Image, ratio float64) RenderOption {
	return func(c *renderConfig) { c.logo = &logoConfig{img: img, ratio: ratio} }
}

func (l *logoConfig) validateOptions() error {
	switch {
	case l.img == nil:
		return fmt.Errorf("%w: nil logo image", ErrInvalidArgument)
	case !(l.ratio > 0 && l.ratio < 1):
		return fmt.Errorf("%w: logo ratio %v must be in (0, 1)", ErrInvalidArgument, l.ratio)
	}
	return nil
}

// maxLogoDamage is the share of each error correction block's capacity that
// a logo may consume. The rest is left for print defects, glare and blur.
const maxLogoDamage = 0.75

// boxModules returns the side, in modules, of the padded logo box for a
// symbol of the given size. The logo side has the same parity as the symbol
// so the box centers on whole modules.
func (l *logoConfig) boxModules(qrSize int) int {
	logo := int(float64(qrSize) * l.ratio)
	if logo%2 != qrSize%2 {
		logo--
	}
	return max(logo, 1) + 2 // one module of padding on each side
}

// validate checks that the logo leaves enough error correction capacity.
func (l *logoConfig) validate(code *Code) error {
	box := l.boxModules(code.Size())
	if box >= code.Size() {
		return fmt.Errorf("%w: logo box of %d modules covers the whole %d-module symbol", ErrLogoTooLarge, box, code.Size())
	}
	if damage := logoDamage(code, box); damage > maxLogoDamage {
		return fmt.Errorf("%w: logo uses %.0f%% of the error correction capacity of ECC %v, over the %.0f%% limit (use a smaller ratio or a higher ECC level)",
			ErrLogoTooLarge, damage*100, code.ECC(), maxLogoDamage*100)
	}
	return nil
}

// logoDamage returns the largest fraction of any error correction block's
// capacity used up by a centered logo box of the given side. Every codeword
// with a module under the box counts as lost, whatever the logo looks like.
func logoDamage(code *Code, box int) float64 {
	ver, ecc := code.Version(), code.ECC()
	numBlocks := int(numErrorCorrectionBlocks[ecc][ver])
	eccLen := int(eccCodeWordsPerBlock[ecc][ver])
	raw := numRawDataModules(ver) / 8

	// Placement order to block: the inverse of the interleaving in
	// addEccAndInterLeave.
	numShort := numBlocks - raw%numBlocks
	shortLen := raw / numBlocks
	blockOf := make([]int, 0, raw)
	for i := 0; i <= shortLen; i++ {
		for j := 0; j < numBlocks; j++ {
			if i != shortLen-eccLen || j >= numShort {
				blockOf = append(blockOf, j)
			}
		}
	}

	codewordAt := getTemplate(ver).codewordAt
	lost := make([]bool, raw)
	perBlock := make([]int, numBlocks)
	start := (code.Size() - box) / 2
	for y := start; y < start+box; y++ {
		for x := start; x < start+box; x++ {
			if cw := codewordAt[y][x]; cw >= 0 && !lost[cw] {
				lost[cw] = true
				perBlock[blockOf[cw]]++
			}
		}
	}
	capacity := float64(correctableErrors(ver, ecc))
	worst := 0.0
	for _, n := range perBlock {
		worst = max(worst, float64(n)/capacity)
	}
	return worst
}

// correctableErrors returns how many codeword errors each block of a symbol
// can correct: half its error correction codewords, less the
// misdecode protection codewords p of ISO/IEC 18004 Table 9, which the
// smallest symbols reserve for detecting errors rather than correcting
// them. Readers that follow the standard correct no more.
func correctableErrors(ver int, ecc ECC) int {
	p := 0
	switch {
	case ver == 1 && ecc == ECCLow:
		p = 3
	case ver == 1 && ecc == ECCMedium, ver == 2 && ecc == ECCLow:
		p = 2
	case ver == 1, ver == 3 && ecc == ECCLow:
		p = 1
	}
	return (int(eccCodeWordsPerBlock[ecc][ver]) - p) / 2
}

// rects returns the padded box and the inner logo area in output units.
func (l *logoConfig) rects(qrSize int, c *renderConfig) (box, inner image.Rectangle) {
	b := l.boxModules(qrSize)
	min := (c.quietZone + (qrSize-b)/2) * c.scale
	box = image.Rect(min, min, min+b*c.scale, min+b*c.scale)
	return box, box.Inset(c.scale)
}

// overlay draws the pad and the scaled logo onto img.
func (l *logoConfig) overlay(img *image.RGBA, qrSize int, c *renderConfig) {
	box, inner := l.rects(qrSize, c)
	draw.Draw(img, box, image.NewUniform(c.bg), image.Point{}, draw.Src)
	drawScaled(img, inner, l.img)
}

// drawScaled draws src into r of dst with nearest-neighbor scaling, blending
// over what is already there.
func drawScaled(dst *image.RGBA, r image.Rectangle, src image.Image) {
	sb := src.Bounds()
	if r.Empty() || sb.Empty() {
		return
	}
	scaled := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	for y := 0; y < r.Dy(); y++ {
		sy := sb.Min.Y + y*sb.Dy()/r.Dy()
		for x := 0; x < r.Dx(); x++ {
			scaled.Set(x, y, src.At(sb.Min.X+x*sb.Dx()/r.Dx(), sy))
		}
	}
	draw.Draw(dst, r, scaled, image.Point{}, draw.Over)
}

// writeSVG writes the pad and the logo, embedded as a PNG data URI.
func (l *logoConfig) writeSVG(sb *strings.Builder, qrSize int, c *renderConfig) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, l.img); err != nil {
		return fmt.Errorf("qr: encode logo for SVG: %w", err)
	}
	box, inner := l.rects(qrSize, c)
	if !colorIsTransparent(c.bg) {
		sb.WriteString("\t<rect" + svgRectAttrs(box) + " fill=\"" + colorToSVG(c.bg) + "\"/>\n")
	}
	sb.WriteString("\t<image" + svgRectAttrs(inner) + " href=\"data:image/png;base64,")
	sb.WriteString(base64.StdEncoding.EncodeToString(buf.Bytes()))
	sb.WriteString("\"/>\n")
	return nil
}

func svgRectAttrs(r image.Rectangle) string {
	return ` x="` + strconv.Itoa(r.Min.X) + `" y="` + strconv.Itoa(r.Min.Y) +
		`" width="` + strconv.Itoa(r.Dx()) + `" height="` + strconv.Itoa(r.Dy()) + `"`
}
