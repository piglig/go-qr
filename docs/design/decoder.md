# Native QR Decoder — Architecture Design

Status: **implemented** in v1.1 and extended in v2 · This is the original
design proposal, kept for its rationale. See [§12](#12-v2-changes) for how the
shipped decoder differs, and the package documentation for the current API.

A zero-dependency QR decoder living in the main `go-qr` module, mirroring the
existing encoder. Closes the generate ⇄ decode loop, removes the gozxing
dependency from the verify path, and provides a specialized fast path for
self-generated images.

---

## 1. Goals / non-goals

**Goals**

- Decode standard Model-2 QR codes (all 40 versions, all 4 ECC levels) from a
  Go `image.Image` with **zero external dependencies**.
- A **fast path** for crisp, axis-aligned images (our own rendered output) that
  decisively beats gozxing on `ns/op` and `allocs/op` (see acceptance targets in
  `tools/bench/README.md`).
- A **robust path** for mildly degraded real-world images (rotation, skew,
  noise) that is "good enough" to not regress the verify use case.
- Reuse the encoder's GF(2^8) arithmetic and spec tables — no second source of
  truth for version/ECC constants.

**Non-goals (explicitly out of scope for v1)**

- Beating ZXing/zxing-cpp on hard real-world photos (severe perspective, low
  light, blur). Robustness is a *fallback*, not a competitive target.
- Micro QR, rMQR, Model-1, multi-code detection in one image, color QR.
- Sub-pixel/multi-sample bilinear correction beyond what the robust path needs.

---

## 2. Public API

Lives in the main module so the verify tool and CLI can drop gozxing.

```go
package go_qr

// Decode reads the first QR code found in img and returns its decoded text.
func Decode(img image.Image) (string, error)

// DecodeResult carries structured output for callers that need more than text.
type DecodeResult struct {
    Text    string
    Version int
    Ecc     Ecc
    Mask    int
    Segments []SegmentInfo // mode + raw bytes per segment
}

// DecodeDetailed is the full-fidelity entry point.
func DecodeDetailed(img image.Image, opts ...DecodeOption) (*DecodeResult, error)

type DecodeOption func(*decodeConfig)

// WithFastPathOnly skips finder-pattern search and assumes a crisp,
// axis-aligned, white-quiet-zone image (the shape ToImage produces). Errors
// out instead of falling back. Use in verify/CI for max speed.
func WithFastPathOnly() DecodeOption

// WithMinQuietZone, WithMaxVersion, etc. as needed.
```

`verify.RoundTrip` becomes a thin wrapper over `go_qr.Decode` and the gozxing
import is deleted from `tools/verify`.

---

## 3. Pipeline overview

```
image.Image
   │
   ▼
┌─────────────┐   fast path: global threshold (1 pass)
│ binarize    │   robust path: Wellner/Bradley moving-average adaptive (1 pass)
└─────────────┘
   │  bitmap (1 byte/px or packed bitset)
   ▼
┌─────────────┐   fast path: infer grid from quiet-zone + module pitch
│ locate      │   robust path: finder-pattern scan (1:1:3:1:1) → 3 capstones
└─────────────┘                + alignment pattern → perspective transform
   │  GridSampler (module(x,y) → bool)
   ▼
┌─────────────┐
│ sample grid │   size = ver*4+17, read each module center
└─────────────┘
   │  BitMatrix (reuse the [][]bool shape of QrCode.modules)
   ▼
┌─────────────┐
│ read format │   15-bit BCH(15,5) → mask + ECC level   (error-correctable)
│ read version│   18-bit BCH(18,6) for ver ≥ 7          (else size-derived)
└─────────────┘
   │
   ▼
┌─────────────┐
│ unmask      │   XOR mask pattern (self-inverse; reuse encoder formulas)
└─────────────┘
   │
   ▼
┌─────────────┐
│ read codewd │   zig-zag module walk (reverse of drawCodewords)
└─────────────┘
   │  raw codewords (interleaved)
   ▼
┌─────────────┐
│ de-interleave│  split into ECC blocks per spec tables
└─────────────┘
   │  per-block [data | ecc]
   ▼
┌─────────────┐
│ RS correct  │  syndromes → Berlekamp-Massey → Chien → Forney
└─────────────┘
   │  corrected data codewords
   ▼
┌─────────────┐
│ bitstream   │  mode indicator → count → payload, per segment
│ → segments  │  numeric / alnum / byte / kanji / ECI
└─────────────┘
   │
   ▼
  string
```

---

## 4. Stage design

### 4.1 Binarization

Two strategies behind one `binarizer` interface returning a `bitmap`
(`[]uint8`, one byte per pixel, 0/1 — quirc's proven memory layout).

- **Fast path — global threshold.** Single pass; threshold fixed near mid-gray
  (literature converges on ~125 for clean codes) or a cheap min/max midpoint.
  No histogram, no float. This is valid because `ToImage` emits pure
  black/white with a light quiet zone.
- **Robust path — moving-average adaptive (Wellner 1993 / Bradley 2007).**
  Single forward pass maintaining a running average of the last *s* pixels per
  row (s ≈ image width / 8); pixel is dark if it is `t%` below the local
  average. O(n), cache-friendly, no per-block float matrix — this is precisely
  why quirc is small and fast and why we prefer it over ZXing's block-based
  HybridBinarizer.

Reference: quirc (`identify.c`) uses exactly this moving-average threshold;
academic surveys confirm adaptive binarization dominates real-image cost.

### 4.2 Localization

- **Fast path.** Assume the image is the output of our renderer: scan the top
  and left quiet zones to find the first dark module, measure module pitch from
  a finder pattern's known 7-module width, and derive `(originX, originY, pitch)`.
  No finder search, no transform. If the assumptions fail (no clean quiet zone,
  non-uniform pitch) → error under `WithFastPathOnly`, else fall back to robust.
- **Robust path.** Classic finder detection:
  1. Row-scan for the `1:1:3:1:1` dark/light run ratio (the same ratio the
     encoder's penalty rule already encodes — see
     `finderPenaltyCountPatterns` in `mask.go`, reusable as a cross-check).
  2. Cluster candidate centers into 3 finder patterns (capstones).
  3. Order them (top-left / top-right / bottom-left) by pairwise distances.
  4. For ver ≥ 2, locate the bottom-right alignment pattern near its expected
     position (`alignmentPatternPositions(ver)`) to pin the 4th corner.
  5. Build a `PerspectiveTransform` (3×3 homography from 4 point pairs) →
     `GridSampler`.

### 4.3 Grid sampling

`size = ver*4 + 17`. For each `(col,row)` map to image space (fast path: affine;
robust path: homography) and read the binarized value at the mapped center.
Output is a `[][]bool` matching the existing `QrCode.modules` shape so
downstream logic can mirror the encoder.

### 4.4 Format & version info

- **Format (15 bits, BCH(15,5)).** Read the two redundant copies, XOR with the
  fixed mask `0x5412`, BCH-error-correct (≤3 bit errors via min Hamming
  distance over the 32 valid codewords — a 32-entry lookup is simplest and
  allocation-free). Yields ECC level (2 bits) + mask (3 bits).
- **Version (18 bits, BCH(18,6)), ver ≥ 7.** Two redundant copies; for ver < 7
  derive version from grid size directly.

### 4.5 Unmask

The mask formulas in `applyMask` (`mask.go`) are XOR and therefore
**self-inverse** — the same per-module condition flips bits back. Factor the
`switch msk` body into a shared `maskBit(msk, x, y) bool` used by both encoder
and decoder (refactor, no behavior change), then apply over non-function
modules.

### 4.6 Codeword reading

Reverse of the encoder's `drawCodewords`: walk columns right-to-left in pairs
(skipping the vertical timing column), zig-zagging up/down, reading 8 bits per
codeword from non-function modules. Function-module map is reconstructed the
same way the encoder builds `isFunction` (finder/timing/alignment/format/version
positions) — factor that placement into a shared helper.

### 4.7 De-interleave + Reed-Solomon correction

- De-interleave using `getEccCodeWordsPerBlock()` and
  `getNumErrorCorrectionBlocks()` (already in `encode.go`) to know block sizes.
- **RS decode (new code, the genuinely new math):**
  1. Compute syndromes `S_i = R(α^i)` using `reedSolomonMultiply` (already in
     `reedsolomon.go`, GF poly `0x11D`).
  2. **Berlekamp–Massey** → error-locator polynomial.
  3. **Chien search** → error positions.
  4. **Forney** → error magnitudes; needs a GF inverse (add `gfInverse` via
     `gfExp/gfLog` tables built once from `reedSolomonMultiply`).
  - If syndromes are all zero, skip — the common clean-image case costs almost
    nothing.

### 4.8 Bitstream → segments

Reverse of `qr_segment.go` encoding: read 4-bit mode indicator, then the
version-dependent character-count bits, then the payload, per mode
(numeric/alphanumeric/byte/kanji), honoring ECI and the terminator. Reuse the
existing alphanumeric charset table and the kanji/Shift-JIS path from
`qr_segment_advanced.go`.

---

## 5. Fast vs robust path switching

```
Decode(img):
  bm := binarizeFast(img)
  if grid, ok := inferGridFast(bm); ok {
      if res, err := decodeFromGrid(bm, grid); err == nil {
          return res            // common case: one cheap pass
      }
  }
  if fastPathOnly { return ErrDecodeFailed }
  bm = binarizeAdaptive(img)    // robust fallback
  grid := locateByFinders(bm)
  return decodeFromGrid(bm, grid)
```

The fast path is the default and falls through to robust automatically; RS
correction validates the result, so a wrong fast-path guess fails closed rather
than returning garbage.

---

## 6. Reuse map (existing symbols)

| Need | Existing symbol | File | Action |
| --- | --- | --- | --- |
| GF(2^8) multiply (poly 0x11D) | `reedSolomonMultiply` | `reedsolomon.go` | reuse as-is |
| GF inverse / log tables | — | — | **new** `gfTables` built from the multiply |
| ECC block geometry | `getEccCodeWordsPerBlock`, `getNumErrorCorrectionBlocks` | `encode.go` | reuse |
| data codeword count | `getNumDataCodewords` | `encode.go` | reuse |
| raw module count | `getNumRawDataModules` | `reedsolomon.go` | reuse |
| alignment positions | `getAlignmentPatternPositions` (method) | `qr_code.go:308` | extract free function `alignmentPatternPositions(ver)` |
| mask formulas | `applyMask` switch | `mask.go:22` | extract `maskBit(msk,x,y)`; XOR self-inverse |
| finder ratio check | `finderPenaltyCountPatterns` | `mask.go:125` | reuse as detector cross-check |
| bit helper | `getBit` | `qr_code.go:363` | reuse |
| segment charsets | alnum table, kanji path | `qr_segment*.go` | reuse for reverse decode |
| function-module map | `isFunction` placement | `qr_code.go` | extract shared placement helper |

Several refactors above are pure extractions (no behavior change) that benefit
the encoder's readability too.

---

## 7. New file layout (main module, package `go_qr`)

```
decode.go            // Decode / DecodeDetailed, config, path switching
decode_binarize.go   // global + adaptive binarizers, bitmap type
decode_locate.go     // fast grid inference + finder/alignment detection
decode_transform.go  // perspective transform + grid sampler
decode_format.go     // format/version BCH decode
decode_extract.go    // unmask, codeword walk, de-interleave
decode_rs.go         // syndromes, Berlekamp-Massey, Chien, Forney, gf tables
decode_segments.go   // bitstream → segments → string
decode_test.go       // unit + table tests
```

`gfTables` (exp/log) shared by encoder and decoder can live in `reedsolomon.go`.

---

## 8. Error model

New sentinels, consistent with the existing `errors.Is` scheme in `error.go`:

| Sentinel | Meaning |
| --- | --- |
| `ErrNoQRCode` | No QR code located in the image. |
| `ErrDecodeFailed` | Located but uncorrectable (RS exhausted, bad format info). |
| `ErrUnsupportedSymbol` | Detected a variant we don't decode (Micro QR, etc.). |

Reuse `ErrUnencodableChar`/segment errors where the bitstream is malformed.

---

## 9. Testing strategy

- **Round-trip:** every encoder golden + a generated matrix (version × ECC ×
  mask × content type) encoded then decoded back, asserting equality. Cheap,
  high coverage, and it pins encoder/decoder symmetry.
- **Cross-decoder:** keep gozxing in `tools/bench` as an oracle — assert
  go-qr's output matches gozxing's on the shared corpus (don't delete the
  benchmark's gozxing import even after `verify` drops it).
- **Robustness:** the `DegradedCorpus` in `tools/bench` (rotation + noise) for
  success-rate tracking against the acceptance target.
- **Fuzz:** `FuzzDecode` over random images and over `encode→mutate-bits→decode`
  to ensure no panics and that corruption beyond ECC budget yields a clean
  error, never wrong text.
- **Benchmarks:** the existing `tools/bench` harness — add `{"native",
  go_qr.Decode}` to the registry; targets already written down.

---

## 10. Phasing

1. **M1 — fast path, clean images.** Global threshold + grid inference + format
   decode + unmask + codeword walk + RS (syndrome+BM+Chien+Forney) + segment
   decode. Round-trips all encoder goldens. Wire into `tools/bench`; hit the
   clean-path acceptance target. *This alone justifies the project: closes the
   loop and beats gozxing on the verify path.*
2. **M2 — robust path.** Adaptive binarizer + finder/alignment detection +
   perspective transform. Pass the degraded corpus target.
3. **M3 — swap verify, ship.** Rewrite `tools/verify` over `go_qr.Decode`,
   delete its gozxing dependency, add `Decode` to CLI (`-decode` flag),
   document, fuzz in CI.

Each milestone is independently shippable; M1 delivers the headline value.

---

## 11. Risks

- **Fast-path false positives.** Mitigated: RS correction validates; wrong
  guesses fail closed and fall through to robust.
- **Robust-path robustness gap vs ZXing.** Accepted by design (§1 non-goals);
  the degraded target only asks for "no regression vs gozxing − 1", and gozxing
  can remain an optional fallback if M2 underdelivers.
- **Refactor blast radius.** The shared extractions (mask, alignment, function
  map) touch encoder code; they are mechanical and covered by existing golden
  tests, so regressions surface immediately.
- **Kanji/ECI completeness.** Lower-traffic modes; can land in M1 as
  byte-mode-only with kanji/ECI following, gated by tests.
```

---

## 12. v2 changes

The v2 decoder keeps the pipeline above, with these differences:

- **API.** `Decode(img, opts...) (*DecodeResult, error)` is the single entry
  point; `DecodeDetailed` and `SegmentInfo` are gone. `DecodedSegment` carries
  the `Mode`, the ECI in effect and the raw payload, and `DecodeResult.Mirrored`
  reports mirror images.
- **Binarization.** The image is converted to luminance once, compositing
  translucent pixels over white. The fast path thresholds it at Otsu's level
  instead of mid-gray. The robust path uses a ZXing-style hybrid binarizer
  (8×8 blocks thresholded against the average black point of a 5×5 block
  neighborhood) instead of the Bradley moving average proposed in §4.1.
- **Retries.** Both paths run on the image as is and inverted, and every
  sampled grid is also decoded transposed, which is how a mirror image samples.
  A fast-path grid that fails to decode falls back to the robust path.
- **Finder selection.** Candidates are confirmed vertically and then
  horizontally. Instead of the three strongest candidates, the decoder picks the
  triple with similar module sizes that best forms a right isosceles triangle.
- **Version information.** For version 7 and up, the version blocks are read
  relative to the top-right and bottom-left finders and BCH-corrected, which
  fixes the dimension instead of trusting the finder-spacing estimate (§4.4).
- **Segments.** Kanji decodes through the encoder's table. Byte data honors
  ECI 26, 1, 3, 20, 27 and 170, and falls back to UTF-8 or ISO-8859-1 without
  an ECI. Structured append and FNC1 in first position (GS1) are decoded
  (added in v2.1); FNC1 in second position, Hanzi and other ECIs return
  `ErrUnsupported`.
- **Still out of scope.** Perspective correction through the alignment pattern
  and multiple symbols per image.
