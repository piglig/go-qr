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
	doVarietyDemo()
	doSegmentDemo()
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

func doVarietyDemo() {
	// Numeric mode encoding (3.33 bits per digit)
	qr, err := go_qr.EncodeText("314159265358979323846264338327950288419716939937510", go_qr.Medium)
	if err != nil {
		return
	}
	err = writePng(toImageStandard(qr, 13, 1), "pi-digits-QR.png")
	if err != nil {
		return
	}

	// Alphanumeric mode encoding (5.5 bits per character)
	qr, err = go_qr.EncodeText("DOLLAR-AMOUNT:$39.87 PERCENTAGE:100.00% OPERATIONS:+-*/", go_qr.High)
	if err != nil {
		return
	}
	err = writePng(toImageStandard(qr, 10, 2), "alphanumeric-QR.png")
	if err != nil {
		return
	}

	// Unicode text as UTF-8
	qr, err = go_qr.EncodeText("こんにちwa、世界！ αβγδ", go_qr.Quartile)
	if err != nil {
		return
	}
	err = writePng(toImageStandard(qr, 10, 3), "unicode-QR.png")
	if err != nil {
		return
	}
}

func doSegmentDemo() {
	// Illustration "silver"
	silver0 := "THE SQUARE ROOT OF 2 IS 1."
	silver1 := "41421356237309504880168872420969807856967187537694807317667973799"
	qr, err := go_qr.EncodeText(silver0+silver1, go_qr.Low)
	if err != nil {
		return
	}
	err = writePng(toImageStandard(qr, 10, 3), "sqrt2-monolithic-QR.png")
	if err != nil {
		return
	}

	seg1, err := go_qr.MakeAlphanumeric(silver0)
	if err != nil {
		return
	}

	seg2, err := go_qr.MakeNumeric(silver1)
	if err != nil {
		return
	}

	segs := []*go_qr.QrSegment{seg1, seg2}
	qr, err = go_qr.EncodeStandardSegments(segs, go_qr.Low)
	if err != nil {
		return
	}
	err = writePng(toImageStandard(qr, 10, 3), "sqrt2-segmented-QR.png")
	if err != nil {
		return
	}

	golden0 := "Golden ratio φ = 1."
	golden1 := "6180339887498948482045868343656381177203091798057628621354486227052604628189024497072072041893911374"
	golden2 := "......"
	qr, err = go_qr.EncodeText(golden0+golden1+golden2, go_qr.Low)
	if err != nil {
		return
	}
	err = writePng(toImageStandard(qr, 8, 5), "phi-monolithic-QR.png")
	if err != nil {
		return
	}

	goldenSeg1, err := go_qr.MakeBytes([]byte(golden0))
	if err != nil {
		return
	}

	goldenSeg2, err := go_qr.MakeNumeric(golden1)
	if err != nil {
		return
	}

	goldenSeg3, err := go_qr.MakeAlphanumeric(golden2)
	if err != nil {
		return
	}

	segs = []*go_qr.QrSegment{goldenSeg1, goldenSeg2, goldenSeg3}
	qr, err = go_qr.EncodeStandardSegments(segs, go_qr.Low)
	if err != nil {
		return
	}
	err = writePng(toImageStandard(qr, 8, 5), "phi-segmented-QR.png")
	if err != nil {
		return
	}

	// Illustration "Madoka": kanji, kana, Cyrillic, full-width Latin, Greek characters
	madoka := "「魔法少女まどか☆マギカ」って、　ИАИ　ｄｅｓｕ　κα？"
	qr, err = go_qr.EncodeText(madoka, go_qr.Low)
	if err != nil {
		return
	}
	err = writePng(toImage(qr, 9, 4, color.RGBA{
		R: 0xFF,
		G: 0xFF,
		B: 0xE0,
		A: 0xFF,
	}, color.RGBA{
		R: 0x30,
		G: 0x30,
		B: 0x80,
		A: 0xFF,
	}), "madoka-utf8-QR.png")
	if err != nil {
		return
	}

	madokaSeg, err := go_qr.MakeKanji(madoka)
	if err != nil {
		return
	}

	segs = []*go_qr.QrSegment{madokaSeg}
	qr, err = go_qr.EncodeStandardSegments(segs, go_qr.Low)
	if err != nil {
		return
	}
	err = writePng(toImage(qr, 9, 4, color.RGBA{
		R: 0xE0,
		G: 0xF0,
		B: 0xEF,
		A: 0xFF,
	}, color.RGBA{
		R: 0x40,
		G: 0x40,
		B: 0x40,
		A: 0xFF,
	}), "madoka-kanji-QR.png")
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
