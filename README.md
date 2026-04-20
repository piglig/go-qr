# go-qr
[![Mentioned in Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go#utilities)
[![Go Report Card](https://goreportcard.com/badge/github.com/piglig/go-qr)](https://goreportcard.com/report/github.com/piglig/go-qr)
[![Build Status](https://github.com/piglig/go-qr/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/piglig/go-qr/actions/workflows/go.yml?query=branch%3Amain)
[![Codecov](https://img.shields.io/codecov/c/github/piglig/go-qr)](https://app.codecov.io/github/piglig/go-qr)
[![GoDoc](https://godoc.org/github.com/piglig/go-qr?status.svg)](https://pkg.go.dev/github.com/piglig/go-qr)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](https://github.com/piglig/go-qr/blob/main/LICENSE)

> 🎶 Go Community Minimalist QR Code Generator Library.

## Overview
Native, zero-dependency QR code generation for Go. Mostly a translation of
[Nayuki's Java QR code generator](https://www.nayuki.io/page/qr-code-generator-library),
extended with PNG/SVG rendering, logo embedding, structured payloads, a
concurrent batch API, and a command-line tool.

## Features
- QR Code Model 2, all 40 versions, all 4 error-correction levels
- Optimal segment-mode switching for mixed numeric / alphanumeric / byte / kanji input
- PNG, SVG, and compact SVG (`fill-rule="evenodd"` single-path) output
- In-memory rendering: `ToPNGBytes`, `ToSVGBytes`, `ToImage`
- Logo embedding with ECC-budget validation
- Structured payloads: Wi-Fi, vCard/MECARD, email, SMS, tel, geo, URL
- Concurrent batch encoding and rendering
- Golden-file regression tests; decoder round-trip via `tools/verify`
- Typed sentinel errors (`errors.Is` friendly)
- MIT licensed

## Installation
```shell
go get github.com/piglig/go-qr
```

## Quick Start
```go
import go_qr "github.com/piglig/go-qr"

qr, err := go_qr.EncodeText("Hello, world!", go_qr.Low)
if err != nil { /* ... */ }

config := go_qr.NewQrCodeImgConfig(10, 4)     // scale=10px, border=4 modules
_ = qr.PNG(config, "hello.png")
_ = qr.SVG(config, "hello.svg")
```

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
`DataTooLongException` is retained for API compatibility and unwraps to
`ErrDataTooLong`.

## Command-Line Tool
```shell
go install github.com/piglig/go-qr/tools/generator@latest
```

Flags:
```
-content string         Content to encode (raw text, or key=val,... when -payload is set)
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

Examples:
```shell
generator -content hello                              # ANSI preview
generator -content hello -png hello.png -svg hello.svg
generator -content hello -svg-optimized hello.svg     # compact single-path SVG
generator -payload wifi -content "ssid=home,password=s3cret,auth=WPA" -png wifi.png
generator -content hello -png hello.png -verify       # round-trip decode check
generator -content hello -stdout png > hello.png
```

## Verifying Output
The `tools/verify` sub-module wraps gozxing to decode rendered output. It is a
separate Go module so the main library stays dependency-free.
```go
import "github.com/piglig/go-qr/tools/verify"

b, _ := qr.ToPNGBytes(cfg)
if err := verify.RoundTrip(b, "Hello, world!"); err != nil {
    // image is not readable by a standard decoder
}
```

## License
See the [LICENSE](LICENSE) file for license rights and limitations (MIT).
