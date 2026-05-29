package go_qr

import "errors"

// Exported sentinel errors. Callers may use errors.Is to detect them without
// relying on error string contents.
var (
	// ErrInvalidConfig is returned when the image configuration is invalid
	// (non-positive scale, negative border, etc.).
	ErrInvalidConfig = errors.New("go_qr: invalid image config")

	// ErrInvalidArgument indicates a function was called with invalid inputs
	// (bad mask, unsupported version range, nil segments, etc.).
	ErrInvalidArgument = errors.New("go_qr: invalid argument")

	// ErrInvalidVersion is returned when the requested version range is
	// outside [MinVersion, MaxVersion].
	ErrInvalidVersion = errors.New("go_qr: invalid version")

	// ErrDataTooLong is returned when the data does not fit in any QR code
	// version that meets the chosen error correction level.
	ErrDataTooLong = errors.New("go_qr: data too long")

	// ErrUnencodableChar is returned when the input contains a character
	// that cannot be represented in the requested encoding mode.
	ErrUnencodableChar = errors.New("go_qr: unencodable character")

	// ErrInvalidImageOutput is returned when the output file path has an
	// unsupported extension or the output target is misconfigured.
	ErrInvalidImageOutput = errors.New("go_qr: invalid image output")

	// ErrNoQRCode is returned when no QR code could be located in the image.
	ErrNoQRCode = errors.New("go_qr: no QR code found")

	// ErrDecodeFailed is returned when a QR code was located but could not be
	// decoded (uncorrectable errors, malformed format/version info, or a
	// bitstream that does not parse).
	ErrDecodeFailed = errors.New("go_qr: decode failed")

	// ErrUnsupportedSymbol is returned for symbols this decoder does not
	// support (Micro QR, segment modes not yet implemented, etc.).
	ErrUnsupportedSymbol = errors.New("go_qr: unsupported symbol")
)
