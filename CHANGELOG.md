# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [2.5.0] - 2026-10-08

### Performance

- `Decode` uses the luminance of `*image.Gray` images and the luma plane of
  `*image.YCbCr` images, which `image/jpeg` returns, in place instead of
  copying it, and keeps its per-block statistics in bytes. Decoding a
  12-megapixel photo allocates 2 MB instead of 25 MB, a 48-megapixel one
  8 MB instead of 98 MB.
- The finder scan rejects run sequences whose center is not the longest
  before testing their ratios, which saves 10% on images without a code.

### Changed

- Byte segments without an ECI that are not UTF-8 are read by a model of
  text instead of fixed rules. Each segment is read both as Shift_JIS and as
  Windows-1252, and each reading is scored by how its characters follow
  each other in text. All undeclared segments of a symbol are judged
  together, and a Kanji mode segment counts for Shift_JIS. On 220 test
  strings (two sets written after the model was fixed) it picks the right
  character set for 217, against 206 for the rules of v2.4.1, which missed
  short half-width katakana words and Windows-1252 text.
- Undeclared single-byte text is read as Windows-1252, a superset of
  ISO-8859-1 for text, so “quotes”, dashes and € come out as such rather
  than as control codes. Segments with ECI 1 or 3 are still read as
  ISO-8859-1.

### Fixed

- A Shift_JIS lead byte followed by a byte that cannot end a Shift_JIS
  character, such as é followed by a space, no longer passes as Shift_JIS.

## [2.4.1] - 2026-10-08

### Fixed

- `Decode` read byte segments in Shift_JIS without an ECI, which Japanese
  encoders commonly write, as ISO-8859-1, so Japanese text came out as
  "ÃÞ»Þ²Ý". Such segments are now read as Shift_JIS when they form valid
  Shift_JIS with a run of katakana or kanji, or contain bytes that
  ISO-8859-1 leaves as control codes. Found on ZXing's test photos, where
  go-qr now reads 166 of 179 (zxing-cpp: 165).

### Changed

- The `generator` CLI (tools/v1.1.3) and `go-qr-mcp` (mcp/v0.1.2) are built
  on v2.4.0 and read photos as well as the library does.
- `go-qr-mcp`'s `decode_qr` no longer downscales images over 2000 pixels
  before decoding. The decoder now locates symbols at several scales
  itself and reads modules at full resolution; on the BoofCV photos,
  decoding at full resolution reads 18 more images and none fewer, for 6%
  more time.

## [2.4.0] - 2026-10-08

### Added

- `Decode` reads photos. On the 536 photos of the BoofCV QR Code dataset it
  reads a code in 77% of them, up from 21% (zxing-cpp 73%, OpenCV's WeChat
  decoder 69%, ZBar 46%), and takes 30% less time doing so. The robust path
  was rebuilt:
  - The module grid is fitted to the symbol's own structure. The corners
    of each finder's nested squares give a homography by least squares,
    so perspective is measured rather than guessed; the timing patterns
    choose the size and verify which finder is which; the alignment
    patterns refine the fit, with a cubic lens correction kept when the
    fixed patterns confirm it. One grid is decoded. Tilts up to 40°
    decode, and most at 50°.
  - When a finder is lost to glare, damage or the image edge, pairs of
    well-confirmed finders imply the third.
  - Modules are read from the grayscale image, each against the modules
    around it, rather than from a binarized one, so module size and uneven
    lighting no longer matter.
  - If nothing is found at full resolution, the symbol is located again in
    the image halved, quartered and so on, which removes texture finer than
    the modules, such as a screen's pixel grid; modules are still read at
    full resolution. Photos of screens and codes with very large modules
    now decode.
  - The local threshold handles modules larger than its window, and large
    images are scanned for finders every second or third row.
  Clean images take the unchanged fast path. Small images that need the
  robust path, which large photos make up for, are up to about 50% slower,
  whether or not they decode. See
  [Performance](docs/explanation/performance.md#reading-photos).
- `TestBoofCV` in `tools/bench` runs go-qr and gozxing over the BoofCV
  dataset, and `TestRobustness` sweeps tilt, module size, blur and lens
  distortion through a simulated camera.

### Changed

- The `generator` CLI (tools/v1.1.1) and `go-qr-mcp` (mcp/v0.1.1) are built
  on v2.3.0, so they reject IBANs with wrong check digits when creating EPC
  payment codes. `go-qr-mcp` uses `EPC.Validate` for its `iban-invalid`
  signal instead of its own checksum.

### Fixed

- The robust decode path misjudged the size of symbols rotated by 30° to
  60°, because it measured module sizes along the image axes, and rejected
  the finders of rotated version 1 symbols.
- Finder triples were ranked by shape alone, so data that mimicked a finder
  on a row or two could displace a real one; strongly foreshortened finders
  were rejected outright.
- The `generator` CLI no longer prints "payload: payload:" when the payload
  package rejects an EPC payment.

## [2.3.0] - 2026-10-07

### Added

- `go-qr-mcp` (module `github.com/piglig/go-qr/mcp`, mcp/v0.1.0), a Model
  Context Protocol server for AI assistants with three tools: `decode_qr`
  decodes local images exactly, `generate_qr` creates plain, payload and
  styled codes and verifies each one, and `inspect_qr` explains what content
  does with risk signals (lookalike or disguised URLs, link shorteners, open
  Wi-Fi, 2FA secrets, invalid IBANs, USSD codes, text addressed to an AI).
  `-root` confines file access. See the [MCP guide](docs/guides/mcp.md).

### Fixed

- `Decode` could fail on a clean, axis-aligned image when the symbol's top
  row happened to read 7:5:9:5:7 (the finder edge followed by data), which
  the fast path mistook for the finder pattern at five times the real module
  size. It occurred in about 1 in 10,000 symbols with a fixed mask, and in
  small images the robust path could not recover. Found by `FuzzEncode`.
- `EPC.Validate` checks the IBAN's ISO 13616 mod-97 check digits and its
  country code, not only its length, so a mistyped IBAN is rejected when
  the code is created instead of when someone scans it.

### Changed

- `Code.Verify` names colors in its errors as `#RRGGBB` instead of Go struct
  syntax.
- The `generator` CLI (tools/v1.1.0) is built on v2.2.0 and exposes its
  options: `-gs1` (including the `(01)...` human-readable form),
  `-structured`, `-utf8-eci`, `-mask`, `-min-version`/`-max-version`,
  `-no-boost`, style flags (`-module`, `-finder`, `-fg`, `-bg`,
  `-gradient`, `-finder-color`), and the `contact`, `event`, `otp` and
  `epc` payloads. `-verify` now uses `Code.Verify`, including the contrast
  checks. `decode` accepts several images, joins structured append
  sequences, and prints details with `-json`.

## [2.2.0] - 2026-10-07

### Added

- `WithModuleShape` (`ModuleSquare`, `ModuleDot`, `ModuleRounded`) and
  `WithFinderShape` (`FinderSquare`, `FinderRounded`, `FinderCircle`) style
  PNG, SVG and `Image` output with anti-aliased edges. Every combination
  keeps module centers and the finder proportions readers rely on.
- `WithFinderColor` colors the finder rings and centers, and `WithGradient`
  paints the dark modules with a linear gradient (SVG `<linearGradient>`).
- `Code.Verify` checks the contrast of every dark color in use: the
  foreground, both ends of a gradient, and the finder colors.

### Fixed

- With `WithGS1`, a GS separator followed by another separator or by '%'
  was written in an alphanumeric segment as "%%", which reads back as a
  literal '%'. Such separators are now carried in byte mode. Found by the
  new encoder fuzz tests.

### Changed

- The documentation is reorganized into a short README and a `docs/`
  directory with a tutorial, task guides, reference pages and background,
  plus contributing and security guides.
- The decoder's fast path measures the module pitch across the center of
  the top-left finder instead of along its top edge, so styled finders
  decode without the slower fallback.

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
