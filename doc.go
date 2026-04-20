// Package go_qr generates QR codes (Model 2, versions 1-40, all four
// error-correction levels) and renders them to PNG, SVG, or image.Image.
//
// # Quick start
//
//	qr, err := go_qr.EncodeText("Hello, world!", go_qr.Low)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	cfg := go_qr.NewQrCodeImgConfig(10, 4)
//	_ = qr.PNG(cfg, "hello.png")
//	_ = qr.SVG(cfg, "hello.svg")
//
// # Configuration
//
// NewQrCodeImgConfig accepts functional options. Colors are read from the
// config for both PNG and SVG output:
//
//   - WithLight / WithDark set the background and foreground color.
//   - WithSVGXMLHeader emits the XML + DOCTYPE prolog.
//   - WithOptimalSVG emits a single <path> with fill-rule="evenodd"
//     (smaller, connected regions merged into one path).
//   - WithLogo embeds a centered logo, validated against the ECC budget.
//
// # In-memory rendering
//
// ToPNGBytes, ToSVGBytes, and ToImage return the rendered output directly,
// avoiding a file round-trip when writing to HTTP responses, archives, or
// further image processing.
//
// # Batch API
//
// EncodeBatch and RenderBatch encode/render many inputs concurrently and
// return results in input order. Per-item failures do not cancel the batch.
//
// # Structured payloads
//
// The sub-package github.com/piglig/go-qr/payload builds canonical strings
// for Wi-Fi, vCard/MECARD, mailto, SMS, tel, geo, and URL payloads that
// standard QR scanners recognize.
//
// # Errors
//
// Errors returned by this package wrap sentinel values so callers can
// classify them with errors.Is:
//
//	ErrInvalidConfig, ErrInvalidArgument, ErrInvalidVersion,
//	ErrDataTooLong, ErrUnencodableChar, ErrInvalidImageOutput
//
// DataTooLongException is retained for API compatibility and unwraps to
// ErrDataTooLong.
package go_qr
