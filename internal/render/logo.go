package render

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"strconv"
	"strings"

	"github.com/piglig/go-qr/v2/internal/errs"
)

// BoxModules returns the side, in modules, of the padded logo box for a
// symbol of the given size. The logo side has the same parity as the symbol
// so the box centers on whole modules.
func (l *Logo) BoxModules(qrSize int) int {
	logo := int(float64(qrSize) * l.Ratio)
	if logo%2 != qrSize%2 {
		logo--
	}
	return max(logo, 1) + 2 // one module of padding on each side
}

// Rects returns the padded box and the inner logo area in output units.
func (l *Logo) Rects(qrSize int, c *Config) (box, inner image.Rectangle) {
	b := l.BoxModules(qrSize)
	min := (c.QuietZone + (qrSize-b)/2) * c.Scale
	box = image.Rect(min, min, min+b*c.Scale, min+b*c.Scale)
	return box, box.Inset(c.Scale)
}

// overlay draws the pad and the scaled logo onto img.
func (l *Logo) overlay(img *image.RGBA, qrSize int, c *Config) {
	box, inner := l.Rects(qrSize, c)
	draw.Draw(img, box, image.NewUniform(c.BG), image.Point{}, draw.Src)
	drawScaled(img, inner, l.Img)
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
func (l *Logo) writeSVG(sb *strings.Builder, qrSize int, c *Config) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, l.Img); err != nil {
		return fmt.Errorf("%w: encode logo for SVG: %v", errs.ErrInvalidArgument, err)
	}
	box, inner := l.Rects(qrSize, c)
	if !colorIsTransparent(c.BG) {
		sb.WriteString("\t<rect" + svgRectAttrs(box) + " fill=\"" + ColorToSVG(c.BG) + "\"/>\n")
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
