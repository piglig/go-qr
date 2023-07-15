package main

import (
	go_qr "github.com/piglig/go-qr"
	"image"
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

	if border > (1<<31-1)/2 || int64(11)+int64(border)*2 > (1<<31-1)/int64(scale) {
		panic("Scale or border too large")
	}

	return nil
}
