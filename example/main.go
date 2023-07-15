package main

import (
	go_qr "github.com/piglig/go-qr"
	"image"
	"image/color"
	"math"
)

func main() {

}

func doBasicDemo() {
	text := "Hello world!"
	errCorLvl := go_qr.Low
	qr, err := go_qr.EncodeText(text, errCorLvl)
	if err != nil {
		return
	}

	_, _ = text, qr
}

func toImage(qr go_qr.QrCode, scale, border, lightColor, darkColor int) *image.RGBA {
	if scale <= 0 || border < 0 {
		panic("Invalid input")
	}

	if border > (math.MaxInt/2) || int64(qr.GetSize())+int64(border)*2 > math.MaxInt/int64(scale) {
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
			_, _ = moduleY, moduleX
			//color := qr.GetModule(moduleX, moduleY)
			isDark := true
			if isDark {
				result.Set(x, y, color.RGBA{R: uint8((darkColor >> 16) & 0xFF), G: uint8((darkColor >> 8) & 0xFF), B: uint8(darkColor & 0xFF), A: 255})
			} else {
				result.Set(x, y, color.RGBA{R: uint8((lightColor >> 16) & 0xFF), G: uint8((lightColor >> 8) & 0xFF), B: uint8(lightColor & 0xFF), A: 255})
			}
		}
	}
	return result
}
