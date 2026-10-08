# Benchmarks and accuracy tests

Compares go-qr with other QR Code libraries. It lives in the `tools` module
because it imports them; the library module stays dependency-free. The
results are published in [Performance](../../docs/explanation/performance.md).

| Test | Compares | Measures |
| --- | --- | --- |
| `BenchmarkEncodeCompare` | go-qr, [skip2/go-qrcode], [boombuler/barcode] | Encoding time and allocations. |
| `BenchmarkDecodeClean` | go-qr, [gozxing] | Decoding time and allocations on crisp rendered images. |
| `TestDecodeAccuracy` | go-qr, gozxing | Decode rate on rendered images, clean and rotated with noise. |
| `TestRobustness` | go-qr, gozxing | Decode rate through a simulated camera: tilt, module size, blur, lens distortion and a random phone mix. |
| `TestBoofCV` | go-qr, gozxing | Decode results on the real photos of the BoofCV dataset. |

## Running

Benchmark your checkout rather than the released library with a workspace
at the repository root:

```shell
go work init . ./tools
cd tools

go test -run='^$' -bench='EncodeCompare|DecodeClean' -benchmem ./bench/
go test -run=TestDecodeAccuracy -v ./bench/
go test -run=TestRobustness -v ./bench/ -sweep
go test -run=TestBoofCV -timeout=60m ./bench/ -boofcv=/path/to/qrcodes/detection -boofcv-out=go.jsonl
```

Flags such as `-sweep` must follow the package path. Without them, the
sweeps and the dataset test are skipped. `-sweep-native` runs the sweeps for
go-qr only, and `-sweep-out file.json` writes its decode counts per sweep
point, which the regression check in CI compares between two versions with
`go run ./regress sweep old.json new.json`.

**`TestRobustness`** renders symbols through a pinhole camera model
(`distort.go`: tilt about any axis, rotation, pixels per module, Gaussian
blur, noise and radial lens distortion) and tabulates decode rates per
decoder and version. A decode counts only if the text matches.
`DistortWithTruth` also returns where each module lands, for measuring
localization error.

**`TestBoofCV`** reads the
[BoofCV QR Code dataset](https://boofcv.org/index.php?title=Performance:QrCode)
(`qrcodes_v3.zip`, about 200 MB, downloaded separately; its license is
unstated, so it is not vendored). It writes one JSON line per decoder and
image with the texts read and the decode time. The dataset labels the
codes' corners but not their contents, so results are scored by agreement
between decoders.

## Adding a decoder

Append a `func(image.Image) (string, error)` to the registry in
`decode_bench_test.go`; every benchmark and accuracy test then runs it:

```go
var decoders = []decoderImpl{
	{"gozxing", decodeGozxing},
	{"native", decodeNative},
}
```

[skip2/go-qrcode]: https://github.com/skip2/go-qrcode
[boombuler/barcode]: https://github.com/boombuler/barcode
[gozxing]: https://github.com/makiuchi-d/gozxing
