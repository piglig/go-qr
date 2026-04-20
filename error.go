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
)

// DataTooLongException is the legacy typed error for data-too-long cases.
// It wraps ErrDataTooLong so callers can use either the type assertion or
// errors.Is to detect it.
type DataTooLongException struct {
	Msg string
}

func (d *DataTooLongException) Error() string {
	if d.Msg == "" {
		return ErrDataTooLong.Error()
	}
	return d.Msg
}

// Unwrap lets errors.Is match ErrDataTooLong.
func (d *DataTooLongException) Unwrap() error { return ErrDataTooLong }
