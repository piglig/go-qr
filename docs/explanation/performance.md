# Performance

## Compared with other libraries

Encoding text to a symbol, without rendering, at error correction level M.
All three libraries run the full eight-mask penalty selection; skip2 defers
it to `Bitmap()`, which the benchmark calls.

| Payload | go-qr | [skip2/go-qrcode] | [boombuler/barcode] |
| --- | --- | --- | --- |
| numeric, 8 chars (v1) | **8.1 µs** · 16 allocs | 70 µs · 934 allocs (8.6×) | 397 µs · 172 allocs (49×) |
| alphanumeric, 14 chars (v1) | **8.6 µs** · 18 allocs | 72 µs · 936 allocs (8.4×) | 408 µs · 181 allocs (47×) |
| URL, 45 chars (v4) | **16 µs** · 22 allocs | 265 µs · 3,450 allocs (16×) | 1,387 µs · 578 allocs (86×) |
| text, 672 chars (v20) | **0.15 ms** · 128 allocs | 3.19 ms · 49,099 allocs (21×) | 16.3 ms · 5,891 allocs (109×) |

Decoding crisp rendered images, compared with [gozxing], a Go port of ZXing:

| Symbol | go-qr | gozxing |
| --- | --- | --- |
| numeric (v1) | **113 µs** · 20 allocs | 684 µs · 53,914 allocs (6.1×) |
| URL (v4) | **224 µs** · 26 allocs | 1,440 µs · 107,701 allocs (6.4×) |
| text (v29) | **2.3 ms** · 117 allocs | 17.5 ms · 1.27M allocs (7.6×) |

### Reading photos

The [BoofCV QR Code dataset](https://boofcv.org/index.php?title=Performance:QrCode)
holds 536 photos with 1,232 codes in 16 categories, from nominal shots to
glare, curved surfaces and photos of screens. The table gives the share of
photos in which a decoder read at least one code. The dataset labels the
codes' corners but not their contents, so a decode counts when its text
matches what another decoder, or the same decoder in another photo of the
same code, read; QR Codes' error correction makes misreads very rare, and
none was found. `Decode` returns one code per image, which costs it
nothing here; `DecodeAll`, which reads every code, is compared below.

| Category | go-qr | [zxing-cpp] 3.1 | WeChat (OpenCV 4.10) | ZBar | OpenCV | [gozxing] |
| --- | --- | --- | --- | --- | --- | --- |
| nominal | 90.8% | **96.9%** | 90.8% | 73.8% | 53.8% | 60.0% |
| perspective | **71.4%** | 62.9% | 45.7% | 42.9% | 37.1% | 37.1% |
| rotations | **100%** | **100%** | **100%** | 61.4% | 95.5% | 45.5% |
| close | **100%** | **100%** | 65.0% | 12.5% | 70.0% | 5.0% |
| monitor | **100%** | **100%** | 94.1% | 0% | **100%** | 0% |
| blurred | **82.2%** | 71.1% | 68.9% | 44.4% | 31.1% | 26.7% |
| curved | **72.0%** | 66.0% | 52.0% | 42.0% | 30.0% | 34.0% |
| damaged | 43.2% | 24.3% | **45.9%** | 21.6% | 10.8% | 10.8% |
| glare | 68.0% | 40.0% | **72.0%** | 38.0% | 14.0% | 22.0% |
| shadows | 92.9% | 92.9% | **100%** | 78.6% | 57.1% | 71.4% |
| brightness | 85.7% | **96.4%** | 78.6% | 64.3% | 28.6% | 71.4% |
| bright spots | 56.2% | 40.6% | **59.4%** | 43.8% | 40.6% | 46.9% |
| high version | 84.8% | **97.0%** | 21.2% | 21.2% | 6.1% | 6.1% |
| noncompliant | **93.8%** | 62.5% | **93.8%** | 62.5% | 6.2% | 18.8% |
| pathological | 82.6% | 43.5% | **91.3%** | 65.2% | 4.3% | 34.8% |
| lots | **100%** | **100%** | 0% | **100%** | **100%** | **100%** |
| **all photos** | **80.6%** | 73.1% | 68.8% | 45.7% | 40.1% | 34.1% |
| decode time, all photos | **4.4 s** | 5.5 s | 106 s | 61 s | 171 s | 34 s |

Counting codes rather than photos, `DecodeAll` (*since v2.6*) reads what
`Decode` reads and the other codes in photos of several:

| Codes decoded | go-qr `Decode` | go-qr `DecodeAll` | [zxing-cpp] 3.1 | WeChat | ZBar | OpenCV | [gozxing] |
| --- | --- | --- | --- | --- | --- | --- | --- |
| lots (7 photos, 420 codes) | 1.7% | **23.3%** | **23.3%** | 0% | 14.0% | 22.6% | 22.6% |
| rotations (44 photos, 133 codes) | 33.1% | **99.2%** | 98.5% | 82.0% | 48.1% | 66.2% | 42.1% |
| **all codes** | 35.1% | **57.1%** | 52.9% | 41.2% | 30.7% | 30.4% | 28.4% |
| median decode time per photo | **3.5 ms** | 13.7 ms | 5.8 ms | 19.2 ms | 36.0 ms | 70.5 ms | 13.4 ms |

WeChat uses a CNN detector and super-resolution; OpenCV's QRCodeDetector
reads with quirc. Large symbols are go-qr's weak spot: version 20 and up
decode in 85% of the photos, where zxing-cpp, which fits a local transform
between each pair of alignment patterns, reads nearly all. Since v2.7 the
grid is corrected locally by the alignment patterns' measured positions,
which raised that from 67%; the four photos still missed are version 40
codes whose global fit is too far off for the alignment patterns to be
found.

The thresholds the decoder uses were tuned on this dataset, so these
figures are optimistic for go-qr; the next table, on photos it was not
tuned on, puts it just behind zxing-cpp. To reproduce them:

```shell
go test -run=TestBoofCV -timeout=60m ./bench/ -boofcv=/path/to/qrcodes/detection
```

`TestBoofCV` writes one line per image; go-qr's times in the tables were
measured by decoding each photo repeatedly for at least 20 ms, since the
system clock rounds a single decode of a few milliseconds. The other
decoders were run from
Python on the same grayscale images.

### Photos the decoder was not tuned on

The QR Code test photos of [ZXing](https://github.com/zxing/zxing/tree/master/core/src/test/resources/blackbox)
(179 photos) and the further ones of [zxing-cpp](https://github.com/zxing-cpp/zxing-cpp/tree/master/test/samples)
(21 photos with expected texts and not in ZXing's set) come with the
expected text of each code, so a decode counts only if its text matches.
These are those projects' own regression sets, which favors them; go-qr
was never tuned on them.

| | go-qr | zxing-cpp | WeChat | gozxing | ZBar | OpenCV |
| --- | --- | --- | --- | --- | --- | --- |
| ZXing photos | **167** | 165 | 155 | 155 | 154 | 91 |
| zxing-cpp photos | 16 | **20** | 16 | 16 | 10 | 7 |
| **all 200** | 91.5% | **92.5%** | 85.5% | 85.5% | 82.0% | 49.0% |

No decoder misread a code's data. Three ZBar results and eight OpenCV ones
interpret a text's character set differently, and zxing-cpp's Python
binding writes two control characters as `<SOH>` and `<DLE>`; these count as
misses above. Evaluating on these sets found go-qr reading undeclared
Japanese Shift_JIS as ISO-8859-1, which v2.4.1 fixes.

### Simulated distortion

`TestRobustness` in `tools/bench` photographs symbols of versions 1, 3, 7
and 12 with a simulated pinhole camera, 32 images per point, with random
rotation, slight blur and noise. Each row varies one parameter; "phone
mix" randomizes all of them (tilt up to 35°, 3 to 8 pixels per module,
blur, noise and mild barrel distortion), 256 images. A decode counts only
if the text matches; neither decoder returned a wrong text.

| Distortion | go-qr | gozxing (`TRY_HARDER`) |
| --- | --- | --- |
| tilt 30° | **100%** | 31% |
| tilt 40° | **100%** | 0% |
| tilt 50° | **72%** | 0% |
| tilt 60° | **44%** | 0% |
| 2 px per module | **91%** | 75% |
| 1.5 px per module | **66%** | 28% |
| blur σ 2 px | **100%** | 56% |
| barrel distortion k₁ = −0.1 | **97%** | 56% |
| barrel distortion k₁ = −0.15 | **69%** | 41% |
| phone mix | **98%** | 42% |

The simulated camera has no glare, texture or damage, which makes these
sweeps kinder than real photos; the BoofCV figures above are the better
guide.

## Cost of features

A version 4 code at scale 10, as in the `PNG`, `StyledPNG`, `ColoredPNG`,
`SVG` and `StyledSVG` benchmarks:

| Output | Time | Relative |
| --- | --- | --- |
| `PNG()` | 0.14 ms | 1× |
| `PNG()` with shapes | 1.6 ms | 11× |
| `PNG()` with a gradient or finder colors | ~5 ms | ~38× |
| `SVG()` | 34 µs | |
| `SVG()` with shapes | 180 µs | |

- Optimal segmentation costs 1–2% over `WithSimpleSegmentation` for typical
  text.
- PNG time is dominated by compression and grows with the pixel count;
  lower `WithScale` for screens. Plain two-color PNGs are written straight
  from the modules at one bit per pixel; shapes need 8-bit gray levels for
  their antialiased edges.
- Gradients and finder colors need full-color PNGs, whose compression is the
  main cost. SVG is the fast choice for styled output.
- `Verify` renders and decodes once, about the cost of `Image` plus
  `Decode`.

### Decoding large images

Decoding a photo-sized image that holds a small code. The luminance of an
`*image.YCbCr`, which `image/jpeg` returns for color photos, or of an
`*image.Gray` is usually read in place; other image types are converted
first.

| Image | Time | Memory |
| --- | --- | --- |
| 12 megapixels | 23 ms | 5.4 MB |
| 12 megapixels, no code | 81 ms | 8.5 MB |
| 48 megapixels | 53 ms | 19.0 MB |

These are the worst case for the search: a version 4 code at 3 pixels per
module on random texture, which half resolution barely resolves. Codes in
camera photos usually take far less: the median over the 12-megapixel
BoofCV photos is 11 ms. An image without a code costs more than one with
a code, because every scale is searched.
Decoding JPEG with `image/jpeg` usually takes longer than reading the code.

## Methodology

Intel Core i7-14700KF, Go 1.25, Windows, best of five `go test -bench`
runs on an otherwise idle machine, pinned to the performance cores: on
its efficiency cores the same benchmarks run up to twice as slow.
Absolute numbers vary between machines; ratios are more stable. The
comparisons live in the [`tools/bench`](../../tools/bench) module, which
depends on the other libraries so that the main module does not; its
README lists the commands for the comparisons.

Feature benchmarks are in the main module, for example
`go test -run='^$' -bench='PNG|SVG|Styled|Colored' -benchmem .`.

[skip2/go-qrcode]: https://github.com/skip2/go-qrcode
[boombuler/barcode]: https://github.com/boombuler/barcode
[gozxing]: https://github.com/makiuchi-d/gozxing
[zxing-cpp]: https://github.com/zxing-cpp/zxing-cpp
