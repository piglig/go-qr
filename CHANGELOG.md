# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [2.1.0] - 2026-10-07

### Added

- New payload types: `OTP` (otpauth:// for authenticator apps), `Contact`
  (vCard 3.0), `Event` (iCalendar) and `EPC` (SEPA GiroCode, with
  `Validate`).
- `payload.Parse` turns decoded text back into a payload value, and every
  type implements the new `payload.Payload` interface.
- `Code.Verify` renders a code, decodes it and checks the data, and rejects
  inverted or low-contrast colors, reporting `ErrUnreadable`.
- `EncodeStructured` splits long text over up to 16 symbols with structured
  append headers; the decoder reports them in
  `DecodeResult.StructuredAppend`, and `JoinStructuredAppend` reassembles
  the message.
- `WithGS1` encodes GS1 element strings with FNC1 in first position, and the
  decoder reads them, reporting `DecodeResult.GS1`. `ModeStructuredAppend`
  and `ModeFNC1` describe the new segments.

### Fixed

- The logo size check estimated damage from the covered area and accepted
  logos that made the symbol unreadable, such as a 0.2 logo on a version 2
  symbol at ECC M. It now counts the codewords under the logo in each error
  correction block and rejects logos that would use more than 75% of any
  block's correction capacity.
- `go install github.com/piglig/go-qr/tools/generator@latest` failed because
  the `tools` module used a `replace` directive. It now requires the released
  library, and CI tests it against the checkout through a Go workspace.

## [2.0.0] - 2026-10-07

Version 2 redesigns the API around functional options. The module path is now
`github.com/piglig/go-qr/v2` and the package is named `qr`.

### Breaking changes

| v1 | v2 |
| --- | --- |
| `import go_qr "github.com/piglig/go-qr"` | `import "github.com/piglig/go-qr/v2"` (package `qr`) |
| `EncodeText(text, ecl)` | `Encode(text, opts...)` |
| `EncodeBinary(data, ecl)` | `EncodeBytes(data, opts...)` |
| `EncodeStandardSegments(segs, ecl)`, `EncodeSegments(segs, ecl, min, max, mask, boost)` | `EncodeSegments(segs, opts...)` with `WithECC`, `WithVersionRange`, `WithMask`, `WithoutECCBoost` |
| `MakeSegmentsOptimally(text, ecl, min, max)` | default behavior of `Encode`; `WithSimpleSegmentation()` opts out |
| `MakeSegments`, `MakeNumeric`, `MakeAlphanumeric`, `MakeBytes`, `MakeKanji`, `MakeEci` | `NumericSegment`, `AlphanumericSegment`, `BytesSegment`, `KanjiSegment`, `ECISegment` (value type `Segment`) |
| `QrCode`, `QrSegment` | `Code`, `Segment` |
| `Ecc`, `Low`, `Medium`, `Quartile`, `High` | `ECC`, `ECCLow`, `ECCMedium`, `ECCQuartile`, `ECCHigh` |
| `Mode` struct, `Numeric`, `Alphanumeric`, `Byte`, `Kanji`, `Eci` | `Mode` enum, `ModeNumeric`, `ModeAlphanumeric`, `ModeByte`, `ModeKanji`, `ModeECI` |
| `NewQrCodeImgConfig(scale, border, opts...)` | `WithScale`, `WithQuietZone` passed to each render call |
| `WithLight`, `WithDark` | `WithBackground`, `WithForeground` |
| `ToImage`, `ToPNGBytes`, `WriteAsPNG`, `PNG(cfg, path)` | `Image`, `PNG`, `WritePNG` |
| `ToSVGBytes`, `WriteAsSVG`, `SVG(cfg, path)`, `WithOptimalSVG` | `SVG`, `WriteSVG`; the single-path renderer is the only one |
| `EncodeBatch`, `RenderBatch`, `BatchInput` | `Batch(ctx, []BatchJob, concurrency)` |
| `Decode(img) (string, error)`, `DecodeDetailed`, `SegmentInfo` | `Decode(img, opts...) (*DecodeResult, error)`, `DecodedSegment` |
| `ErrNoQRCode`, `ErrUnsupportedSymbol` | `ErrNotFound`, `ErrUnsupported` |
| `ErrInvalidConfig`, `ErrInvalidImageOutput` | `ErrInvalidArgument` |
| `BitBuffer`, `Ecc.FormatBits` | unexported |
| `generator encode -optimal`, `-border`, `-svg-optimized` | optimal by default (`-simple` opts out), `-quiet-zone`, `-svg` |

The default error correction level is `ECCMedium`, and `Encode` switches modes
optimally, including Kanji, so symbols can differ from v1 `EncodeText`.

### Added

- `Code.Version`, `Code.ECC`, `Code.Mask` and `String` methods on `ECC` and
  `Mode`.
- `WithUTF8ECI` declares UTF-8 with an ECI designator for non-ASCII text.
- `Code.WriteText` and `Code.String` render Unicode half-block text, and the
  CLI gains `-stdout text`.
- `Batch` takes a context, reports panics as errors, and accepts encode and
  render options per job.
- `ErrLogoTooLarge`; every error now wraps a sentinel.
- The decoder reads Kanji segments, ECI 26/1/3/20/27/170 byte data, inverted
  and mirrored symbols, low-contrast and unevenly lit images, transparent
  backgrounds and YCbCr images. It uses version information to size v7+
  symbols. `DecodeResult.Mirrored` reports mirror images.
- Testable examples replace the `example` module.

### Fixed

- `MakeSegmentsOptimally` hung for data needing a version above 27 and could
  return segments beyond the maximum version (#99, #100).
- `MakeSegmentsOptimally("")` panicked (#102).
- SVG output measured the quiet zone in user units instead of modules, so it
  was too thin and logos were drawn off-center.
- The logo pad is drawn in the background color instead of white.
- Malformed alphanumeric data could make the decoder index past its charset
  and panic; out-of-range numeric groups were accepted.
- The decoder picked spurious finder candidates in large symbols.
- `generator` help and README examples placed flags after the content, where
  they were encoded as text.

### Performance

- Optimal segmentation is about 6× faster with 60× fewer allocations.
- PNG rendering is about 10× faster (40 instead of 168,000 allocations), and
  PNGs without a logo are written as 1-bit paletted images.
- Decoding is faster on clean images (5–27%) and rotated images (14–19%), and
  images without contrast are rejected immediately (#109).

### Changed

- CI tests every package and the `tools` module with `-race` across Go
  versions, and runs gofmt, staticcheck, govulncheck and a fuzz smoke test.
- The module has no dependencies; testify was replaced by standard-library
  test helpers. Go 1.23 is required.

## [1.1.0] - 2026-05-29

### Added

- **Native QR decoder** (zero dependencies). `Decode(image.Image) (string,
  error)` and `DecodeDetailed` recover text, version, ECC level, mask, and
  per-segment info. A fast path handles crisp, axis-aligned images (the shape
  this library's renderers emit); a robust fallback locates finder patterns and
  corrects for rotation/noise via an affine transform. `WithFastPathOnly`
  disables the fallback. New sentinels: `ErrNoQRCode`, `ErrDecodeFailed`,
  `ErrUnsupportedSymbol`.
- Reed-Solomon decoding (syndromes, Berlekamp-Massey, Chien, Forney) sharing
  the encoder's GF(2^8) arithmetic.
- `generator decode <image>` CLI subcommand to decode an image and print its text.
- Fuzz tests for the decoder (`FuzzDecodeRoundTrip`, `FuzzDecodeNoPanic`) and a
  comparative decode benchmark harness under `tools/bench`.

### Changed

- `tools/verify` now wraps the native `go_qr.Decode` instead of gozxing; the
  verify path is dependency-free. gozxing remains only in `tools/bench` as a
  benchmark oracle.

## [1.0.0] - 2026-04-21

### Breaking changes

- **Image config API unified.** Colors now live on `QrCodeImgConfig` and are set
  via options, not on `BatchJob` or ad-hoc arguments.
  - New: `WithLight(color.Color)`, `WithDark(color.Color)`.
  - `WithSVGXMLHeader` is now a no-arg option: `WithSVGXMLHeader()` (was
    `WithSVGXMLHeader(bool)`).
  - `BatchJob.Light` / `BatchJob.Dark` string fields removed; configure colors
    on the shared `QrCodeImgConfig` instead.
- **Error sentinels.** Errors are now returned wrapped around exported
  sentinels so callers can use `errors.Is`:
  - `ErrInvalidConfig`, `ErrInvalidArgument`, `ErrInvalidVersion`,
    `ErrDataTooLong`, `ErrUnencodableChar`, `ErrInvalidImageOutput`.
  - `DataTooLongException` now implements `Unwrap() error` returning
    `ErrDataTooLong`; existing type assertions continue to work.

### Migration

```go
// before
cfg := NewQrCodeImgConfig(10, 4, WithSVGXMLHeader(true))
job := BatchJob{Text: "hi", Ecl: Low, Light: "#ffffff", Dark: "#000000"}

// after
cfg := NewQrCodeImgConfig(10, 4,
    WithSVGXMLHeader(),
    WithLight(color.White),
    WithDark(color.Black),
)
job := BatchJob{Text: "hi", Ecl: Low}
```

```go
// before
if err.Error() == "data too long" { ... }

// after
if errors.Is(err, ErrDataTooLong) { ... }
```

### Added

- Package-level godoc in `doc.go` covering quick start, config, in-memory
  rendering, batch, payloads, and error sentinels.
- `ToPNGBytes`, `ToSVGBytes`, `ToImage` for in-memory rendering without file
  I/O.
- Split render logic into `render_png.go` / `render_svg.go`; shared
  composition primitive `renderImage`.

### Changed

- Non-optimal SVG renderer rewritten to emit a single `<path>` with per-module
  subpaths; pre-sized `strings.Builder` and `strconv.AppendInt` scratch buffer
  reduce allocations to `2 allocs/op` (was ~1800) on the standard benchmark.
- `qr_code.go` split into focused files: `config.go`, `encode.go`, `mask.go`,
  `reedsolomon.go`, `render_png.go`, `render_svg.go`. No behavior change.
- README rewritten around the unified API.

### Removed

- Unused `docs/assets/` (old CLI demo GIFs and sample SVG referenced by the
  previous README).
