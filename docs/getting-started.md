# Getting started

This tutorial builds a small program that creates a branded QR Code for a
link, checks that it scans, saves it as PNG and SVG, and reads it back. It
takes about ten minutes. You need Go 1.23 or later.

## 1. Create a module

```shell
mkdir qrdemo && cd qrdemo
go mod init example.com/qrdemo
go get github.com/piglig/go-qr/v2
```

## 2. Encode some text

Create `main.go`:

```go
package main

import (
	"fmt"
	"log"

	"github.com/piglig/go-qr/v2"
)

func main() {
	code, err := qr.Encode("https://go.dev")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("version", code.Version(), "error correction", code.ECC())
	fmt.Print(code)
}
```

Run it with `go run .`. It prints the symbol version, the error correction
level, and the code itself drawn with block characters, which you can scan
from the terminal with a phone.

`Encode` made every decision for you: the smallest version that fits, the
most compact encoding modes, and error correction raised from the default
level M as far as the data still fits. The [encoding guide](guides/encoding.md)
shows how to control each of these.

## 3. Save it as an image

Replace the body of `main` after the error check:

```go
	png, err := code.PNG(qr.WithScale(8))
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("qr.png", png, 0o644); err != nil {
		log.Fatal(err)
	}
```

and add `"os"` to the imports. `WithScale(8)` draws each module as 8×8
pixels; the default quiet zone of four modules is included.
`code.SVG()` works the same way and returns a compact vector image.

## 4. Give it a style

Rendering options control the look. Use rounded modules and finders, and a
gradient from navy to teal:

```go
	navy := color.RGBA{R: 0x1a, G: 0x23, B: 0x7e, A: 0xff}
	teal := color.RGBA{G: 0x69, B: 0x5c, A: 0xff}
	style := []qr.RenderOption{
		qr.WithScale(8),
		qr.WithModuleShape(qr.ModuleRounded),
		qr.WithFinderShape(qr.FinderRounded),
		qr.WithGradient(navy, teal, 45),
	}
	png, err := code.PNG(style...)
```

(add `"image/color"` to the imports). Shapes and gradients are *Since v2.2*;
the [styling guide](guides/styling.md) lists every option.

## 5. Check that it scans

A style can make a code hard to read, for example a gradient that fades to a
light color. Before saving, ask the library to render the code with your
options, decode it, and check the colors:

```go
	if err := code.Verify(style...); err != nil {
		log.Fatal("this style is not safe to publish: ", err)
	}
```

Try changing `teal` to a light color such as `color.RGBA{R: 0xb2, G: 0xdf,
B: 0xdb, A: 0xff}` and run the program again: `Verify` now reports that the
contrast is too low. (*Since v2.1*.)

## 6. Read it back

The library includes a decoder. Read the image you just wrote:

```go
	f, err := os.Open("qr.png")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		log.Fatal(err)
	}
	res, err := qr.Decode(img)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("decoded:", res.Text)
```

This needs the `image/png` import; rename the `png` variable from step 3 to
`data` so the names do not clash.

## The complete program

```go
package main

import (
	"fmt"
	"image/color"
	"image/png"
	"log"
	"os"

	"github.com/piglig/go-qr/v2"
)

func main() {
	code, err := qr.Encode("https://go.dev")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("version", code.Version(), "error correction", code.ECC())

	navy := color.RGBA{R: 0x1a, G: 0x23, B: 0x7e, A: 0xff}
	teal := color.RGBA{G: 0x69, B: 0x5c, A: 0xff}
	style := []qr.RenderOption{
		qr.WithScale(8),
		qr.WithModuleShape(qr.ModuleRounded),
		qr.WithFinderShape(qr.FinderRounded),
		qr.WithGradient(navy, teal, 45),
	}
	if err := code.Verify(style...); err != nil {
		log.Fatal("this style is not safe to publish: ", err)
	}

	data, err := code.PNG(style...)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("qr.png", data, 0o644); err != nil {
		log.Fatal(err)
	}
	svg, err := code.SVG(style...)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile("qr.svg", svg, 0o644); err != nil {
		log.Fatal(err)
	}

	f, err := os.Open("qr.png")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		log.Fatal(err)
	}
	res, err := qr.Decode(img)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("decoded:", res.Text)
}
```

## Next steps

- Put structured data in the code, such as Wi-Fi credentials or a contact
  card, with the [payload package](guides/payloads.md).
- Add a logo with `WithLogo`: see [Rendering](guides/rendering.md#logos).
- Try every option interactively in the
  [playground](https://piglig.github.io/go-qr/), which also shows the Go code
  for the settings you choose.
