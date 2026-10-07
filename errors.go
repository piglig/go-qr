package qr

import "errors"

// Sentinel errors. Every error returned by this package wraps one of them, so
// callers can test for a category with errors.Is.
var (
	// ErrInvalidArgument reports an invalid option or argument, such as a mask
	// outside 0-7 or an unknown error correction level.
	ErrInvalidArgument = errors.New("qr: invalid argument")

	// ErrInvalidVersion reports a version range outside
	// [MinVersion, MaxVersion] or with min > max.
	ErrInvalidVersion = errors.New("qr: invalid version")

	// ErrDataTooLong reports data that does not fit in the largest allowed
	// version at the requested error correction level.
	ErrDataTooLong = errors.New("qr: data too long")

	// ErrUnencodableChar reports a character that the requested segment mode
	// cannot represent.
	ErrUnencodableChar = errors.New("qr: unencodable character")

	// ErrInvalidConfig reports an invalid image configuration (non-positive
	// scale, negative border, etc.).
	ErrInvalidConfig = errors.New("qr: invalid image config")

	// ErrInvalidImageOutput reports an unsupported output file extension.
	ErrInvalidImageOutput = errors.New("qr: invalid image output")

	// ErrNotFound reports that no QR Code could be located in an image.
	ErrNotFound = errors.New("qr: no QR code found")

	// ErrDecodeFailed reports a located symbol that could not be decoded:
	// too many errors, unreadable format information, or a malformed
	// bitstream.
	ErrDecodeFailed = errors.New("qr: decode failed")

	// ErrUnsupported reports a symbol feature this decoder does not handle.
	ErrUnsupported = errors.New("qr: unsupported symbol")
)
