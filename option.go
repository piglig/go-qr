package go_qr

import "image/color"

// qrCodeConfig holds configuration options for generating QR codes.
type qrCodeConfig struct {
	svgXMLHeader bool
	optimalSVG   bool
	logo         *logoConfig
}

// WithSVGXMLHeader enables the XML header and DOCTYPE prolog in SVG output.
func WithSVGXMLHeader() func(*QrCodeImgConfig) {
	return func(q *QrCodeImgConfig) {
		q.options.svgXMLHeader = true
	}
}

// WithOptimalSVG enables the compact SVG renderer that emits one <path> with
// fill-rule="evenodd" instead of one <rect> per module.
func WithOptimalSVG() func(*QrCodeImgConfig) {
	return func(q *QrCodeImgConfig) {
		q.options.optimalSVG = true
	}
}

// WithLight sets the light (background) color used by both PNG and SVG output.
func WithLight(c color.Color) func(*QrCodeImgConfig) {
	return func(q *QrCodeImgConfig) {
		q.light = c
	}
}

// WithDark sets the dark (foreground) color used by both PNG and SVG output.
func WithDark(c color.Color) func(*QrCodeImgConfig) {
	return func(q *QrCodeImgConfig) {
		q.dark = c
	}
}
