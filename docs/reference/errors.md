# Errors

Every error returned by the package wraps one of the sentinel values below.
Test for a category with `errors.Is`; the message adds details such as the
bit count that did not fit, and may change between releases.

```go
code, err := qr.Encode(text, qr.WithECC(qr.ECCHigh))
switch {
case errors.Is(err, qr.ErrDataTooLong):
	code, err = qr.Encode(text, qr.WithECC(qr.ECCLow)) // trade robustness for capacity
case err != nil:
	return err
}
```

| Sentinel | Returned by | When |
| --- | --- | --- |
| `ErrInvalidArgument` | all | An invalid option or argument: an unknown ECC level, a mask outside 0–7, a zero scale, a negative quiet zone, a nil color or image, a logo ratio outside (0, 1), an unknown shape, a NaN gradient angle, an ECI number above 999999, an incomplete structured append sequence passed to `JoinStructuredAppend`. |
| `ErrInvalidVersion` | encoding | A version range outside 1–40, or with min above max. |
| `ErrDataTooLong` | encoding | The data does not fit the largest allowed version at the requested error correction level, or more than 16 symbols would be needed by `EncodeStructured`. |
| `ErrUnencodableChar` | segment constructors | A character the requested mode cannot hold, such as a lowercase letter in `AlphanumericSegment`. |
| `ErrLogoTooLarge` | rendering, `Verify` | The codewords under the logo would use more than 75% of the correction capacity of an error correction block. |
| `ErrUnreadable` *Since v2.1* | `Verify` | The rendering could not be decoded back to the code's data, or a dark color lacks contrast with the background. |
| `ErrNotFound` | `Decode` | No symbol was located: the image is too small, has no contrast, or contains no finder patterns. |
| `ErrDecodeFailed` | `Decode` | A symbol was located but not read: unreadable format information, too many errors to correct, or a malformed bitstream. |
| `ErrUnsupported` | `Decode` | The symbol uses a feature the decoder does not implement: an unsupported ECI, FNC1 in second position, or Hanzi mode. |

The `payload` package has its own sentinels: `ErrUnrecognized` and
`ErrMalformed` from `Parse`, and `ErrInvalidEPC` from `EPC.Validate`.
