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
none was found. go-qr returns one code per image, which costs it nothing
here but in multi-code images it reads one code of many.

| Category | go-qr | [zxing-cpp] 3.1 | WeChat (OpenCV 4.10) | ZBar | OpenCV | [gozxing] |
| --- | --- | --- | --- | --- | --- | --- |
| nominal | 90.8% | **96.9%** | 90.8% | 73.8% | 53.8% | 60.0% |
| perspective | **71.4%** | 62.9% | 45.7% | 42.9% | 37.1% | 37.1% |
| rotations | **100%** | **100%** | **100%** | 61.4% | 95.5% | 45.5% |
| close | **100%** | **100%** | 65.0% | 12.5% | 70.0% | 5.0% |
| monitor | **100%** | **100%** | 94.1% | 0% | **100%** | 0% |
| blurred | **73.3%** | 71.1% | 68.9% | 44.4% | 31.1% | 26.7% |
| curved | **72.0%** | 66.0% | 52.0% | 42.0% | 30.0% | 34.0% |
| damaged | 43.2% | 24.3% | **45.9%** | 21.6% | 10.8% | 10.8% |
| glare | 58.0% | 40.0% | **72.0%** | 38.0% | 14.0% | 22.0% |
| shadows | 92.9% | 92.9% | **100%** | 78.6% | 57.1% | 71.4% |
| brightness | 78.6% | **96.4%** | 78.6% | 64.3% | 28.6% | 71.4% |
| bright spots | 53.1% | 40.6% | **59.4%** | 43.8% | 40.6% | 46.9% |
| high version | 66.7% | **97.0%** | 21.2% | 21.2% | 6.1% | 6.1% |
| noncompliant | 87.5% | 62.5% | **93.8%** | 62.5% | 6.2% | 18.8% |
| pathological | 82.6% | 43.5% | **91.3%** | 65.2% | 4.3% | 34.8% |
| lots | **100%** | **100%** | 0% | **100%** | **100%** | **100%** |
| **all photos** | **77.1%** | 73.1% | 68.8% | 45.7% | 40.1% | 34.1% |
| decode time, all photos | 8.7 s | **5.5 s** | 106 s | 61 s | 171 s | 34 s |

WeChat uses a CNN detector and super-resolution; OpenCV's QRCodeDetector
reads with quirc. Large symbols are go-qr's weak spot: version 20 and up
decode in two thirds of the photos where zxing-cpp, which fits a local
transform between each pair of alignment patterns, reads nearly all.

The thresholds the decoder uses were tuned on this dataset, so these
figures are optimistic for go-qr; the next table, on photos it was not
tuned on, puts it just behind zxing-cpp. To reproduce them:

```shell
go test -run=TestBoofCV -timeout=60m ./bench/ -boofcv=/path/to/qrcodes/detection
```

`TestBoofCV` writes one line per image; the other decoders were run from
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
| ZXing photos | **166** | 165 | 155 | 155 | 154 | 91 |
| zxing-cpp photos | 16 | **20** | 16 | 16 | 10 | 7 |
| **all 200** | 91.0% | **92.5%** | 85.5% | 85.5% | 82.0% | 49.0% |

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
| barrel distortion k₁ = −0.1 | **84%** | 56% |
| barrel distortion k₁ = −0.15 | **66%** | 41% |
| phone mix | **98%** | 42% |

The simulated camera has no glare, texture or damage, which makes these
sweeps kinder than real photos; the BoofCV figures above are the better
guide.

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

### Decoding large images

Decoding a photo-sized image that holds a small code. The luminance of an
`*image.YCbCr`, which `image/jpeg` returns for color photos, or of an
`*image.Gray` is usually read in place; other image types are converted
first.

| Image | Time | Memory |
| --- | --- | --- |
| 12 megapixels | 46 ms | 2.0 MB |
| 12 megapixels, no code | 102 ms | 7.3 MB |
| 48 megapixels | 185 ms | 7.6 MB |

An image without a code costs more than one with a code, because every
scale is searched.
Decoding JPEG with `image/jpeg` usually takes longer than reading the code.

## Methodology

Intel Core i7-14700KF, Go 1.25, Windows, best of five `go test -bench`
runs on an otherwise idle machine. Absolute numbers vary between machines;
ratios are more stable. The comparisons live in the
[`tools/bench`](../../tools/bench) module, which depends on the other
libraries so that the main module does not; its README lists the commands
for the comparisons.

Feature benchmarks are in the main module, for example
`go test -run='^$' -bench='PNG|SVG|Styled|Colored' -benchmem .`.

[skip2/go-qrcode]: https://github.com/skip2/go-qrcode
[boombuler/barcode]: https://github.com/boombuler/barcode
[gozxing]: https://github.com/makiuchi-d/gozxing
[zxing-cpp]: https://github.com/zxing-cpp/zxing-cpp
