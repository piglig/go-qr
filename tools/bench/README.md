# Benchmark harness

Comparative benchmarks for QR **encoders** (`BenchmarkEncodeCompare`: go-qr,
skip2/go-qrcode, boombuler/barcode) and **decoders** (`BenchmarkDecodeClean`,
`TestDecodeAccuracy`: go-qr's native `qr.Decode` and gozxing) over a shared
corpus, reporting throughput, allocations, and decode success rate. It lives in
the `tools` submodule because it imports those libraries; the main `go-qr`
module stays dependency-free.

## What it measures

| Metric | Where | Why |
| --- | --- | --- |
| `ns/op` | `BenchmarkDecodeClean` | Latency on crisp, self-generated images (the verify path). |
| `B/op`, `allocs/op` | `BenchmarkDecodeClean` (`-benchmem`) | GC pressure — the clearest Go-native-vs-Java-port win surface. |
| success rate | `TestDecodeAccuracy` | Robustness on clean vs degraded corpora. Latency is meaningless if it can't read the code. |

## Corpus

`corpus.go` generates, via `go-qr` itself:

- **Clean** — 5 inputs spanning the version range (short numeric → high-version
  byte payload), rendered axis-aligned at `benchScale` px/module. This is the
  case a specialized decoder should dominate.
- **Degraded** — the clean set put through a seeded 7° rotation + Gaussian
  sensor noise (stdlib only, deterministic). This is the robustness column where
  the ZXing family is expected to lead.

## Adding a decoder

The harness is pluggable: append a `func(image.Image) (string, error)` to the
registry in `decode_bench_test.go` and every benchmark and accuracy case runs
against it:

```go
var decoders = []decoderImpl{
	{"gozxing", decodeGozxing},
	{"native", decodeNative},
}
```

## Running

```shell
cd tools

# Encoder and decoder throughput + allocations
go test -run=^$ -bench='EncodeCompare|DecodeClean' -benchmem ./bench/

# Robustness table (clean vs degraded success rate)
go test -run=TestDecodeAccuracy -v ./bench/
```

## Baseline (gozxing, 8 px/module, this machine — replace with your own)

| Case | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| numeric_short | 778,341 | 291,712 | 53,914 |
| url_medium | 1,567,514 | 582,134 | 107,701 |
| byte_long (high version) | 18,785,784 | 6,747,575 | 1,272,821 |

Accuracy: clean 5/5 (100%), degraded 4/5 (80%).

### Reading the baseline

The headline is the **allocation count**: 54k–1.3M allocations to decode a
*single* QR code. That is the Java-port tax (BitMatrix objects, boxed results,
interface dispatch) and it is the most reachable win for a native decoder. The
`byte_long` case (18 ms, 6.7 MB) shows it scales super-linearly with version —
batch decoding of large codes is where a native fast path would pay off most.

## Acceptance targets for the native decoder

Set before building so "did it work" is objective:

- **Clean path:** ≥ 5× faster `ns/op` and ≤ 10% of gozxing's `allocs/op` on
  every clean case; 5/5 accuracy.
- **Degraded path:** no worse than `gozxing - 1` on the degraded success count.
  We are not trying to beat ZXing on robustness — only to not regress the
  verify use case.

If the clean-path target is missed, the dependency-elimination argument still
stands on its own; if the degraded target is badly missed, keep gozxing as an
optional robustness fallback rather than ripping it out.
