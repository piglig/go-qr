# go-qr
[![Mentioned in Awesome Go](https://awesome.re/mentioned-badge.svg)](https://github.com/avelino/awesome-go#utilities)
[![Go Report Card](https://goreportcard.com/badge/github.com/piglig/go-qr)](https://goreportcard.com/report/github.com/piglig/go-qr)
[![Build Status](https://github.com/piglig/go-qr/actions/workflows/go.yml/badge.svg?branch=main)](https://github.com/piglig/go-qr/actions/workflows/go.yml?query=branch%3Amain)
[![Codecov](https://img.shields.io/codecov/c/github/piglig/go-qr)](https://app.codecov.io/github/piglig/go-qr)
[![GoDoc](https://godoc.org/github.com/piglig/go-qr?status.svg)](https://pkg.go.dev/github.com/piglig/go-qr)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](https://github.com/piglig/go-qr/blob/main/LICENSE)

> 🎶 Go Community Minimalist QR Code Generator Library.

## Overview
This library is native, high quality and minimalistic. Generate QR code from string text
 
It is mostly a translation of [project Nayuki's Java version of the QR code generator](https://www.nayuki.io/page/qr-code-generator-library).

## Features
* Minimalist native code implementation
* Based on QR Code Model 2 standard, supports all 40 versions and all 4 error correction levels
* Output format: Raw modules/pixels of the QR symbol
* Detects finder-like penalty patterns more accurately than other implementations
* Encoding space optimisation for numeric and special alphanumeric texts
* Japanese Unicode Text Encoding Optimisation
* For mixed numeric/alphanumeric/general/kanji text, computes optimal segment mode switching
* Good test coverage
* MIT's Open Source License

## Installation
```go
go get github.com/piglig/go-qr
```

## [Examples](https://github.com/piglig/go-qr/tree/master/example/main.go)
```go
package main

import (
	"errors"
	"fmt"
	go_qr "github.com/piglig/go-qr"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"strings"
)

func main() {
	doBasicDemo()
}

func doBasicDemo() {
	text := "Hello, world!"
	errCorLvl := go_qr.Low
	qr, err := go_qr.EncodeText(text, errCorLvl)
	if err != nil {
		return
	}
	img := toImageStandard(qr, 10, 4)
	err = writePng(img, "hello-world-QR.png")
	if err != nil {
		return
	}

	svg, err := toSvgString(qr, 4, "#FFFFFF", "#000000")
	if err != nil {
		return
	}

	svgFile, err := os.Create("hello-world-QR.svg")
	if err != nil {
		return
	}
	defer svgFile.Close()
	_, err = svgFile.WriteString(svg)
	if err != nil {
		return
	}
}

func toImageStandard(qr *go_qr.QrCode, scale, border int) *image.RGBA {
	return toImage(qr, scale, border, color.White, color.Black)
}

func toImage(qr *go_qr.QrCode, scale, border int, lightColor, darkColor color.Color) *image.RGBA {
	if scale <= 0 || border < 0 || qr == nil {
		panic("Invalid input")
	}

	if border > (math.MaxInt32/2) || int64(qr.GetSize())+int64(border)*2 > math.MaxInt32/int64(scale) {
		panic("Scale or border too large")
	}

	size := qr.GetSize() + border*2
	imageWidth := size * scale
	imageHeight := size * scale
	result := image.NewRGBA(image.Rect(0, 0, imageWidth, imageHeight))
	for y := 0; y < imageHeight; y++ {
		for x := 0; x < imageWidth; x++ {
			moduleX := x/scale - border
			moduleY := y/scale - border
			isDark := qr.GetModule(moduleX, moduleY)
			if isDark {
				result.Set(x, y, darkColor)
			} else {
				result.Set(x, y, lightColor)
			}
		}
	}
	return result
}

func writePng(img *image.RGBA, filepath string) error {
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	err = png.Encode(file, img)
	if err != nil {
		return err
	}

	return nil
}

func toSvgString(qr *go_qr.QrCode, border int, lightColor, darkColor string) (string, error) {
	if border < 0 {
		return "", errors.New("border must be non-negative")
	}

	if qr == nil {
		return "", errors.New("qr is nil")
	}

	var brd = int64(border)
	sb := strings.Builder{}
	sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	sb.WriteString("<!DOCTYPE svg PUBLIC \"-//W3C//DTD SVG 1.1//EN\" \"http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd\">\n")
	sb.WriteString(fmt.Sprintf("<svg xmlns=\"http://www.w3.org/2000/svg\" version=\"1.1\" viewBox=\"0 0 %d %d\" stroke=\"none\">\n",
		int64(qr.GetSize())+brd*2, int64(qr.GetSize())+brd*2))
	sb.WriteString("\t<rect width=\"100%\" height=\"100%\" fill=\"" + lightColor + "\"/>\n")
	sb.WriteString("\t<path d=\"")

	for y := 0; y < qr.GetSize(); y++ {
		for x := 0; x < qr.GetSize(); x++ {
			if qr.GetModule(x, y) {
				if x != 0 || y != 0 {
					sb.WriteString(" ")
				}
				sb.WriteString(fmt.Sprintf("M%d,%dh1v1h-1z", int64(x)+brd, int64(y)+brd))
			}
		}
	}
	sb.WriteString("\" fill=\"" + darkColor + "\"/>\n")
	sb.WriteString("</svg>\n")

	return sb.String(), nil
}

```

## Command Line Tool
**generator** command line tool to generate the QR Code.
### Installation
In order to use the tool, compile it using the following command
```shell
go install github.com/piglig/go-qr/tools/generator@latest
```

### Usage
```shell
generator [options] [arguments]
  -content string
        Content to encode in the QR code
  -png string
        Output PNG file name
  -svg string
        Output SVG file name
```

### Example
* **Text Art**
```shell
generator -content hello
```
![Gif](/docs/assets/text_art.gif)

* **Image Type**
```shell
generator -content hello -png hello.png -svg hello.svg
```
![Gif](/docs/assets/image_type.gif)

## License
See the [LICENSE](LICENSE) file for license rights and limitations (MIT).