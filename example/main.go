package main

import (
	go_qr "github.com/piglig/go-qr"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
)

func main() {
	doBasicDemo()
}

func doBasicDemo() {
	text := "Hello world!"
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
}

func toImageStandard(qr *go_qr.QrCode, scale, border int) *image.RGBA {
	return toImage(qr, scale, border, color.White, color.Black)
}

func toImage(qr *go_qr.QrCode, scale, border int, lightColor, darkColor color.Color) *image.RGBA {
	if scale <= 0 || border < 0 || qr == nil {
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
			isDark := qr.GetModule(moduleX, moduleY)
			if isDark {
				result.Set(x, y, color.Black)
			} else {
				result.Set(x, y, color.White)
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
