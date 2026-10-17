package qr

import "github.com/piglig/go-qr/v2/internal/errs"

// Sentinel errors. Every error returned by this package wraps one of them, so
// callers can test for a category with errors.Is, with three exceptions: the
// Write methods return the error of their io.Writer, wrapped with %w;
// DecodeContext, DecodeAllContext and Batch return the error of a context
// that stopped them; and a BatchResult reports a job that panicked, a bug in
// this package, with an error of its own.
var (
	// ErrInvalidArgument reports an invalid option or argument, such as a mask
	// outside 0-7, an unknown error correction level or a zero scale.
	ErrInvalidArgument = errs.ErrInvalidArgument

	// ErrInvalidVersion reports a version range outside
	// [MinVersion, MaxVersion] or with min > max.
	ErrInvalidVersion = errs.ErrInvalidVersion

	// ErrDataTooLong reports data that does not fit in the largest allowed
	// version at the requested error correction level.
	ErrDataTooLong = errs.ErrDataTooLong

	// ErrUnencodableChar reports a character that the requested segment mode
	// cannot represent.
	ErrUnencodableChar = errs.ErrUnencodableChar

	// ErrLogoTooLarge reports a logo that covers more modules than the
	// symbol's error correction can be expected to recover. See WithLogo.
	ErrLogoTooLarge = errs.ErrLogoTooLarge

	// ErrUnreadable reports a rendering that Code.Verify could not read
	// back, or whose colors phone scanners are likely to fail on.
	ErrUnreadable = errs.ErrUnreadable

	// ErrNotFound reports that no QR Code could be located in an image.
	ErrNotFound = errs.ErrNotFound

	// ErrDecodeFailed reports a located symbol that could not be decoded:
	// too many errors, unreadable format information, or a malformed
	// bitstream.
	ErrDecodeFailed = errs.ErrDecodeFailed

	// ErrUnsupported reports a symbol feature this decoder does not handle.
	ErrUnsupported = errs.ErrUnsupported
)
