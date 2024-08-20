package go_qr

// qrCodeConfig holds configuration options for generating QR codes.
type qrCodeConfig struct {
	// svgXMLHeader indicates whether to include the XML header in the SVG output.
	svgXMLHeader bool
}

// WithSVGXMLHeader returns a function that sets the svgXMLHeader option to true
// in the provided QrCodeImgConfig.
func WithSVGXMLHeader(header bool) func(*QrCodeImgConfig) {
	return func(q *QrCodeImgConfig) {
		q.options.svgXMLHeader = header
	}
}
