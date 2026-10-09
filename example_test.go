package qr_test

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"strings"

	"github.com/piglig/go-qr/v2"
)

func ExampleEncode() {
	code, err := qr.Encode("Hello, world!")
	if err != nil {
		panic(err)
	}
	fmt.Println("version", code.Version(), "ecc", code.ECC(), "size", code.Size())
	// Output:
	// version 1 ecc M size 21
}

func ExampleEncode_options() {
	code, err := qr.Encode("https://example.com",
		qr.WithECC(qr.ECCHigh),
		qr.WithVersionRange(5, 10),
		qr.WithMask(3),
	)
	if err != nil {
		panic(err)
	}
	fmt.Println("version", code.Version(), "ecc", code.ECC(), "mask", code.Mask())
	// Output:
	// version 5 ecc H mask 3
}

func ExampleEncode_mixedContent() {
	// Optimal segmentation switches modes inside the text, so digits and
	// Kanji cost far fewer bits than byte mode would.
	text := "注文番号 1234567890123456"
	optimal, _ := qr.Encode(text, qr.WithECC(qr.ECCLow))
	simple, _ := qr.Encode(text, qr.WithECC(qr.ECCLow), qr.WithSimpleSegmentation())
	fmt.Println("optimal version", optimal.Version())
	fmt.Println("simple version", simple.Version())
	// Output:
	// optimal version 1
	// simple version 2
}

func ExampleEncode_tooLong() {
	_, err := qr.Encode(strings.Repeat("x", 3000), qr.WithECC(qr.ECCHigh))
	fmt.Println(errors.Is(err, qr.ErrDataTooLong))
	// Output:
	// true
}

func ExampleEncodeSegments() {
	num, _ := qr.NumericSegment("0123456789")
	alnum, _ := qr.AlphanumericSegment("ORDER-")
	kanji, _ := qr.KanjiSegment("漢字")
	code, err := qr.EncodeSegments(
		[]qr.Segment{alnum, num, qr.BytesSegment([]byte("/x")), kanji},
		qr.WithECC(qr.ECCMedium),
	)
	if err != nil {
		panic(err)
	}
	fmt.Println("version", code.Version())
	// Output:
	// version 2
}

func ExampleECISegment() {
	// Declare UTF-8 explicitly for strict ISO/IEC 18004 readers. Encode does
	// this automatically with WithUTF8ECI.
	eci, _ := qr.ECISegment(26)
	code, err := qr.EncodeSegments([]qr.Segment{eci, qr.BytesSegment([]byte("héllo"))})
	if err != nil {
		panic(err)
	}
	fmt.Println("version", code.Version())
	// Output:
	// version 1
}

func ExampleCode_Module() {
	code, _ := qr.Encode("hi", qr.WithECC(qr.ECCLow), qr.WithMask(0))
	// Print the top row of the symbol: the two top finder patterns.
	for x := 0; x < code.Size(); x++ {
		if code.Module(x, 0) {
			fmt.Print("#")
		} else {
			fmt.Print(".")
		}
	}
	fmt.Println()
	// Output:
	// #######.##.##.#######
}

func ExampleCode_PNG() {
	code, err := qr.Encode("https://example.com")
	if err != nil {
		panic(err)
	}
	png, err := code.PNG(qr.WithScale(4), qr.WithQuietZone(2))
	if err != nil {
		panic(err)
	}
	fmt.Println(len(png) > 0, string(png[1:4]))
	// Output: true PNG
}

func ExampleCode_WriteSVG() {
	code, _ := qr.Encode("hi", qr.WithECC(qr.ECCLow))
	var buf strings.Builder
	err := code.WriteSVG(&buf,
		qr.WithScale(1),
		qr.WithForeground(color.RGBA{R: 0x33, G: 0x33, B: 0x99, A: 0xff}),
		qr.WithBackground(color.Transparent),
	)
	if err != nil {
		panic(err)
	}
	fmt.Println(strings.Contains(buf.String(), `viewBox="0 0 29 29"`))
	// Output: true
}

func ExampleCode_String() {
	code, _ := qr.Encode("hi", qr.WithECC(qr.ECCLow), qr.WithMask(0))
	var buf strings.Builder
	_ = code.WriteText(&buf, qr.WithQuietZone(0))
	// Two module rows per line: 21 rows fit in 11 lines.
	fmt.Println(strings.Count(buf.String(), "\n"))
	// Output: 11
}

func ExampleBatch() {
	jobs := []qr.BatchJob{
		{Text: "first", Format: qr.FormatPNG},
		{Text: "second", Format: qr.FormatSVG, Render: []qr.RenderOption{qr.WithScale(4)}},
		{Text: strings.Repeat("x", 5000)}, // too long: fails alone
	}
	for i, r := range qr.Batch(context.Background(), jobs, 0) {
		fmt.Println(i, len(r.Data) > 0, errors.Is(r.Err, qr.ErrDataTooLong))
	}
	// Output:
	// 0 true false
	// 1 true false
	// 2 false true
}

func ExampleCode_Verify() {
	code, _ := qr.Encode("https://example.com", qr.WithECC(qr.ECCHigh))

	brand := []qr.RenderOption{
		qr.WithForeground(color.RGBA{R: 0x1a, G: 0x3c, B: 0x6e, A: 0xff}),
		qr.WithBackground(color.RGBA{R: 0xf5, G: 0xf0, B: 0xe1, A: 0xff}),
	}
	fmt.Println(code.Verify(brand...))

	pastel := []qr.RenderOption{
		qr.WithForeground(color.RGBA{R: 0x9c, G: 0xc5, B: 0xe8, A: 0xff}),
		qr.WithBackground(color.White),
	}
	fmt.Println(errors.Is(code.Verify(pastel...), qr.ErrUnreadable))
	// Output:
	// <nil>
	// true
}

func ExampleEncodeStructured() {
	text := strings.Repeat("A long message split across symbols. ", 8)
	codes, err := qr.EncodeStructured(text, qr.WithECC(qr.ECCLow), qr.WithVersionRange(1, 3))
	if err != nil {
		panic(err)
	}
	results := make([]*qr.DecodeResult, len(codes))
	for i, c := range codes {
		img, _ := c.Image()
		results[i], _ = qr.Decode(img)
	}
	joined, _ := qr.JoinStructuredAppend(results...)
	fmt.Println(len(codes), "symbols, intact:", joined == text)
	// Output: 6 symbols, intact: true
}

func ExampleWithGradient() {
	code, _ := qr.Encode("https://example.com", qr.WithECC(qr.ECCQuartile))
	opts := []qr.RenderOption{
		qr.WithModuleShape(qr.ModuleRounded),
		qr.WithFinderShape(qr.FinderRounded),
		qr.WithGradient(color.RGBA{R: 0x1a, G: 0x23, B: 0x7e, A: 0xff}, color.RGBA{G: 0x69, B: 0x5c, A: 0xff}, 45),
	}
	// Check the style before publishing it.
	if err := code.Verify(opts...); err != nil {
		panic(err)
	}
	svg, _ := code.SVG(opts...)
	fmt.Println(strings.Contains(string(svg), "<linearGradient"))
	// Output: true
}

func ExampleDecode() {
	code, _ := qr.Encode("decode me", qr.WithECC(qr.ECCQuartile))
	img, _ := code.Image(qr.WithModuleShape(qr.ModuleDot))

	res, err := qr.Decode(img)
	if err != nil {
		panic(err)
	}
	fmt.Println(res.Text, res.Version, res.ECC)
	// Output: decode me 1 Q
}

// Corners locate the symbol in the image, for cropping or drawing an
// outline, in the symbol's own orientation.
func ExampleDecodeResult_corners() {
	code, _ := qr.Encode("where am I")
	img, _ := code.Image(qr.WithScale(4)) // 4 pixels per module, 4-module quiet zone

	res, err := qr.Decode(img)
	if err != nil {
		panic(err)
	}
	fmt.Println(res.Corners, res.Inverted)
	// Output: [(16,16) (100,16) (100,100) (16,100)] false
}

func ExampleDecodeAll() {
	sheet := image.NewRGBA(image.Rect(0, 0, 400, 200))
	draw.Draw(sheet, sheet.Bounds(), image.White, image.Point{}, draw.Src)
	for i, text := range []string{"left", "right"} {
		code, _ := qr.Encode(text)
		img, _ := code.Image(qr.WithScale(5))
		draw.Draw(sheet, img.Bounds().Add(image.Pt(i*200, 0)), img, image.Point{}, draw.Src)
	}

	results, err := qr.DecodeAll(sheet)
	if err != nil {
		panic(err)
	}
	for _, res := range results {
		fmt.Println(res.Text, res.Corners[0])
	}
	// Output:
	// left (20,20)
	// right (220,20)
}
