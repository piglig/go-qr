# Decoding

`Decode` finds a QR Code in any `image.Image` and reads it:

```go
f, _ := os.Open("photo.jpg")
img, _, err := image.Decode(f) // import _ "image/jpeg" to register the format
if err != nil {
	return err
}
res, err := qr.Decode(img)
if err != nil {
	return err
}
fmt.Println(res.Text)
```

## The result

| Field | Meaning |
| --- | --- |
| `Text` | The payload as UTF-8, with byte segments interpreted by their ECI (see below). |
| `Version`, `ECC`, `Mask` | Symbol parameters. |
| `Mirrored` | The symbol was read from a mirror image. |
| `Segments` | Each segment: `Mode`, `NumChars`, the `ECI` in effect (-1 for none), and the raw `Data` (digits, characters, bytes, or Shift_JIS for Kanji). |
| `StructuredAppend` *Since v2.1* | Position in a sequence, when the symbol is one of several; see [joining](encoding.md#data-too-long-for-one-symbol). |
| `GS1` *Since v2.1* | The symbol carries a GS1 element string. |

Use `Segments` when the exact bytes matter, for example for binary payloads
or text in an encoding other than UTF-8.

## What the decoder reads

`Decode` first tries a fast path for crisp, axis-aligned images such as the
ones this library renders. If that fails, it locates the finder patterns
with a locally adaptive threshold and fits a model of the module grid to
the symbol's own structure: the corners of the finders' nested squares,
the alignment patterns and the timing patterns. The model follows
perspective and moderate lens distortion, and when a finder is unreadable
the other two imply it. Modules are then read from the grayscale image,
each against the modules around it. If nothing is found at full
resolution, the symbol is located again in the image downscaled by 2, 4,
and so on, which removes texture finer than the modules, such as a
screen's pixel grid; the modules are still read at full resolution. Each
path is tried on the image as is and inverted, and every sampled symbol is
also read mirrored.

| Input | Supported |
| --- | --- |
| Rendered images at any scale; PNG, JPEG, GIF, paletted, transparent backgrounds | ✅ |
| Rotation, noise, blur, low contrast, uneven lighting | ✅ |
| Perspective (photos taken at an angle, up to about 50°) | ✅ *Since v2.4* |
| Photos of screens, very large modules, a finder covered by glare or damage | ✅ *Since v2.4* |
| Light-on-dark (inverted) and mirror images | ✅ |
| Styled codes: dots, rounded modules, round finders, gradients | ✅ |
| Version information of version 7+ symbols, with error correction | ✅ |
| Numeric, alphanumeric, byte and Kanji segments | ✅ |
| Structured append and GS1 (FNC1 in first position) | ✅ *Since v2.1* |
| Lens distortion | ✅ moderate; strong wide-angle distortion of large symbols ⚠️ |
| Curved surfaces (bottles, cans) | ⚠️ small symbols only |
| Several symbols in one image | ❌ only one is read |
| Micro QR, rMQR, FNC1 in second position, Hanzi mode | ❌ `ErrUnsupported` |

Modules should be at least 2 pixels wide. Large symbols (version 20 and up)
in photos are the hardest case: lens distortion bends their grid more than
the model follows. Move closer to the code rather than zooming out. See
[Performance](../performance.md#reading-photos) for how the decoder
compares with others on real photos.

### Text encodings

Byte segments are interpreted by the ECI in effect:

| ECI | Character set |
| --- | --- |
| 26 | UTF-8 |
| 1, 3 | ISO-8859-1 |
| 20 | Shift_JIS |
| 27, 170 | ASCII |
| none | UTF-8 if the bytes are valid UTF-8; otherwise Shift_JIS or Windows-1252, whichever reads as more plausible text |

Encoders often omit the ECI: Japanese ones write Shift_JIS, others
ISO-8859-1 or, from Windows software, Windows-1252, which puts “quotes”,
dashes and € where ISO-8859-1 has control codes. *Since v2.4.2*, the decoder
reads the bytes both ways and scores each reading by how characters follow
each other in text: accented letters sit inside words, katakana and kanji
run together, a half-width katakana rarely stands alone where the Latin-1
symbol with the same byte, such as ° in 25°C, often does. All undeclared
segments of a symbol are read in one character set, and a Kanji mode
segment in the symbol counts for Shift_JIS. One or two characters can read
plausibly both ways, such as "ﾒﾓ" and "ÒÓ"; use an ECI when that matters.

Other ECIs return `ErrUnsupported`; the raw bytes are still in `Segments` if
you need them.

## Options

`WithFastPathOnly()` skips the robust path. Use it to check images you have
just rendered yourself, where it is faster and an unexpected failure is a
bug worth surfacing.

## Errors

| Error | Meaning |
| --- | --- |
| `ErrNotFound` | No symbol was located, or the image is too small or has no contrast. |
| `ErrDecodeFailed` | A symbol was located but could not be read, for example because there was too much damage to correct. |
| `ErrUnsupported` | The symbol uses a feature listed above as unsupported. |

## Decoding untrusted images

The decoder is fuzz-tested against arbitrary images and malformed
bitstreams, and returns errors rather than panicking. Its cost grows with
the number of pixels, so downscale very large images (for example, to at
most 1,000 pixels on the long side) before decoding untrusted uploads.
