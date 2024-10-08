package main

import (
	go_qr "github.com/piglig/go-qr"
	"image/color"
)

func main() {
	doBasicDemo()
	doVarietyDemo()
	doSegmentDemo()
	doMaskDemo()
}

func doBasicDemo() {
	text := "Hello, world!"
	errCorLvl := go_qr.Low
	qr, err := go_qr.EncodeText(text, errCorLvl)
	if err != nil {
		return
	}
	config := go_qr.NewQrCodeImgConfig(10, 4)
	err = qr.PNG(config, "hello-world-QR.png")
	if err != nil {
		return
	}

	err = qr.SVG(config, "hello-world-QR.svg", "#FFFFFF", "#000000")
	if err != nil {
		return
	}

	err = qr.SVG(go_qr.NewQrCodeImgConfig(10, 4, go_qr.WithSVGXMLHeader(true)), "hello-world-QR-xml-header.svg", "#FFFFFF", "#000000")
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

	config := go_qr.NewQrCodeImgConfig(13, 1)
	err = qr.PNG(config, "pi-digits-QR.png")
	if err != nil {
		return
	}

	// Alphanumeric mode encoding (5.5 bits per character)
	qr, err = go_qr.EncodeText("DOLLAR-AMOUNT:$39.87 PERCENTAGE:100.00% OPERATIONS:+-*/", go_qr.High)
	if err != nil {
		return
	}

	err = qr.PNG(go_qr.NewQrCodeImgConfig(10, 2), "alphanumeric-QR.png")
	if err != nil {
		return
	}

	// Unicode text as UTF-8
	qr, err = go_qr.EncodeText("こんにちwa、世界！ αβγδ", go_qr.Quartile)
	if err != nil {
		return
	}
	err = qr.PNG(go_qr.NewQrCodeImgConfig(10, 3), "unicode-QR.png")
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

	err = qr.PNG(go_qr.NewQrCodeImgConfig(10, 3), "sqrt2-monolithic-QR.png")
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
	err = qr.PNG(go_qr.NewQrCodeImgConfig(10, 3), "sqrt2-segmented-QR.png")
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

	err = qr.PNG(go_qr.NewQrCodeImgConfig(8, 5), "phi-monolithic-QR.png")
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
	err = qr.PNG(go_qr.NewQrCodeImgConfig(8, 5), "phi-segmented-QR.png")
	if err != nil {
		return
	}

	// Illustration "Madoka": kanji, kana, Cyrillic, full-width Latin, Greek characters
	madoka := "「魔法少女まどか☆マギカ」って、　ИАИ　ｄｅｓｕ　κα？"
	qr, err = go_qr.EncodeText(madoka, go_qr.Low)
	if err != nil {
		return
	}

	config := go_qr.NewQrCodeImgConfig(9, 4)
	config.SetLight(color.RGBA{
		R: 0xFF,
		G: 0xFF,
		B: 0xE0,
		A: 0xFF,
	})
	config.SetDark(color.RGBA{
		R: 0x30,
		G: 0x30,
		B: 0x80,
		A: 0xFF,
	})
	err = qr.PNG(config, "madoka-utf8-QR.png")
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

	config = go_qr.NewQrCodeImgConfig(9, 4)
	config.SetLight(color.RGBA{
		R: 0xE0,
		G: 0xF0,
		B: 0xEF,
		A: 0xFF,
	})
	config.SetDark(color.RGBA{
		R: 0x40,
		G: 0x40,
		B: 0x40,
		A: 0xFF,
	})
	err = qr.PNG(config, "madoka-kanji-QR.png")
	if err != nil {
		return
	}
}

func doMaskDemo() {
	segs, err := go_qr.MakeSegments("https://www.github.com/piglig")
	if err != nil {
		return
	}

	qr, err := go_qr.EncodeSegments(segs, go_qr.High, go_qr.MinVersion, go_qr.MaxVersion, -1, true)
	if err != nil {
		return
	}

	config := go_qr.NewQrCodeImgConfig(9, 4)
	config.SetLight(color.RGBA{
		R: 0xE0,
		G: 0xFF,
		B: 0xE0,
		A: 0xFF,
	})
	config.SetDark(color.RGBA{
		R: 0x20,
		G: 0x60,
		B: 0x20,
		A: 0xFF,
	})
	err = qr.PNG(config, "project-piglig-automask-QR.png")
	if err != nil {
		return
	}

	qr, err = go_qr.EncodeSegments(segs, go_qr.High, go_qr.MinVersion, go_qr.MaxVersion, 3, true)
	if err != nil {
		return
	}

	config = go_qr.NewQrCodeImgConfig(8, 6)
	config.SetLight(color.RGBA{
		R: 0xFF,
		G: 0xE0,
		B: 0xE0,
		A: 0xFF,
	})
	config.SetDark(color.RGBA{
		R: 0x60,
		G: 0x20,
		B: 0x20,
		A: 0xFF,
	})
	err = qr.PNG(config, "project-piglig-mask3-QR.png")
	if err != nil {
		return
	}

	// Chinese text as UTF-8
	segs, err = go_qr.MakeSegments("維基百科（Wikipedia，聆聽i/ˌwɪkᵻˈpiːdi.ə/）是一個自由內容、公開編輯且多語言的網路百科全書協作計畫")
	if err != nil {
		return
	}

	qr, err = go_qr.EncodeSegments(segs, go_qr.Medium, go_qr.MinVersion, go_qr.MaxVersion, 0, true) // Force mask 0
	if err != nil {
		return
	}
	err = qr.PNG(go_qr.NewQrCodeImgConfig(10, 3), "unicode-mask0-QR.png")
	if err != nil {
		return
	}

	qr, err = go_qr.EncodeSegments(segs, go_qr.Medium, go_qr.MinVersion, go_qr.MaxVersion, 1, true) // Force mask 1
	if err != nil {
		return
	}
	err = qr.PNG(go_qr.NewQrCodeImgConfig(10, 3), "unicode-mask1-QR.png")
	if err != nil {
		return
	}

	qr, err = go_qr.EncodeSegments(segs, go_qr.Medium, go_qr.MinVersion, go_qr.MaxVersion, 5, true) // Force mask 5
	if err != nil {
		return
	}
	err = qr.PNG(go_qr.NewQrCodeImgConfig(10, 3), "unicode-mask5-QR.png")
	if err != nil {
		return
	}

	qr, err = go_qr.EncodeSegments(segs, go_qr.Medium, go_qr.MinVersion, go_qr.MaxVersion, 7, true) // Force mask 7
	if err != nil {
		return
	}
	err = qr.PNG(go_qr.NewQrCodeImgConfig(10, 3), "unicode-mask7-QR.png")
	if err != nil {
		return
	}
}
