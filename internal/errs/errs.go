// Package errs defines the sentinel errors of package qr, which re-exports
// them, so that the internal packages can wrap them too.
package errs

import "errors"

// The sentinels; package qr documents each.
var (
	ErrInvalidArgument = errors.New("qr: invalid argument")
	ErrInvalidVersion  = errors.New("qr: invalid version")
	ErrDataTooLong     = errors.New("qr: data too long")
	ErrUnencodableChar = errors.New("qr: unencodable character")
	ErrLogoTooLarge    = errors.New("qr: logo too large")
	ErrUnreadable      = errors.New("qr: rendering is not readable")
	ErrNotFound        = errors.New("qr: no QR code found")
	ErrDecodeFailed    = errors.New("qr: decode failed")
	ErrUnsupported     = errors.New("qr: unsupported symbol")
)
