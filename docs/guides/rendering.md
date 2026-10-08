# Rendering

A `*qr.Code` renders to PNG, SVG, an `*image.RGBA` or text. Every method
takes the same `RenderOption` values:

```go
png, err := code.PNG(qr.WithScale(8), qr.WithQuietZone(2))
err = code.WriteSVG(w, qr.WithForeground(navy), qr.WithBackground(color.Transparent))
```

## Output formats

| Method | Output | Notes |
| --- | --- | --- |
| `PNG(opts...)` / `WritePNG(w, opts...)` | PNG | 1-bit paletted for plain codes, 8-bit paletted for two-color styles, RGBA with gradients, finder colors or a logo. |
| `SVG(opts...)` / `WriteSVG(w, opts...)` | SVG | Plain codes are one `fill-rule="evenodd"` path tracing each dark region, so files stay small. |
| `Image(opts...)` | `*image.RGBA` | For drawing on top of the code or encoding to another format. |
| `String()` / `WriteText(w, opts...)` | Unicode text | Two module rows per line with `█▀▄`; dark on light. Only `WithQuietZone` applies. |

The `Write*` methods stream to any `io.Writer`, such as an
`http.ResponseWriter`:

```go
func handler(w http.ResponseWriter, r *http.Request) {
	code, err := qr.Encode(r.URL.Query().Get("text"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	code.WriteSVG(w)
}
```

## Options

| Option | Default | Effect |
| --- | --- | --- |
| `WithScale(n)` | 10 | Side of a module in pixels (PNG, Image) or user units (SVG). |
| `WithQuietZone(n)` | 4 | Light margin around the symbol, in modules. The standard requires 4; many readers manage with 2. |
| `WithForeground(c)` | black | Color of dark modules. |
| `WithBackground(c)` | white | Color of light modules and the quiet zone. A fully transparent color omits the SVG background. |
| `WithLogo(img, ratio)` | none | A centered logo; see [Logos](#logos). |
| `WithSVGXMLHeader()` | off | Adds the XML declaration and DOCTYPE to SVG output. |

Shapes, finder colors and gradients are covered in [Styling](styling.md).

Readers need strong contrast: keep the foreground dark and the background
light. Light-on-dark codes are readable by this library's decoder and some
apps, but many scanners reject them.

## Logos

`WithLogo(img, ratio)` draws `img` over the center of the symbol, scaled to
`ratio` of the symbol's side and surrounded by a one-module pad in the
background color:

```go
code, _ := qr.Encode("https://example.com", qr.WithECC(qr.ECCHigh))
png, err := code.PNG(qr.WithLogo(logo, 0.2))
```

The modules under a logo are lost, and error correction has to restore them.
Before drawing, the library counts which codewords the logo covers in each
error correction block, and fails with `ErrLogoTooLarge` if any block would
need more than 75% of its correction capacity. The remaining 25% is left for
print defects, glare and blur.

In practice:

- Encode with `qr.WithECC(qr.ECCHigh)`. A ratio of 0.2 fits every version at
  level H.
- At lower levels only small logos fit, and at level L hardly any.
- Prefer simple, high-contrast logos with some transparent margin.

## Checking readability

*Since v2.1.* `Verify` renders the code exactly as `Image` would with the
same options, decodes the result, and checks that it carries the code's data.
It also rejects colors that phone cameras struggle with even when this
library's tolerant decoder succeeds: any dark color that is lighter than the
background or has less than 40% luminance contrast with it.

```go
opts := []qr.RenderOption{qr.WithForeground(brand), qr.WithLogo(logo, 0.2)}
if err := code.Verify(opts...); err != nil {
	// errors.Is(err, qr.ErrUnreadable) or errors.Is(err, qr.ErrLogoTooLarge)
	return err
}
png, err := code.PNG(opts...)
```

Call it whenever colors, styles or logos come from users or designers.
`Verify` checks the raster rendering; SVG output with the same options draws
the same modules.

## Rendering cost

Plain PNGs are the fastest raster output and SVG is cheap in every style.
Styles, gradients and logos cost more; the [performance](../explanation/performance.md)
page has numbers. For many codes, see [Batch processing](batch.md).
