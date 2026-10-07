# go-qr
[![Mentioned in Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go#utilities)
[![Go Report Card](https://goreportcard.com/badge/github.com/piglig/go-qr/v2)](https://goreportcard.com/report/github.com/piglig/go-qr/v2)
[![Build Status](https://github.com/piglig/go-qr/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/piglig/go-qr/actions/workflows/go.yml?query=branch%3Amain)
[![Codecov](https://img.shields.io/codecov/c/github/piglig/go-qr)](https://app.codecov.io/github/piglig/go-qr)
[![Go Reference](https://pkg.go.dev/badge/github.com/piglig/go-qr/v2.svg)](https://pkg.go.dev/github.com/piglig/go-qr/v2)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](https://github.com/piglig/go-qr/blob/main/LICENSE)

> 🎶 Minimalist, zero-dependency QR code generator **and decoder** for Go.

## Contents
- [Features](#features)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Encoding](#encoding)
- [Rendering](#rendering)
- [Styling](#styling)
- [Decoding](#decoding)
- [Structured Payloads](#structured-payloads)
- [Batch Processing](#batch-processing)
- [Errors](#errors)
- [Performance](#performance)
- [Command-Line Tool](#command-line-tool)
- [License](#license)

## Features
- QR Code Model 2: all 40 versions and all four error correction levels
- Optimal mixed-mode segmentation (numeric, alphanumeric, byte, Kanji) by default, with optional UTF-8 ECI
- PNG, compact single-path SVG, `image.RGBA` and Unicode text output, with custom colors and logos
- Styled codes: dot and rounded modules, rounded and circular finders, finder colors and gradients, each checkable with `Verify`
- A native decoder that reads rotated, inverted, mirrored, low-contrast and unevenly lit images, plus Kanji and ECI character sets
- Structured payloads: Wi-Fi, MECARD, email, SMS, tel, geo, URL
- Concurrent, cancelable batch encoding and rendering
- No dependencies outside the standard library

## Installation
```shell
go get github.com/piglig/go-qr/v2
```
Requires Go 1.23 or later. See the [CHANGELOG](CHANGELOG.md) for release
notes, including the v1 → v2 API changes.

## Quick Start
```go
import "github.com/piglig/go-qr/v2"

code, err := qr.Encode("Hello, world!")
if err != nil {
    log.Fatal(err)
}

png, err := code.PNG()            // []byte; or code.WritePNG(w)
svg, err := code.SVG()            // []byte; or code.WriteSVG(w)
fmt.Print(code)                   // Unicode block preview
```

## Encoding
`Encode` picks the smallest version that holds the text, switches between
numeric, alphanumeric, byte and Kanji modes wherever that saves bits, raises
the error correction level as far as the data still fits, and chooses the mask
with the lowest ISO/IEC 18004 penalty. Every choice can be overridden:

```go
code, err := qr.Encode(text,
    qr.WithECC(qr.ECCHigh),       // minimum level; default ECCMedium
    qr.WithVersionRange(5, 10),   // default 1..40
    qr.WithMask(3),               // default: lowest penalty
    qr.WithoutECCBoost(),         // keep exactly ECCHigh
)
fmt.Println(code.Version(), code.ECC(), code.Mask(), code.Size())
```

| Option | Effect |
| --- | --- |
| `WithECC(level)` | Minimum error correction level: `ECCLow`, `ECCMedium` (default), `ECCQuartile`, `ECCHigh`. |
| `WithVersionRange(min, max)` | Restrict the symbol version; pass the same value twice to fix it. |
| `WithMask(m)` | Force mask pattern 0–7. |
| `WithoutECCBoost()` | Do not raise the ECC level into spare capacity. |
| `WithSimpleSegmentation()` | Encode the whole text in one mode instead of switching modes. |
| `WithUTF8ECI()` | Declare UTF-8 with an ECI designator when the text is not ASCII. Strict ISO/IEC 18004 readers assume ISO-8859-1 otherwise. |

`EncodeBytes(data, opts...)` encodes binary data in a single byte segment.

### Long data and GS1
`EncodeStructured` spreads text that is too long for one symbol over up to 16
symbols linked by structured append headers. Readers report each symbol's
position, and `JoinStructuredAppend` reassembles the message:

```go
codes, err := qr.EncodeStructured(longText, qr.WithVersionRange(1, 10))
// ... later, after decoding every symbol in any order:
text, err := qr.JoinStructuredAppend(results...)
```

`WithGS1()` marks the data as a GS1 element string for logistics and retail
systems. Separate variable-length elements with the ASCII GS character
(`\x1d`); `DecodeResult.GS1` reports such symbols:

```go
code, err := qr.Encode("0109501101530003"+"17250101"+"10ABC123\x1d"+"21XYZ", qr.WithGS1())
```

### Explicit segments
To control the encoding exactly, build segments and pass them to
`EncodeSegments`:

```go
order, _ := qr.AlphanumericSegment("ORDER-")
number, _ := qr.NumericSegment("0123456789")
name, _ := qr.KanjiSegment("山田")
code, err := qr.EncodeSegments([]qr.Segment{order, number, name})
```

`BytesSegment` and `ECISegment` complete the set.

## Rendering
Rendering methods take `RenderOption` values:

```go
png, err := code.PNG(qr.WithScale(8), qr.WithQuietZone(2))
err = code.WriteSVG(w,
    qr.WithForeground(color.RGBA{R: 0x1a, G: 0x3c, B: 0x6e, A: 0xff}),
    qr.WithBackground(color.Transparent),
)
img, err := code.Image(qr.WithLogo(logo, 0.2)) // *image.RGBA for composition
err = code.WriteText(os.Stdout)                // █▀▄ half blocks
```

| Method | Output |
| --- | --- |
| `Image(opts...)` | `*image.RGBA`, for further drawing |
| `PNG(opts...)`, `WritePNG(w, opts...)` | PNG; a 1-bit paletted image unless a logo is drawn |
| `SVG(opts...)`, `WriteSVG(w, opts...)` | SVG with a single `fill-rule="evenodd"` path tracing the dark regions |
| `String()`, `WriteText(w, opts...)` | Unicode text, two module rows per line |

| Option | Effect |
| --- | --- |
| `WithScale(n)` | Pixels (PNG) or user units (SVG) per module; default 10. |
| `WithQuietZone(n)` | Margin in modules; default 4, as the standard requires. |
| `WithForeground(c)` / `WithBackground(c)` | Module and background colors; default black on white. A transparent background omits the SVG background. |
| `WithLogo(img, ratio)` | Draw `img` over the center at `ratio` of the symbol side, on a one-module pad. |
| `WithSVGXMLHeader()` | Add the XML declaration and DOCTYPE to SVG output. |

### Logos
A logo hides the modules under it, so the symbol relies on error correction
to stay readable. Rendering fails with `ErrLogoTooLarge` when the codewords
under the logo would use more than 75% of the correction capacity of any
error correction block, which leaves the rest for print defects and glare.
Encode with `WithECC(qr.ECCHigh)` for the largest logos; a ratio of 0.2 fits
every version at that level:

```go
code, _ := qr.Encode("https://example.com", qr.WithECC(qr.ECCHigh))
png, err := code.PNG(qr.WithLogo(logo, 0.2))
```

### Checking readability
`Verify` renders the code with the given options, decodes the image and
checks that it carries exactly the code's data. It also rejects colors that
phone scanners commonly fail on: a foreground lighter than the background, or
less than 40% luminance contrast. Call it before publishing a code with
custom colors or a logo:

```go
opts := []qr.RenderOption{qr.WithForeground(brandBlue), qr.WithLogo(logo, 0.2)}
if err := code.Verify(opts...); err != nil {
    // errors.Is(err, qr.ErrUnreadable) or qr.ErrLogoTooLarge
}
png, err := code.PNG(opts...)
```

## Styling
![Style samples](docs/images/styles.png)

```go
png, err := code.PNG(
    qr.WithModuleShape(qr.ModuleRounded),
    qr.WithFinderShape(qr.FinderRounded),
    qr.WithGradient(navy, teal, 45),
)
```

| Option | Effect |
| --- | --- |
| `WithModuleShape(s)` | `ModuleSquare` (default), `ModuleDot`, or `ModuleRounded`, which rounds corners whose neighbors are light so runs merge into smooth shapes. |
| `WithFinderShape(s)` | `FinderSquare` (default), `FinderRounded`, or `FinderCircle`. |
| `WithFinderColor(ring, center)` | Colors of the three finder patterns. |
| `WithGradient(from, to, angle)` | Linear gradient over the dark modules; angle in degrees, 0 is left to right. Finders follow it unless `WithFinderColor` is set. |

Every style keeps the module centers and the 1:1:3:1:1 finder proportions
that readers rely on, and edges are anti-aliased. Styles still cost some
robustness, so check the exact options with `Verify` before publishing; it
also checks that every dark color, including both ends of a gradient,
contrasts with the background. Readers' "pure barcode" modes assume square
finders and do not read rounded or circular ones, but normal camera scanning
does.

Styled PNGs in two colors are 8-bit paletted images; gradients and finder
colors need full RGBA PNGs, which take several times longer to encode. SVG
output stays fast in every style.

## Decoding
```go
img, _, _ := image.Decode(f)
res, err := qr.Decode(img)
fmt.Println(res.Text, res.Version, res.ECC, res.Mirrored)
for _, s := range res.Segments {
    fmt.Println(s.Mode, s.ECI, s.Data)
}
```

`Decode` first tries a fast path for crisp, axis-aligned images such as the
ones this package renders. It then falls back to a robust path that finds the
three finder patterns with a locally adaptive threshold and samples the grid
through an affine transform. Each path is tried on the image as is and
inverted, and each sampled symbol is also read mirrored.
`WithFastPathOnly()` skips the robust path when the input is known to be
freshly rendered.

| Input | Supported |
| --- | --- |
| Crisp renders, any scale; PNG, JPEG (YCbCr), paletted, transparent backgrounds | ✅ |
| Rotation, noise, low contrast, uneven lighting | ✅ |
| Light-on-dark (inverted) and mirror images | ✅ |
| Numeric, alphanumeric, byte and Kanji segments | ✅ |
| ECI 26 (UTF-8), 1/3 (ISO-8859-1), 20 (Shift_JIS), 27/170 (ASCII) | ✅ |
| Byte data without ECI | UTF-8 if valid, otherwise ISO-8859-1 |
| Perspective distortion, multiple symbols per image | ❌ |
| Structured append and GS1 (FNC1 in first position) | ✅ |
| Micro QR, rMQR, FNC1 in second position, Hanzi, other ECIs | ❌ `ErrUnsupported` |

## Structured Payloads
The `payload` package builds the strings that phone scanners act on, and
parses decoded text back into them:

```go
import "github.com/piglig/go-qr/v2/payload"

wifi := payload.WiFi{SSID: "home", Password: "s3cret", Auth: payload.WPA}
code, err := qr.Encode(wifi.String())

p, err := payload.Parse(res.Text) // after qr.Decode
if w, ok := p.(payload.WiFi); ok {
    fmt.Println("join", w.SSID)
}
```

| Type | Format | Scanner action |
| --- | --- | --- |
| `WiFi` | `WIFI:` | Join a network |
| `VCard` | `MECARD:` | Save a contact (compact) |
| `Contact` | vCard 3.0 | Save a contact with several phones and emails |
| `Event` | iCalendar `VEVENT` | Add a calendar event |
| `OTP` | `otpauth://` | Enroll a 2FA authenticator (TOTP/HOTP) |
| `EPC` | EPC069-12 (GiroCode) | Prefill a SEPA transfer; check it with `Validate` |
| `Email`, `SMS`, `Tel`, `Geo`, `URL` | `mailto:`, `sms:`, `tel:`, `geo:`, `http(s)://` | Compose, call, open a map or a page |

## Batch Processing
`Batch` encodes and renders jobs on a worker pool and returns results in job
order. A failing job does not stop the others, and canceling the context skips
the jobs that have not started:

```go
results := qr.Batch(ctx, []qr.BatchJob{
    {Text: "one", Format: qr.FormatPNG},
    {Text: "two", Format: qr.FormatSVG,
        Encode: []qr.EncodeOption{qr.WithECC(qr.ECCHigh)},
        Render: []qr.RenderOption{qr.WithScale(4)}},
    {Text: "encode only"}, // FormatNone
}, 0) // 0 workers = GOMAXPROCS
for _, r := range results {
    if r.Err != nil { /* ... */ }
    _ = r.Code // *qr.Code
    _ = r.Data // rendered bytes
}
```

All functions and methods are safe for concurrent use, and a `*qr.Code` is
immutable once returned.

## Errors
Every error wraps one of these sentinels, so callers can test categories with
`errors.Is`:

| Sentinel | Meaning |
| --- | --- |
| `ErrInvalidArgument` | Invalid option or argument: mask outside 0–7, zero scale, nil color, logo ratio outside (0, 1)… |
| `ErrInvalidVersion` | Version range outside 1–40 or with min > max. |
| `ErrDataTooLong` | The data does not fit the largest allowed version at the requested level. |
| `ErrUnencodableChar` | A character the requested segment mode cannot represent. |
| `ErrLogoTooLarge` | The logo would use too much error correction capacity. |
| `ErrUnreadable` | `Verify` could not read the rendering back, or its colors are risky for scanners. |
| `ErrNotFound` | No QR Code was located in the image. |
| `ErrDecodeFailed` | A symbol was located but could not be read. |
| `ErrUnsupported` | The symbol uses a feature the decoder does not implement. |

```go
if _, err := qr.Encode(s, qr.WithECC(qr.ECCHigh)); errors.Is(err, qr.ErrDataTooLong) {
    // retry with a lower level, or split the data
}
```

## Performance
Encoding (text → symbol, no rendering, ECC Medium) against the two most
popular Go generators. All three run the full eight-mask penalty selection;
skip2 defers it to `Bitmap()`, which the benchmark forces.

| Payload | go-qr | [skip2/go-qrcode][skip2] | [boombuler/barcode][boombuler] |
| --- | --- | --- | --- |
| numeric, 8 chars (v1) | **25 µs** · 17 allocs | 74 µs · 934 allocs (2.9×) | 407 µs · 172 allocs (16×) |
| alphanumeric, 14 chars (v1) | **26 µs** · 19 allocs | 76 µs · 936 allocs (3.0×) | 414 µs · 181 allocs (16×) |
| URL, 45 chars (v4) | **101 µs** · 23 allocs | 278 µs · 3,450 allocs (2.8×) | 1,425 µs · 578 allocs (14×) |
| text, 672 chars (v20) | **1.07 ms** · 129 allocs | 3.35 ms · 49,099 allocs (3.1×) | 16.3 ms · 5,891 allocs (15×) |

Decoding crisp rendered images against [gozxing][gozxing], a ZXing port:

| Symbol | go-qr | [gozxing][gozxing] |
| --- | --- | --- |
| numeric (v1) | **109 µs** · 17 allocs | 686 µs · 53,914 allocs (6.3×) |
| URL (v4) | **206 µs** · 23 allocs | 1,399 µs · 107,701 allocs (6.8×) |
| text (v29) | **2.3 ms** · 114 allocs | 16.9 ms · 1.27M allocs (7.4×) |

Both decoders read the whole clean corpus and 4 of 5 images in a degraded
corpus (7° rotation plus Gaussian noise). In the failing image, the rotation
pushes the finder patterns out of the frame.

<sub>Intel Core i7-14700KF, Go 1.25, Windows, best of five `go test -bench` runs on an otherwise idle machine. Absolute numbers vary by machine; reproduce with [`tools/bench`](tools/bench): `go test -run=^$ -bench='EncodeCompare|DecodeClean' -benchmem ./bench/`.</sub>

[skip2]: https://github.com/skip2/go-qrcode
[boombuler]: https://github.com/boombuler/barcode
[gozxing]: https://github.com/makiuchi-d/gozxing

## Command-Line Tool
```shell
go install github.com/piglig/go-qr/tools/generator@latest
```

The `tools` module pins the released library. To build it against a local
checkout, for example while changing the library, create a workspace at the
repository root with `go work init . ./tools`.

```
generator <command> [flags] [args]

Commands:
  encode     Encode text or a structured payload into QR image(s)
  decode     Decode a QR code image into text
  version    Print version and exit
  help       Show help
```

### `encode`
Content is given as a positional argument or with `-content`. Flags must come
before the content: everything after the first positional argument is encoded.

```
-content string      Content to encode (a positional argument takes precedence)
-payload string      Structured payload: wifi, vcard, email, sms, tel, geo, url
-ecc string          Error correction: low, medium, quartile, high (default "high")
-simple              Encode the whole text in one mode instead of switching modes
-scale int           Pixels (PNG) or units (SVG) per module (default 10)
-quiet-zone int      Margin around the symbol, in modules (default 4)
-png string          Output PNG file
-svg string          Output SVG file
-stdout string       Write to stdout instead: png, svg, or text
-logo string         Logo image (png/jpeg/gif) to draw in the center
-logo-ratio float    Logo side as a fraction of the symbol side (default 0.2)
-verify              Decode the generated PNG and check it matches the input
-preview             Print an ANSI preview to stderr
-quiet               Suppress non-error output
```
`-stdout` cannot be combined with `-png` or `-svg`. Without any output flag
the command prints a preview.

### `decode`
```
generator decode <image-file>   # png/jpeg/gif; prints the text to stdout
```

### Examples
```shell
generator encode hello                                          # ANSI preview
generator encode -png hello.png -svg hello.svg hello
generator encode -stdout text hello                             # Unicode text
generator encode -payload wifi -png wifi.png "ssid=home,password=s3cret,auth=WPA"
generator encode -logo logo.png -png branded.png -verify "https://example.com"
generator encode -stdout png hello > hello.png
generator decode hello.png
```

## License
MIT; see [LICENSE](LICENSE).
