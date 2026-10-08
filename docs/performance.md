# Performance

## Compared with other libraries

Encoding text to a symbol, without rendering, at error correction level M.
All three libraries run the full eight-mask penalty selection; skip2 defers
it to `Bitmap()`, which the benchmark calls.

| Payload | go-qr | [skip2/go-qrcode] | [boombuler/barcode] |
| --- | --- | --- | --- |
| numeric, 8 chars (v1) | **25 µs** · 17 allocs | 74 µs · 934 allocs (2.9×) | 407 µs · 172 allocs (16×) |
| alphanumeric, 14 chars (v1) | **26 µs** · 19 allocs | 76 µs · 936 allocs (3.0×) | 414 µs · 181 allocs (16×) |
| URL, 45 chars (v4) | **101 µs** · 23 allocs | 278 µs · 3,450 allocs (2.8×) | 1,425 µs · 578 allocs (14×) |
| text, 672 chars (v20) | **1.07 ms** · 129 allocs | 3.35 ms · 49,099 allocs (3.1×) | 16.3 ms · 5,891 allocs (15×) |

Decoding crisp rendered images, compared with [gozxing], a Go port of ZXing:

| Symbol | go-qr | gozxing |
| --- | --- | --- |
| numeric (v1) | **109 µs** · 17 allocs | 686 µs · 53,914 allocs (6.3×) |
| URL (v4) | **206 µs** · 23 allocs | 1,399 µs · 107,701 allocs (6.8×) |
| text (v29) | **2.3 ms** · 114 allocs | 16.9 ms · 1.27M allocs (7.4×) |

Robustness, from `TestRobustness` in `tools/bench`: symbols of versions 1,
3, 7 and 12 photographed by a simulated pinhole camera, 32 images per point,
with random rotation, slight blur and noise. Each row varies one parameter;
"phone mix" randomizes all of them (tilt up to 35°, 3 to 8 pixels per
module, blur, noise and mild barrel distortion), 256 images. A decode counts
only if the text matches; neither decoder returned a wrong text.

| Distortion | go-qr | gozxing (`TRY_HARDER`) |
| --- | --- | --- |
| tilt 30° | **100%** | 31% |
| tilt 40° | **100%** | 0% |
| tilt 50° | **72%** | 0% |
| tilt 60° | **44%** | 0% |
| 2 px per module | **78%** | 75% |
| 1.5 px per module | **44%** | 28% |
| blur σ 2 px | **100%** | 56% |
| barrel distortion k₁ = −0.1 | **84%** | 56% |
| barrel distortion k₁ = −0.15 | **66%** | 41% |
| phone mix | **98%** | 42% |

Large symbols under strong barrel distortion remain the weak spot: version
12 decodes 38% of the time at k₁ = −0.1, where the cubic correction cannot
follow the distortion to the symbol's far corner.

## Cost of features

A version 3 code at scale 10:

| Output | Time | Relative |
| --- | --- | --- |
| `PNG()` | 0.58 ms | 1× |
| `PNG()` with shapes | 1.6 ms | 2.7× |
| `PNG()` with a gradient or finder colors | ~6 ms | ~10× |
| `SVG()` | 34 µs | |
| `SVG()` with shapes | 180 µs | |

- Optimal segmentation costs 1–2% over `WithSimpleSegmentation` for typical
  text.
- PNG time is dominated by compression and grows with the pixel count;
  lower `WithScale` for screens.
- Gradients and finder colors need full-color PNGs, whose compression is the
  main cost. SVG is the fast choice for styled output.
- `Verify` renders and decodes once, about the cost of `Image` plus
  `Decode`.

## Methodology

Intel Core i7-14700KF, Go 1.25, Windows, best of five `go test -bench`
runs on an otherwise idle machine. Absolute numbers vary between machines;
ratios are more stable. The comparisons live in the
[`tools/bench`](../tools/bench) module, which depends on the other libraries
so that the main module does not:

```shell
go work init . ./tools   # benchmark the local checkout
cd tools
go test -run='^$' -bench='EncodeCompare|DecodeClean' -benchmem ./bench/
go test -run=TestDecodeAccuracy -v ./bench/
```

Feature benchmarks are in the main module, for example
`go test -run='^$' -bench='PNG|SVG|Styled|Colored' -benchmem .`.

[skip2/go-qrcode]: https://github.com/skip2/go-qrcode
[boombuler/barcode]: https://github.com/boombuler/barcode
[gozxing]: https://github.com/makiuchi-d/gozxing
