package go_qr

import "image/color"

// WithSVGXMLHeader enables the XML header and DOCTYPE prolog in SVG output.
func WithSVGXMLHeader() Option {
	return func(q *QrCodeImgConfig) {
		q.svgXMLHeader = true
	}
}

// WithOptimalSVG enables the compact SVG renderer that emits one <path> with
// fill-rule="evenodd" instead of one <rect> per module.
func WithOptimalSVG() Option {
	return func(q *QrCodeImgConfig) {
		q.optimalSVG = true
	}
}

// WithLight sets the light (background) color used by both PNG and SVG output.
func WithLight(c color.Color) Option {
	return func(q *QrCodeImgConfig) {
		q.light = c
	}
}

// WithDark sets the dark (foreground) color used by both PNG and SVG output.
func WithDark(c color.Color) Option {
	return func(q *QrCodeImgConfig) {
		q.dark = c
	}
}
