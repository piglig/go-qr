# Upgrading to v2

Version 2 moved to the module path `github.com/piglig/go-qr/v2`, renamed the
package to `qr`, and replaced positional parameters and configuration
structs with functional options. Version 1 continues to work at its old
path; both can be used in the same program while you migrate.

## Steps

1. Change the import and the package name:

   ```go
   // v1
   import go_qr "github.com/piglig/go-qr"
   // v2
   import "github.com/piglig/go-qr/v2" // package qr
   ```

2. Replace calls using the table below. The compiler finds every one.
3. Review behavior changes: the default error correction level is now M,
   and `Encode` uses optimal segmentation, so symbols can differ in size
   from v1's `EncodeText`. Pass `qr.WithSimpleSegmentation()` for v1's
   single-mode output.
4. If you passed a border to SVG output, note that the quiet zone is now in
   modules for every format; v1 SVG used user units.

## Before and after

```go
// v1
qr, err := go_qr.EncodeText("hello", go_qr.High)
cfg := go_qr.NewQrCodeImgConfig(10, 4, go_qr.WithDark(navy))
err = qr.PNG(cfg, "hello.png")
text, err := go_qr.Decode(img)

// v2
code, err := qr.Encode("hello", qr.WithECC(qr.ECCHigh))
png, err := code.PNG(qr.WithScale(10), qr.WithQuietZone(4), qr.WithForeground(navy))
err = os.WriteFile("hello.png", png, 0o644)
res, err := qr.Decode(img) // res.Text
```

## API mapping

| v1 | v2 |
| --- | --- |
| `EncodeText(text, ecl)` | `Encode(text, opts...)` |
| `EncodeBinary(data, ecl)` | `EncodeBytes(data, opts...)` |
| `EncodeStandardSegments(segs, ecl)`, `EncodeSegments(segs, ecl, min, max, mask, boost)` | `EncodeSegments(segs, opts...)` with `WithECC`, `WithVersionRange`, `WithMask`, `WithoutECCBoost` |
| `MakeSegmentsOptimally(text, ecl, min, max)` | default behavior of `Encode`; `WithSimpleSegmentation()` opts out |
| `MakeNumeric`, `MakeAlphanumeric`, `MakeBytes`, `MakeKanji`, `MakeEci`, `MakeSegments` | `NumericSegment`, `AlphanumericSegment`, `BytesSegment`, `KanjiSegment`, `ECISegment` |
| `QrCode`, `QrSegment` | `Code`, `Segment` |
| `Ecc`, `Low`, `Medium`, `Quartile`, `High` | `ECC`, `ECCLow`, `ECCMedium`, `ECCQuartile`, `ECCHigh` |
| `Numeric`, `Alphanumeric`, `Byte`, `Kanji`, `Eci` | `ModeNumeric`, `ModeAlphanumeric`, `ModeByte`, `ModeKanji`, `ModeECI` |
| `NewQrCodeImgConfig(scale, border, opts...)` | `WithScale(scale)`, `WithQuietZone(border)` on each render call |
| `WithLight`, `WithDark` | `WithBackground`, `WithForeground` |
| `ToImage`, `ToPNGBytes`, `WriteAsPNG`, `PNG(cfg, path)` | `Image`, `PNG`, `WritePNG`; write files with `os.WriteFile` |
| `ToSVGBytes`, `WriteAsSVG`, `SVG(cfg, path)`, `WithOptimalSVG` | `SVG`, `WriteSVG`; the compact renderer is the only one |
| `EncodeBatch`, `RenderBatch`, `BatchInput` | `Batch(ctx, []BatchJob, concurrency)` |
| `Decode(img) (string, error)`, `DecodeDetailed`, `SegmentInfo` | `Decode(img, opts...) (*DecodeResult, error)`, `DecodedSegment` |
| `ErrNoQRCode`, `ErrUnsupportedSymbol` | `ErrNotFound`, `ErrUnsupported` |
| `ErrInvalidConfig`, `ErrInvalidImageOutput` | `ErrInvalidArgument` |
| `BitBuffer`, `Ecc.FormatBits` | removed from the API |
| CLI `-optimal`, `-border`, `-svg-optimized` | optimal by default (`-simple` opts out), `-quiet-zone`, `-svg` |

The [changelog](../CHANGELOG.md#200---2026-10-07) lists everything that was
added and fixed in v2.
