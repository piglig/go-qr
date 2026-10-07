package qr_test

import (
	"errors"
	"fmt"
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
