# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
