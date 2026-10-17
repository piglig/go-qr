# How it works

This page describes the three pipelines in the library and the reasoning
behind decisions that affect users. It is also a map of the source for
contributors.

## Encoding

```
text ──► segments ──► bitstream ──► codewords + ECC ──► module matrix ──► mask ──► Code
```

1. **Segmentation** (`segment_optimal.go`). A dynamic program over
   (character, mode) pairs finds the sequence of numeric, alphanumeric, byte
   and Kanji segments with the fewest bits. The cost of a segment header
   depends on the width of its character count field, which changes at
   versions 10 and 27, so the split is recomputed at those boundaries while
   searching for the smallest version that fits; a range of versions that
   even the cheapest encoding of the text overflows is skipped without
   segmenting.
2. **Bitstream** (`encode.go`, `bitbuffer.go`). Segments are written with
   their headers, followed by the terminator and the alternating pad bytes
   0xEC and 0x11 up to the version's capacity. If there is room, the error
   correction level is raised: the symbol size is fixed by then, so a higher
   level is free.
3. **Error correction** (`internal/symbol/builder.go`, `internal/reedsolomon`). The data is
   split into blocks, each extended with Reed–Solomon codewords over GF(2⁸),
   and the blocks are interleaved.
4. **Layout** (`internal/symbol`: `builder.go`, `layout.go`, `template.go`). Finder, timing
   and alignment patterns and format and version information are drawn,
   then the codewords are placed in the two-column zig-zag defined by the
   standard. The zig-zag is walked once per version into a map of the data
   bit each module carries, cached with the version's other tables, so
   placing the codewords is a pass over the rows that also packs them for
   masking.
5. **Masking** (`internal/symbol`: `mask.go`, `template.go`). All eight masks are scored with
   the ISO penalty rules and the lowest wins. The modules are packed into
   64-bit words, as rows and, transposed, as columns, and the per-version
   mask patterns are cached packed, so applying a mask is an XOR of words.
   The rules are scored a word at a time: runs of five and 2×2 blocks are
   shifts, ANDs and bit counts, and so is the 1:1:3:1:1 finder-like
   pattern, since the area outside the symbol, which counts as light,
   is the zeros shifted in. Scaled versions of the pattern (2:2:6:2:2 and
   up) need a dark run of six or more after two light modules; the few
   such runs are checked one by one.

The result, `Code`, is immutable. Building it uses a separate `builder`, so
no scratch state is retained.

## Rendering

All renderers work from `Code.Module(x, y)` and the same `renderConfig`
(`render.go`).

- **Plain PNG** (`render_png.go`) is written straight from the modules as a
  1-bit paletted PNG: each module row becomes one image row, repeated
  `scale` times into a pooled zlib compressor, without building an image
  first. The bytes are those `image/png` would write.
- **Plain SVG** (`render_svg.go`) builds a graph of the edges between dark
  and light modules and walks it into one path per region, with
  `fill-rule="evenodd"` for holes. A version 40 code becomes a few kilobytes
  instead of thousands of rectangles.
- **Styles** (`render_style.go`, `render_color.go`) rasterize each shape
  once into an anti-aliased coverage tile (4×4 supersampling), cached per
  shape and scale, and copy tiles into a coverage map. Two-color output maps
  coverage through a 256-entry palette; gradients and finder colors blend
  per pixel into RGBA.
- **Logos** (`logo.go`) are checked against the error correction budget
  before drawing; see below.

## Decoding

```
image ──► luminance ──► threshold ──► locate ──► fit grid ──► read modules ──► format/version ──► unmask ──► RS correct ──► segments
```

1. **Luminance** (`decode.go`). Pixels are converted to 8-bit luminance,
   compositing transparency over white, with fast paths for the common image
   types. The luminance of gray images and the luma plane of JPEG's YCbCr
   images are used in place.
2. **Fast path.** For crisp, axis-aligned images, an Otsu threshold
   separates dark from light, the bounding box of dark pixels gives the
   symbol, and the runs across the top-left finder give the module pitch.
3. **Robust path** (`decode_locate.go`). A ZXing-style hybrid threshold
   per 8×8 block against its neighborhood copes with shadows and low
   contrast; a block with no contrast of its own, such as the inside of a
   large module, takes the midpoint of the smallest surrounding window, in
   powers of two, that has contrast. Pixels are compared with their block's
   threshold only when read. Finder candidates are found by their 1:1:3:1:1
   runs along every row, or every second or third row of large photos,
   confirmed vertically and horizontally; triples are ranked by how well
   they form a right isosceles triangle and by how many rows confirm each
   finder relative to its size.
   Module sizes are measured along the symbol's edges, not the image axes,
   so rotation does not distort them. For version 7 and up, the version
   information blocks near the finders give the exact size.
4. **Structural fit** (`decode_fit.go`). Rays from each finder's center
   cross the edges of its three nested squares; lines fitted to the edges
   give twelve corners per finder whose module coordinates are known, and
   each finder's shape carries the local perspective. A homography is
   fitted to the 36 corners by normalized least squares. The size is the
   nearby version whose timing patterns read back best, confirmed by the
   version information from version 7. Every alignment pattern is then
   predicted, located in a small window and added to the fit; with enough
   of them, a cubic polynomial correction for lens distortion is fitted
   too, and kept only if the fixed patterns read back better with it.
   Finally the alignment patterns are located again through the fitted
   grid, and their displacements from it, interpolated bilinearly between
   neighboring patterns, correct the grid locally where the symbol bends
   or the lens distorts more than the global model follows; the
   correction is kept unless the fixed patterns read back worse. If
   the timing patterns of the best triple do not read back, the other
   finder assignments and triples are fitted and the best kept. If none
   reads back well, pairs of well-confirmed finders imply the third, which
   glare, damage or the image edge may hide: each hypothesis is fitted the
   same way, with the inferred finder's center as a weighted guess, and
   the best replaces the triples' fit only if it reads back clearly
   better. The grid is decoded once. Round finder styles, whose edges are
   not straight, fall back to a perspective transform anchored on the
   alignment pattern or the finder edges.
5. **Module reading** (`decode_gray.go`). Each module's luminance is the
   mean of nine samples over its center, bilinear for modules of three
   pixels or more, and it is dark when darker than the mean of a window of
   modules around it. The window is measured in modules, not pixels, so it
   suits any module size and follows uneven lighting. When the grid does
   not decode but its function patterns read back as a symbol's, 75% of
   their modules or more, the grid most likely sits on a symbol whose
   modules glare, blur or damage made hard to tell apart, and it is
   decoded again with soft decisions (`decode_soft.go`,
   `decode_equalize.go`, *since v2.7*). The function patterns serve as
   pilots, as in a radio receiver: their values are known, so they
   calibrate, over a window around each module, a model of the image,
   luminance = a + b·dark + c·(darkness of the neighbors), whose terms are
   the light level, the contrast that glare lowers and the blur that
   darkens modules next to dark ones. Each module is decided by the model
   given its neighbors' decisions, three times over, and its reliability
   is the likelihood ratio of its two values.
6. **Scales.** Symbols are located in a pyramid of the image, halved
   repeatedly down to 150 pixels. Texture finer than a module, such as a
   screen's pixel grid or halftone dots, averages out at the coarser
   levels, and a module that a local threshold split becomes a few pixels.
   Locating costs about the pixels of the level, and camera photos hold
   far more pixels than their codes need: on the BoofCV photos, only 7 of
   the 109 readable 8-megapixel ones need full resolution. So the search
   starts at the level nearest half a megapixel, goes coarser, then finer
   down to full resolution; it tries the levels a search from full
   resolution would, so it finds the same symbols. Modules are read at
   full resolution. A finder whose modules span fewer than 3 pixels at
   its level locates the symbol too imprecisely to read it there; before
   full resolution has been searched, the symbol is located again at full
   resolution, within a window around its finders, which costs the
   window's pixels rather than the image's.
7. **Retries.** Light symbols on dark backgrounds are searched for after
   all levels, at full resolution, unless `WithoutInverted` skips them
   (*since v2.7*), and each sampled grid is also decoded
   transposed, which is how a mirror image samples. `DecodeAll` searches
   the levels in the same order, so that it locates a symbol `Decode`
   reads in the same way. `DecodeContext` and `DecodeAllContext` check
   their context between these stages: before each level, each window
   located again at full resolution and each triple `DecodeAll` decodes.
8. **Matrix and bitstream** (`internal/symbol/matrix.go`, `decode_segments.go`). The
   format information is BCH-corrected, the mask removed, codewords read in
   the placement order, blocks de-interleaved and Reed–Solomon corrected, and
   the segments parsed, applying ECI character sets, GS1 and structured
   append headers. The codewords each block corrected are counted, and
   `DecodeResult` reports them with the unused error correction of
   ISO/IEC 15415 (*since v2.7*). In the soft-decision attempts, a block
   that hard decisions cannot correct has its 2, 4, … least reliable codewords
   erased, which a Reed–Solomon code corrects twice as many of as errors
   (generalized minimum distance decoding, Forney 1966). Erasing every
   check byte would always yield a codeword, so these attempts keep the
   misdecode protection codewords of ISO/IEC 18004 Table 9 and two more
   bytes per block unused, and must read a data segment; in 3 million
   random grids with intact function patterns, none decoded.

The decoder reuses the encoder's layout code to know which modules are
function patterns, so the two cannot disagree about the geometry.

## Grading

*Since v2.7.* `Grade` (`grade.go`) decodes by hard decisions only and has
the search record the image position of every module center the fitted
grid located, and of the quiet zone's. It re-encodes the corrected data
to know each module's intended color, then measures the ISO/IEC 15415
parameters from the image: reflectance over a disc at each center,
linearized from sRGB; the symbol contrast and the global threshold
between the extremes; each module's modulation and reflectance margin,
graded per codeword through the encoder's map of the bit each module
carries and weighed against the error correction that the codewords
below each grade would use; the finder patterns' damage; the module
pitch along both axes; and the offset of the alignment pattern centers
from the regular grid through the finders.

## Design decisions

**Optimal segmentation by default.** Mixed content, such as a URL with a
long number or Japanese text with digits, often fits a smaller version. The
cost is about 1–2% of encoding time, so it is the default, and
`WithSimpleSegmentation` opts out.

**Error correction boost.** Once the version is chosen, unused capacity is
pure padding. Filling it with error correction instead makes the code more
robust at no cost in size. `WithoutECCBoost` keeps the requested level for
callers who need it exactly.

**Logo budget per block.** Covering area is a poor measure of logo damage:
a codeword spans eight modules, so partial coverage destroys it entirely,
and each error correction block can repair only its own codewords. The
library maps every module to its codeword (cached per version), counts the
codewords under the logo in each block, and allows at most 75% of any
block's capacity. The earlier area-based rule accepted logos that could not
be decoded at all.

**Contrast checks in `Verify`.** This library's decoder reads codes that
many phone cameras do not, such as very low contrast. `Verify` therefore
also requires 40% luminance contrast for every dark color, the
ISO/IEC 15415 threshold for grade C, so passing it means more than "our
decoder can read it".

**Mask choice and other readers.** The ISO penalty counts exact 1:1:3:1:1
patterns, but ZXing-family readers accept runs up to 50% off and stop
scanning after three candidates. Rarely, under 1% of symbols, data above the
bottom-left finder is mistaken for a finder by such readers. Avoiding it
would change the mask of almost half of all symbols and slow encoding 1.3 to
2.4×, so the library keeps the standard's choice. See
[Troubleshooting](../guides/troubleshooting.md).

**No dependencies.** Image formats, compression and color handling come from
the standard library, so the module adds nothing to a user's dependency
graph. Comparisons with other libraries live in the separate `tools`
module.
