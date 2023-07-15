package main

import (
	go_qr "github.com/piglig/go-qr"
	"image"
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
	return result
}
