# go-qr
[![Mentioned in Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go#utilities)
[![Go Report Card](https://goreportcard.com/badge/github.com/piglig/go-qr)](https://goreportcard.com/report/github.com/piglig/go-qr)
[![Build Status](https://github.com/piglig/go-qr/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/piglig/go-qr/actions/workflows/go.yml?query=branch%3Amain)
[![Codecov](https://img.shields.io/codecov/c/github/piglig/go-qr)](https://app.codecov.io/github/piglig/go-qr)
[![GoDoc](https://godoc.org/github.com/piglig/go-qr?status.svg)](https://pkg.go.dev/github.com/piglig/go-qr)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](https://github.com/piglig/go-qr/blob/main/LICENSE)

> 🎶 Minimalist, zero-dependency QR code generator **and decoder** for Go.

## Contents
- [Overview](#overview)
- [Features](#features)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Performance](#performance)
- [Rendering](#rendering)
- [Advanced Encoding](#advanced-encoding)
- [Logo Embedding](#logo-embedding)
- [Decoding](#decoding)
- [Structured Payloads](#structured-payloads)
- [Batch API](#batch-api)
- [Concurrency](#concurrency)
- [Errors](#errors)
- [Command-Line Tool](#command-line-tool)
- [Verifying Output](#verifying-output)
- [License](#license)

## Overview
Native, zero-dependency QR code generation **and decoding** for Go — a fully
original implementation of QR Code Model 2. It covers the complete pipeline:
optimal segment-mode encoding, Reed–Solomon ECC over GF(2⁸), ISO/IEC 18004 mask
selection, PNG/SVG rendering, logo embedding, structured payloads, a concurrent
batch API, a native decoder, and a command-line tool.

## Features
- QR Code Model 2, all 40 versions, all 4 error-correction levels
- Optimal segment-mode switching for mixed numeric / alphanumeric / byte / kanji input
- PNG, SVG, and compact SVG (`fill-rule="evenodd"` single-path) output
- In-memory rendering: `ToPNGBytes`, `ToSVGBytes`, `ToImage`
- Native zero-dependency decoding: `Decode` / `DecodeDetailed` (fast axis-aligned path + rotation/noise-tolerant fallback)
- Logo embedding with ECC-budget validation
- Structured payloads: Wi-Fi, vCard/MECARD, email, SMS, tel, geo, URL
- Concurrent batch encoding and rendering
- Golden-file regression tests; decoder round-trip via `tools/verify`
- MIT licensed

## Installation
```shell
go get github.com/piglig/go-qr
```
Requires Go 1.18+. The library itself has **zero third-party runtime
dependencies** (only the standard library); the comparison and verification
helpers under [`tools/`](tools) live in a separate module and are never pulled
into your build. See the [CHANGELOG](CHANGELOG.md) for release notes.

## Quick Start
```go
import go_qr "github.com/piglig/go-qr"

qr, err := go_qr.EncodeText("Hello, world!", go_qr.Low)
if err != nil { /* ... */ }

config := go_qr.NewQrCodeImgConfig(10, 4)     // scale=10px, border=4 modules
_ = qr.PNG(config, "hello.png")
_ = qr.SVG(config, "hello.svg")
```

## Performance

### Encoding
go-qr's encoder is benchmarked against the two most popular Go QR generators.
The comparison measures the **core encode** only — text → in-memory symbol, no
image rendering — at `Medium` ECC. All three libraries perform the full ISO/IEC
18004 eight-mask penalty selection (skip2 defers it to `Bitmap()`, which the
benchmark forces).

| Payload (ECC = Medium) | go-qr | [skip2/go-qrcode][skip2] | [boombuler/barcode][boombuler] |
| --- | --- | --- | --- |
| numeric, 8 chars (v1) | **22.6 µs** · 18 allocs | 69.6 µs · 934 allocs — **3.1× slower** | 310 µs · 172 allocs — **13.7× slower** |
| alphanumeric, 14 chars (v1) | **23.4 µs** · 19 allocs | 69.5 µs · 936 allocs — **3.0×** | 318 µs · 181 allocs — **13.6×** |
| URL, 45 chars (v3) | **56 µs** · 28 allocs | 245 µs · 3,450 allocs — **4.4×** | 1,266 µs · 578 allocs — **22.6×** |
| byte, 672 chars (high ver) | **977 µs** · 83 allocs | 3,768 µs · 49,099 allocs — **3.9×** | 14,324 µs · 5,891 allocs — **14.7×** |

go-qr is **3–4×** faster than skip2/go-qrcode and **13–23×** faster than
boombuler/barcode, while allocating one to three orders of magnitude less memory
(18 vs 934 vs 172 allocations for a version-1 symbol). The low allocation count
also makes the concurrent [batch API](#batch-api) scale cleanly.

### Decoding
The native decoder is benchmarked against [gozxing][gozxing] (a ZXing port) on
crisp, freshly-rendered images:

| Symbol (clean) | go-qr native | [gozxing][gozxing] |
| --- | --- | --- |
| numeric (v1) | **129 µs** · 21 allocs | 800 µs · 53,914 allocs — **6.2× slower** |
| URL (v3) | **269 µs** · 26 allocs | 1,740 µs · 107,701 allocs — **6.5×** |
| byte (high ver) | **3.3 ms** · 121 allocs | 20.8 ms · 1.27M allocs — **6.3×** |

On accuracy, both decoders read 100% of the clean corpus; on a synthetic
degraded corpus (7° rotation + Gaussian noise) both land at 80% — i.e. go-qr
matches the ZXing-family decoder on robustness while being ~6× faster with
~3 orders of magnitude fewer allocations. The native decoder corrects rotation
and noise via an affine transform; it does **not** yet do perspective (camera
skew) correction, which is where the remaining degraded cases fail.

<sub>Measured on an AMD Ryzen 5 7600X, Go 1.25, `go test -bench`, median of runs. Numbers are illustrative and vary by machine; reproduce with the harness in [`tools/bench`](tools/bench) (`go test -run=^$ -bench=BenchmarkEncodeCompare -benchmem` and `-bench=BenchmarkDecodeClean`).</sub>

[skip2]: https://github.com/skip2/go-qrcode
[boombuler]: https://github.com/boombuler/barcode
[gozxing]: https://github.com/makiuchi-d/gozxing

## Rendering

### Files
```go
qr.PNG(config, "out.png")
qr.SVG(config, "out.svg")
```

### Writers
```go
qr.WriteAsPNG(config, w)
qr.WriteAsSVG(config, w)
```

### In-memory
```go
pngBytes, _ := qr.ToPNGBytes(config)
svgBytes, _ := qr.ToSVGBytes(config)
img, _      := qr.ToImage(config) // image.Image
```

### Config options
`NewQrCodeImgConfig(scale, border, opts...)` accepts:

| Option | Purpose |
| --- | --- |
| `WithLight(color.Color)` | Background color. Transparent light omits the SVG background rect. |
| `WithDark(color.Color)` | Foreground (module) color. |
| `WithSVGXMLHeader()` | Emit `<?xml ... ?>` + DOCTYPE prolog in SVG. |
| `WithOptimalSVG()` | Emit a single `<path>` with `fill-rule="evenodd"` (smaller, connected regions merged). |
| `WithLogo(img, sizeRatio)` | Embed a centered logo; validated against the ECC budget. |

Example:
```go
cfg := go_qr.NewQrCodeImgConfig(10, 4,
    go_qr.WithLight(color.White),
    go_qr.WithDark(color.RGBA{R: 0x20, G: 0x60, B: 0x20, A: 0xFF}),
    go_qr.WithOptimalSVG(),
)
```

## Advanced Encoding
`EncodeText` analyzes the whole string and encodes it in a single best-fit mode
(numeric, alphanumeric, or byte). For mixed-content strings you can recover
extra capacity with `MakeSegmentsOptimally`, which splits the input into the
optimal sequence of numeric / alphanumeric / byte segments, then encode with
`EncodeSegments`:

```go
segs, err := go_qr.MakeSegmentsOptimally(
    "PROJECT-1234567890 details", go_qr.Medium, go_qr.MinVersion, go_qr.MaxVersion)
if err != nil { /* ... */ }

// mask = -1 auto-selects the lowest-penalty mask; boostEcl = true upgrades the
// ECC level as far as the data still fits the chosen version.
qr, err := go_qr.EncodeSegments(
    segs, go_qr.Medium, go_qr.MinVersion, go_qr.MaxVersion, -1, true)
```

### Kanji
Kanji content can be encoded explicitly with `MakeKanji` (input runes must be
representable in the QR Kanji / Shift-JIS charset):

```go
seg, err := go_qr.MakeKanji("日本語")
if err != nil { /* ... */ }
qr, err := go_qr.EncodeStandardSegments([]*go_qr.QrSegment{seg}, go_qr.Medium)
```

> Kanji is currently **encode-only**: it is not selected by the automatic mode
> detection in `EncodeText` / `MakeSegmentsOptimally`, and the native decoder
> does not yet decode Kanji segments.

## Logo Embedding
```go
logo, _ := png.Decode(f)
cfg := go_qr.NewQrCodeImgConfig(10, 4, go_qr.WithLogo(logo, 0.2))
b, err := qr.ToPNGBytes(cfg)
```
`sizeRatio` is the logo side length as a fraction of the QR module area. The
library draws a 1-module-wide light padding around the logo and rejects ratios
that exceed the current ECC level's recovery budget — use `High` ECC for ratios
above ~0.22.

## Decoding
Native, zero-dependency QR decoding — the inverse of the encoder:

```go
img, _ := png.Decode(f)
text, err := go_qr.Decode(img)        // image.Image -> text
```

`Decode` first tries a fast path that assumes a crisp, axis-aligned image (the
shape this library's renderers produce), then falls back to a robust path that
locates the finder patterns and corrects for rotation/noise via an affine
transform. For structured output use `DecodeDetailed`:

```go
res, _ := go_qr.DecodeDetailed(img)
// res.Text, res.Version, res.Ecc, res.Mask, res.Segments
```

Pass `WithFastPathOnly()` to skip the robust fallback when inputs are known to
be freshly rendered (e.g. CI round-trip checks) for maximum speed.

## Structured Payloads
The `payload` sub-package builds canonical strings for common QR use cases:

```go
import "github.com/piglig/go-qr/payload"

text := payload.WiFi{SSID: "home", Password: "s3cret", Auth: payload.WPA}.String()
qr, _ := go_qr.EncodeText(text, go_qr.Medium)
```

Supported: `WiFi`, `VCard` (MECARD), `Email` (mailto), `SMS` (smsto), `Tel`,
`Geo`, `URL`.

## Batch API
```go
jobs := []go_qr.BatchJob{
    {Text: "one", Ecc: go_qr.Medium, Format: go_qr.FormatPNG},
    {Text: "two", Ecc: go_qr.Medium, Format: go_qr.FormatSVG,
     Config: go_qr.NewQrCodeImgConfig(10, 4, go_qr.WithOptimalSVG())},
}
results := go_qr.RenderBatch(jobs, 0) // 0 = runtime.NumCPU()
for _, r := range results {
    if r.Err != nil { /* ... */ }
    _ = r.Bytes
}
```
`EncodeBatch` returns `*QrCode` values; `RenderBatch` returns rendered bytes.
Results are in input order; per-item failures do not cancel the batch.

## Concurrency
The top-level `EncodeText`, `EncodeSegments`, `Decode`, and the rendering
methods are safe to call concurrently from multiple goroutines. The encoder
keeps a process-wide cache of version-invariant mask templates that is built
once per version and only ever read afterwards, so concurrent encodes share it
without locking. A `*QrCode` value is immutable once returned and can be
rendered from several goroutines at once. The [batch API](#batch-api) builds on
this directly.

## Errors
All errors returned from the library wrap one of these sentinels, usable with
`errors.Is`:

| Sentinel | Meaning |
| --- | --- |
| `ErrInvalidConfig` | `QrCodeImgConfig` has bad scale/border. |
| `ErrInvalidArgument` | Bad mask, nil segments, out-of-range value. |
| `ErrInvalidVersion` | Requested version range outside `[MinVersion, MaxVersion]`. |
| `ErrDataTooLong` | Input does not fit any version at the chosen ECC level. |
| `ErrUnencodableChar` | Character not representable in the requested mode. |
| `ErrInvalidImageOutput` | Output path extension or target is unsupported. |

```go
if _, err := go_qr.EncodeText(s, go_qr.High); errors.Is(err, go_qr.ErrDataTooLong) {
    // fall back to lower ECC, split segments, etc.
}
```

## Command-Line Tool
```shell
go install github.com/piglig/go-qr/tools/generator@latest
```

The CLI is organized as subcommands:
```
generator <command> [flags] [args]

Commands:
  encode     Encode text or a structured payload into QR image(s)
  decode     Decode a QR code image into text
  version    Print version and exit
  help       Show help

Run "generator <command> -h" for command-specific flags.
```

### `encode`
Content is given as a positional argument or via `-content`.
```
-content string         Content to encode (overridden by a positional argument)
-payload string         Structured payload: wifi, vcard, email, sms, tel, geo, url
-ecc string             Error correction: low, medium, quartile, high (default "high")
-scale int              Pixels per module for PNG / units per module for SVG (default 10)
-border int             Quiet-zone border, in modules (default 4)
-png string             Output PNG file path
-svg string             Output SVG file path
-svg-optimized string   Output optimized SVG file path
-stdout string          Write to stdout: png, svg, or svg-optimized
-logo string            Path to a logo image (png/jpeg/gif) to embed in the center
-logo-ratio float       Logo side length as fraction of QR module-area side (default 0.2)
-verify                 Decode the generated PNG and assert it matches the input
-preview                Print ANSI terminal preview to stderr
-quiet                  Suppress non-error output
```
`-stdout` cannot be combined with file outputs (`-png` / `-svg` /
`-svg-optimized`); the conflict is rejected with an error.

### `decode`
```
generator decode <image-file>   # png/jpeg/gif; prints decoded text to stdout
```

Examples:
```shell
generator encode hello                                       # ANSI preview
generator encode hello -png hello.png -svg hello.svg
generator encode hello -svg-optimized hello.svg              # compact single-path SVG
generator encode -payload wifi "ssid=home,password=s3cret,auth=WPA" -png wifi.png
generator encode hello -png hello.png -verify                # round-trip decode check
generator encode hello -stdout png > hello.png
generator decode hello.png                                   # decode an image, print text
generator version
```

## Verifying Output
The `tools/verify` sub-module wraps the native `go_qr.Decode` for round-trip
assertions in tests and CI.
```go
import "github.com/piglig/go-qr/tools/verify"

b, _ := qr.ToPNGBytes(cfg)
if err := verify.RoundTrip(b, "Hello, world!"); err != nil {
    // image is not readable by a standard decoder
}
```

## License
See the [LICENSE](LICENSE) file for license rights and limitations (MIT).
