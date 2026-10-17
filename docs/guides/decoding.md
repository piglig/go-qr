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
| `Corners` *Since v2.6* | The outer corners of the symbol in the image, quiet zone excluded, in the symbol's own orientation: top-left, top-right, bottom-right, bottom-left. They follow rotation, perspective and mirroring: `Corners[0]`, `[1]` and `[3]` are always the corners at the three finder patterns. |
| `Inverted` *Since v2.6* | The symbol has light modules on a dark background. |
| `Segments` | Each segment: `Mode`, `NumChars`, the `ECI` in effect (-1 for none), and the raw `Data` (digits, characters, bytes, or Shift_JIS for Kanji). |
| `StructuredAppend` *Since v2.1* | Position in a sequence, when the symbol is one of several; see [joining](encoding.md#data-too-long-for-one-symbol). |
| `GS1` *Since v2.1* | The symbol carries a GS1 element string. |
| `ErrorsCorrected` *Since v2.7* | The number of codewords error correction repaired. |
| `UnusedErrorCorrection` *Since v2.7* | The share of error correction left unused, from 1 (no errors) to 0, by the block that needed most; see below. |
| `SoftDecision` *Since v2.7* | Hard decisions did not read the symbol; soft decisions did (see [below](#what-the-decoder-reads)). |

Use `Segments` when the exact bytes matter, for example for binary payloads
or text in an encoding other than UTF-8.

### How much error correction a symbol used

`UnusedErrorCorrection` is the unused error correction of ISO/IEC 15415,
computed for each block as 1 − (e + 2t) / (d − p), with e codewords erased,
t errors corrected, d check codewords and p of them misdecode protection,
and reported for the block with the least left. A symbol read cleanly has
1; one that error correction only just recovered has 0. ISO/IEC 15415
grades a printed symbol A from 0.62 and fails it below 0.25, so a low value
from a good photo is a sign of a damaged or poorly printed code, before it
stops scanning:

```go
res, err := qr.Decode(img)
if err == nil && res.UnusedErrorCorrection < 0.25 {
	log.Printf("label %q is barely readable", res.Text)
}
```

The value depends on the photo as much as on the print: blur, glare and
low resolution use error correction too.

## What the decoder reads

`Decode` first tries a fast path for crisp, axis-aligned images such as the
ones this library renders. If that fails, it locates the finder patterns
with a locally adaptive threshold and fits a model of the module grid to
the symbol's own structure: the corners of the finders' nested squares,
the alignment patterns and the timing patterns. The model follows
perspective and moderate lens distortion, and, through the measured
positions of the alignment patterns, a surface that bends locally
(*Since v2.7*); when a finder is unreadable
the other two imply it. Modules are then read from the grayscale image,
each against the modules around it. Symbols are located in the image and
in its halvings, which remove texture finer than the modules, such as a
screen's pixel grid; the modules are always read at full resolution. In
large images the search starts at the halving nearest half a megapixel,
where the codes of camera photos are usually found, and goes on to
coarser and then finer ones; a code that is found there but too small to
locate precisely is located again at full resolution, in a window around
it (*Since v2.7*). Light symbols on a dark background are searched for
last, and every sampled symbol is also read mirrored.

| Input | Supported |
| --- | --- |
| Rendered images at any scale; PNG, JPEG, GIF, paletted, transparent backgrounds | ✅ |
| Rotation, noise, blur, low contrast, uneven lighting | ✅ |
| Perspective (photos taken at an angle, up to about 50°) | ✅ *Since v2.4* |
| Photos of screens, very large modules, a finder covered by glare or damage | ✅ *Since v2.4* |
| Glare, blur or damage over the data modules | ✅ *Since v2.7*, by soft decisions; see below |
| Light-on-dark (inverted) and mirror images | ✅ |
| Styled codes: dots, rounded modules, round finders, gradients | ✅ |
| Version information of version 7+ symbols, with error correction | ✅ |
| Numeric, alphanumeric, byte and Kanji segments | ✅ |
| Structured append and GS1 (FNC1 in first position) | ✅ *Since v2.1* |
| Lens distortion | ✅ moderate; strong wide-angle distortion of large symbols ⚠️ |
| Curved surfaces (bottles, cans) | ⚠️ small symbols only |
| Several symbols in one image | ✅ with `DecodeAll` *Since v2.6* |
| Micro QR, rMQR, FNC1 in second position, Hanzi mode | ❌ `ErrUnsupported` |

When a located symbol does not decode, the decoder reads its modules
again with soft decisions (*Since v2.7*): the function patterns, whose
values are known, calibrate a model of the light, contrast and blur
around each module, and the codewords it is least sure of are erased,
which Reed–Solomon codes correct twice as many of as errors. On the
BoofCV photos this reads 12 more, mostly with glare or blur, and none
fewer. These attempts keep part of the error correction unused against
misreads. `Verify` does not count them: a design must scan with readers
that decode by hard decisions only, as most do.

Modules should be at least 2 pixels wide. Large symbols (version 20 and up)
in photos are the hardest case: a lens or a bent sheet distorts their grid
far from the finders, where small errors in the model add up. Move closer to the code rather than zooming out. See
[Performance](../explanation/performance.md#reading-photos) for how the decoder
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
dashes and € where ISO-8859-1 has control codes. *Since v2.5*, the decoder
reads the bytes both ways and scores each reading by how characters follow
each other in text: accented letters sit inside words, katakana and kanji
run together, a half-width katakana rarely stands alone where the Latin-1
symbol with the same byte, such as ° in 25°C, often does. All undeclared
segments of a symbol are read in one character set, and a Kanji mode
segment in the symbol counts for Shift_JIS. One or two characters can read
plausibly both ways, such as "ﾒﾓ" and "ÒÓ"; use an ECI when that matters.

Other ECIs make `Decode` fail with `ErrUnsupported`, without a result.

## Several symbols in one image

*Since v2.6.* `DecodeAll` reads every symbol in the image, such as the labels
in a photo of a shelf, and returns them in reading order: rows from top to
bottom, each from left to right.

```go
results, err := qr.DecodeAll(img)
if err != nil {
	return err // nothing decoded: the error Decode would return
}
for _, res := range results {
	fmt.Println(res.Text, res.Corners)
}
```

Each symbol is returned once, and identical symbols at different places
are all returned. The error is nil when at least one symbol decodes.
`Corners` tell the symbols apart, and the results of a structured append
sequence photographed together go straight to `JoinStructuredAppend`:

```go
results, err := qr.DecodeAll(img)
if err != nil {
	return err
}
text, err := qr.JoinStructuredAppend(results...)
```

`DecodeAll` keeps searching after the first symbol, through every scale
and polarity, so it takes several times as long as `Decode` on an image of
one symbol: 13.7 ms against 3.5 ms, the medians over the BoofCV photos.
`qr.WithMaxSymbols(n)` stops it after `n` symbols.

## Grading print quality

*Since v2.7.* `Grade` decodes a symbol by hard decisions, as `Verify`
does, and grades it by the print quality parameters of ISO/IEC 15415, A
to F, the lowest of which is the symbol's grade:

```go
q, err := qr.Grade(img)
if err != nil {
	return err
}
fmt.Println(q.Grade, q.SymbolContrast, q.Modulation, q.UnusedErrorCorrection)
```

| Parameter | What it measures | A | B | C | D |
| --- | --- | --- | --- | --- | --- |
| `SymbolContrast` | Highest less lowest reflectance in the symbol and quiet zone | ≥ 0.70 | ≥ 0.55 | ≥ 0.40 | ≥ 0.20 |
| `Modulation` | How evenly dark and light the modules are, weighed against the error correction left | | | | |
| `ReflectanceMargin` | Modulation for modules on the right side of the threshold | | | | |
| `FixedPatternDamage` | Modules of the finder patterns and separators read wrong or weakly, the worst finder counting | 0 | 1 | 2 | 3 |
| `AxialNonuniformity` | Difference between the module pitch along the two axes, relative | ≤ 0.06 | ≤ 0.08 | ≤ 0.10 | ≤ 0.12 |
| `GridNonuniformity` | Largest offset, in modules, of an alignment pattern from a regular grid | ≤ 0.38 | ≤ 0.50 | ≤ 0.63 | ≤ 0.75 |
| `UnusedErrorCorrection` | As [in the result](#how-much-error-correction-a-symbol-used) | ≥ 0.62 | ≥ 0.50 | ≥ 0.37 | ≥ 0.25 |

Modulation and reflectance margin grade each module (A from 0.50 of half
the symbol contrast away from the threshold, B 0.40, C 0.30, D 0.20) and
each codeword by its worst module; at each grade level, the codewords
below it count as erased, and the level is worth no more than the grade
of the error correction they would leave. A few weak codewords therefore
cost nothing while error correction absorbs them.

The grades are estimates, for comparing prints and watching them wear;
they do not replace a calibrated verifier. Reflectance is estimated from
the image's luminance, linearized as sRGB, relative to white; each module
is measured over a disc of 0.6 modules at the center the decoder located;
and a single image counts, where a verifier calibrates its lighting and
averages several. Take the photo squarely, evenly lit, with modules at
least 5 pixels wide: perspective lowers the two nonuniformity grades, and
glare, blur and shadows lower the others, as they do a print's
readability.

Styles show in the grades: rounded and circular finders differ from the
standard's square ones at their corners, which `FixedPatternDamage`
grades F, and a logo uses up error correction:

| Rendering of a version 3-H code | Grade |
| --- | --- |
| Square, dot or rounded modules; gradients of dark colors | A |
| `WithLogo(logo, 0.2)` | C: unused error correction 0.45 |
| `WithFinderShape(qr.FinderRounded)` or `FinderCircle` | F: fixed pattern damage |

## Options

`WithFastPathOnly()` skips the robust path. Use it to check images you have
just rendered yourself, where it is faster and an unexpected failure is a
bug worth surfacing. `WithMaxSymbols(n)` *Since v2.6* limits `DecodeAll` to
`n` symbols; `Decode` ignores it. `WithoutInverted()` *Since v2.7* skips the
search for light symbols on a dark background; see
[scanning video](#scanning-video).

## Scanning video

Most frames of a camera feed in which a code is being looked for hold no
code, and a frame without a code is the slowest to decode: every scale of
it is searched. Three things keep up with the frames:

- `WithoutInverted()` skips the last search, for light symbols on a dark
  background, which printed codes rarely are. On frames of texture without
  a code, from VGA to 12 megapixels, it saves 28% of the time.
- A frame needs no more pixels than its code: downscale frames to a
  megapixel or two before decoding when the code fills a good part of
  the view.
- Once a code is found, look for it in the next frames around where it
  was, through `SubImage`; `Decode` reports `Corners` in the coordinates
  of the full frame:

```go
type subImager interface {
	SubImage(r image.Rectangle) image.Image
}

var last image.Rectangle // where the code was in the previous frame
for frame := range frames {
	var res *qr.DecodeResult
	var err error
	if !last.Empty() {
		res, err = qr.Decode(frame.(subImager).SubImage(last), qr.WithoutInverted())
	}
	if last.Empty() || err != nil {
		res, err = qr.Decode(frame, qr.WithoutInverted())
	}
	if err != nil {
		last = image.Rectangle{}
		continue
	}
	last = around(res.Corners, frame.Bounds()) // the corners' bounding box, enlarged
	handle(res)
}
```

## Errors

| Error | Meaning |
| --- | --- |
| `ErrNotFound` | No symbol was located, or the image is too small or has no contrast. |
| `ErrDecodeFailed` | A symbol was located but could not be read, for example because there was too much damage to correct. |
| `ErrUnsupported` | The symbol uses a feature listed above as unsupported. |
| `context.Canceled`, `context.DeadlineExceeded` | `DecodeContext` or `DecodeAllContext` stopped because the context was done. |

## Decoding untrusted images

The decoder is fuzz-tested against arbitrary images and malformed
bitstreams, and returns errors rather than panicking. Its cost grows with
the number of pixels on any image, at most about 120 ms for a
12-megapixel one without a code, including images crafted with many
finder patterns (*since v2.5.1*), and
decoding the image file itself costs more. Check the dimensions of untrusted uploads
with `image.DecodeConfig` and reject oversized ones before decoding them,
and bound the time each one may take with a context, as below.

## Bounding the time

*Since v2.7.* `DecodeContext` and `DecodeAllContext` stop searching when a
context is done, for example at a request's deadline, or when a newer frame
of video makes the current one moot:

```go
ctx, cancel := context.WithTimeout(r.Context(), 200*time.Millisecond)
defer cancel()
res, err := qr.DecodeContext(ctx, img)
if errors.Is(err, context.DeadlineExceeded) {
	// The image took too long; it holds no code that is easy to find.
}
```

`DecodeAllContext` returns the symbols it read before the context was
done, along with the context's error. The search checks the context
between its stages, such as the scales of the image, so it can run on for
one stage after the context is done: at full resolution, up to about half
of the time an image without a code takes.
