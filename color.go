package go_qr

import (
	"fmt"
	"image/color"
)

// colorToSVGHex formats a color.Color as an SVG-compatible string.
// Opaque colors return #RRGGBB; translucent colors return rgba(r,g,b,a)
// (broadly supported, unlike 8-digit hex which is SVG 2 only).
func colorToSVGHex(c color.Color) string {
	r, g, b, a := c.RGBA()
	if a == 0xffff {
		return fmt.Sprintf("#%02X%02X%02X", r>>8, g>>8, b>>8)
	}
	return fmt.Sprintf("rgba(%d,%d,%d,%.3f)", r>>8, g>>8, b>>8, float64(a)/0xffff)
}

// colorIsTransparent reports whether the color has zero alpha.
// When the light color is transparent, the SVG background rectangle is omitted.
func colorIsTransparent(c color.Color) bool {
	_, _, _, a := c.RGBA()
	return a == 0
}
