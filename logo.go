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
// ErrLogoTooLarge when the covered area exceeds a conservative budget for the
// symbol's ECC level (5% L, 12% M, 20% Q, 25% H). Encode with
// WithECC(ECCHigh) to allow the largest logos.
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

// eccRecoveryBudget returns the fraction of modules that can safely be
// covered at the given ECC level. The values are below the nominal recovery
// capacity because covered modules also hit format information, alignment
// patterns and codewords unevenly.
func eccRecoveryBudget(ecc ECC) float64 {
	switch ecc {
	case ECCMedium:
		return 0.12
	case ECCQuartile:
		return 0.20
	case ECCHigh:
		return 0.25
	}
	return 0.05
}

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

// validate checks that the logo fits the error correction budget of code.
func (l *logoConfig) validate(code *Code) error {
	box := l.boxModules(code.Size())
	if box >= code.Size() {
		return fmt.Errorf("%w: logo box of %d modules covers the whole %d-module symbol", ErrLogoTooLarge, box, code.Size())
	}
	covered := float64(box*box) / float64(code.Size()*code.Size())
	if budget := eccRecoveryBudget(code.ECC()); covered > budget {
		return fmt.Errorf("%w: logo covers %.1f%% of modules, over the %.0f%% budget for ECC %v",
			ErrLogoTooLarge, covered*100, budget*100, code.ECC())
	}
	return nil
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
