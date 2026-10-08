# Standards support

go-qr implements QR Code Model 2 as specified in ISO/IEC 18004:2015. This
page lists what the encoder and decoder support, by section of the standard.

| Feature | Encoder | Decoder | Notes |
| --- | --- | --- | --- |
| Versions 1–40 | ✅ | ✅ | |
| Error correction L, M, Q, H | ✅ | ✅ | Reed–Solomon over GF(2⁸). |
| Numeric, alphanumeric, byte modes | ✅ | ✅ | |
| Kanji mode (Shift_JIS) | ✅ | ✅ | |
| Mixed modes, optimal segmentation | ✅ | — | Default for `Encode`. |
| ECI | ✅ | ✅ | Encoder: any assignment via `ECISegment`, UTF-8 via `WithUTF8ECI`. Decoder: see [character sets](#character-sets). |
| Structured append | ✅ | ✅ | *Since v2.1.* Up to 16 symbols. |
| FNC1 in first position (GS1) | ✅ | ✅ | *Since v2.1.* |
| FNC1 in second position (AIM) | ❌ | ❌ | Reported as `ErrUnsupported`. |
| Mask selection by penalty score | ✅ | — | All eight masks scored per §7.8.3. |
| Format and version information with BCH correction | ✅ | ✅ | Decoder corrects up to 3 bit errors. |
| Mirror images | — | ✅ | |
| Reflectance reversal (light on dark) | — | ✅ | |
| Micro QR, rMQR, Model 1 | ❌ | ❌ | |
| Hanzi mode (GB/T 18284) | ❌ | ❌ | Reported as `ErrUnsupported`. |

## Character sets

| ECI | Character set | Decoder |
| --- | --- | --- |
| 1, 3 | ISO-8859-1 | ✅ |
| 20 | Shift_JIS | ✅ |
| 26 | UTF-8 | ✅ |
| 27, 170 | ASCII | ✅ |
| other | | `ErrUnsupported`; raw bytes are in `DecodeResult.Segments` |

Without an ECI, the decoder reads byte segments as UTF-8 when they are valid
UTF-8, and otherwise as Shift_JIS or Windows-1252 (a superset of ISO-8859-1
for text), whichever reads as more plausible text; see
[Decoding](../guides/decoding.md#text-encodings).

## Payload formats

| Type | Specification |
| --- | --- |
| `WiFi` | ZXing Wi-Fi network config format |
| `VCard` | NTT DoCoMo MECARD |
| `Contact` | vCard 3.0, RFC 2426 |
| `Event` | iCalendar VEVENT, RFC 5545 |
| `OTP` | Google Authenticator Key URI Format |
| `EPC` | EPC069-12 version 002 (reads 001 and 002) |
| `Email`, `SMS`, `Tel`, `Geo` | RFC 6068, RFC 5724, RFC 3966, RFC 5870 |
