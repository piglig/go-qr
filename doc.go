// Package qr encodes, renders and decodes QR Codes (ISO/IEC 18004 Model 2,
// versions 1-40, all four error correction levels) with no dependencies
// outside the standard library.
//
// # Encoding
//
// Encode chooses the smallest version that holds the text, switches between
// numeric, alphanumeric, byte and Kanji modes to minimize the bit count,
// raises the error correction level into spare capacity and picks the mask
// with the lowest penalty. Options override each choice:
//
//	code, err := qr.Encode("Hello, world!")
//	code, err := qr.Encode(text, qr.WithECC(qr.ECCHigh), qr.WithVersionRange(5, 10))
//
// EncodeBytes encodes binary data, and EncodeSegments encodes segments built
// with NumericSegment, AlphanumericSegment, BytesSegment, KanjiSegment and
// ECISegment exactly as given. WithGS1 marks a GS1 element string, and
// EncodeStructured spreads text too long for one symbol over up to 16
// linked by structured append.
//
// # Rendering
//
// A Code renders to PNG, SVG, an *image.RGBA or Unicode text. Render options
// set the module scale, quiet zone, colors and an optional center logo:
//
//	png, err := code.PNG(qr.WithScale(8))
//	err = code.WriteSVG(w, qr.WithForeground(navy), qr.WithBackground(color.Transparent))
//	fmt.Print(code)
//
// WithModuleShape, WithFinderShape, WithFinderColor and WithGradient style
// the code. Code.Verify decodes a rendering to confirm it is readable
// before it is published.
//
// # Decoding
//
// Decode locates and reads a symbol in an image, from crisp renderings to
// camera photos: rotation, perspective, lens distortion, blur, glare,
// noise, low contrast, a damaged finder, inverted and mirrored symbols. It
// returns the text along with the version, error correction level, mask,
// segments and the symbol's corners in the image:
//
//	res, err := qr.Decode(img)
//
// DecodeAll reads every symbol in an image, in reading order, and
// JoinStructuredAppend joins the symbols of a structured append sequence.
//
// # Batches
//
// Batch encodes and renders many jobs concurrently, in order, with
// cancellation through a context.
//
// # Payloads
//
// The payload subpackage builds the strings that phone scanners act on,
// such as Wi-Fi networks, contacts, calendar events, 2FA accounts, SEPA
// payments, email, SMS, phone numbers, locations and URLs, and parses
// decoded text back into them.
//
// # Errors
//
// Every error wraps one of the sentinel values ErrInvalidArgument,
// ErrInvalidVersion, ErrDataTooLong, ErrUnencodableChar, ErrLogoTooLarge,
// ErrUnreadable, ErrNotFound, ErrDecodeFailed or ErrUnsupported; test for
// them with errors.Is.
//
// All functions and methods are safe for concurrent use, and a *Code is
// immutable.
//
// Guides, a tutorial and an interactive playground are linked from
// https://github.com/piglig/go-qr.
package qr
