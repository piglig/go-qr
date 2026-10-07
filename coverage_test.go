package qr

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

// Covers option.go: WithLight and WithDark mutate the config's color fields.
func TestWithLightAndWithDark(t *testing.T) {
	cfg := NewQrCodeImgConfig(10, 4,
		WithLight(color.RGBA{R: 10, G: 20, B: 30, A: 255}),
		WithDark(color.RGBA{R: 40, G: 50, B: 60, A: 255}),
	)
	assertEqual(t, color.RGBA{R: 10, G: 20, B: 30, A: 255}, cfg.Light())
	assertEqual(t, color.RGBA{R: 40, G: 50, B: 60, A: 255}, cfg.Dark())
}

// Covers encode.go: EncodeBytes, including empty input.
func TestEncodeBytes(t *testing.T) {
	qr, err := EncodeBytes([]byte("hello binary"), WithECC(ECCLow))
	assertNoError(t, err)
	assertNotNil(t, qr)
	assertGreater(t, qr.Size(), 0)

	qr, err = EncodeBytes(nil)
	assertNoError(t, err)
	assertEqual(t, 1, qr.Version())
}

// Covers color.go: translucent path and fully-transparent path.
func TestColorToSVGHexAndTransparent(t *testing.T) {
	// Opaque → #RRGGBB
	assertEqual(t, "#1A2B3C", colorToSVGHex(color.RGBA{R: 0x1a, G: 0x2b, B: 0x3c, A: 0xff}))
	// Translucent → rgba(...)
	out := colorToSVGHex(color.RGBA{R: 10, G: 20, B: 30, A: 128})
	assertTrue(t, strings.HasPrefix(out, "rgba("), "got %q", out)
	// Transparent detection
	assertTrue(t, colorIsTransparent(color.RGBA{}))
	assertFalse(t, colorIsTransparent(color.Black))
}

// Covers batch.go: default config branch, invalid format error, nil cfg path.
func TestRenderBatchInvalidFormatAndDefaults(t *testing.T) {
	jobs := []BatchJob{
		{Text: "ok", ECC: ECCLow, Format: FormatSVG},   // nil config → default
		{Text: "bad", ECC: ECCLow, Format: Format(99)}, // invalid format
		{Text: "ok", ECC: ECCLow, Format: FormatPNG, Config: NewQrCodeImgConfig(4, 2)},
	}
	results := RenderBatch(jobs, 2)
	assertLen(t, results, 3)
	assertNoError(t, results[0].Err)
	assertNotEmpty(t, results[0].Bytes)
	assertError(t, results[1].Err)
	assertContains(t, results[1].Err.Error(), "invalid batch format")
	assertNoError(t, results[2].Err)
	assertNotEmpty(t, results[2].Bytes)
}

// Covers batch.go runWorkers fast paths.
func TestRunWorkersEdgeCases(t *testing.T) {
	// n == 0 → early return
	runWorkers(0, 4, func(int) { t.Fatal("should not run") })
	// concurrency == 1 → synchronous loop
	var count int
	runWorkers(3, 1, func(int) { count++ })
	assertEqual(t, 3, count)
}

// Covers logo.go eccRecoveryBudget every branch including default.
func TestEccRecoveryBudgetAllBranches(t *testing.T) {
	assertInDelta(t, 0.05, eccRecoveryBudget(ECCLow), 1e-9)
	assertInDelta(t, 0.12, eccRecoveryBudget(ECCMedium), 1e-9)
	assertInDelta(t, 0.20, eccRecoveryBudget(ECCQuartile), 1e-9)
	assertInDelta(t, 0.25, eccRecoveryBudget(ECCHigh), 1e-9)
	assertInDelta(t, 0.05, eccRecoveryBudget(ECC(99)), 1e-9) // default
}

// Covers logo.go logoRect error paths: bad sizeRatio, nil image, oversize.
func TestLogoRectErrors(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))

	// Bad ratios
	_, _, err := (&logoConfig{img: img, sizeRatio: 0}).logoRect(25, 10, 4)
	assertError(t, err)
	_, _, err = (&logoConfig{img: img, sizeRatio: 1.5}).logoRect(25, 10, 4)
	assertError(t, err)

	// Nil image
	_, _, err = (&logoConfig{img: nil, sizeRatio: 0.2}).logoRect(25, 10, 4)
	assertError(t, err)

	// Ratio too large → boxModules >= qrSize
	_, _, err = (&logoConfig{img: img, sizeRatio: 0.95}).logoRect(25, 10, 4)
	assertError(t, err)
	assertContains(t, err.Error(), "too large")
}

// Covers render_svg.go injectSVGFragment fallback when </svg> is missing.
func TestInjectSVGFragmentNoClosingTag(t *testing.T) {
	out := injectSVGFragment("<svg>", "<g/>")
	assertEqual(t, "<svg><g/>", out)

	out = injectSVGFragment("<svg></svg>", "<g/>")
	assertEqual(t, "<svg><g/></svg>", out)
}

// Covers logo.go validate → ratio exceeds ECC budget.
func TestLogoValidateExceedsBudget(t *testing.T) {
	// ECCLow ECC budget is 5%. A logo with sizeRatio 0.5 occupies ~25%+ of modules.
	img := image.NewRGBA(image.Rect(0, 0, 20, 20))
	qr, err := encodeText("hi", ECCLow)
	assertNoError(t, err)
	logo := &logoConfig{img: img, sizeRatio: 0.5}
	assertError(t, logo.validate(qr, 10, 4))
}

// Covers logo.go svgEmbed happy path, producing <rect> + <image> fragment.
func TestLogoSVGEmbed(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	logo := &logoConfig{img: img, sizeRatio: 0.2}
	frag, err := logo.svgEmbed(25, 10, 4)
	assertNoError(t, err)
	assertContains(t, frag, "<rect ")
	assertContains(t, frag, "<image ")
	assertContains(t, frag, "data:image/png;base64,")
}

// Covers logo.go overlayOnImage happy path.
func TestLogoOverlayOnImage(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3, 3))
	src.Set(1, 1, color.RGBA{R: 255, A: 255})

	// End-to-end PNG render with logo to exercise overlayOnImage + validate.
	var buf bytes.Buffer
	assertNoError(t, png.Encode(&buf, src))
	qr, err := encodeText("hello", ECCHigh)
	assertNoError(t, err)
	cfg := NewQrCodeImgConfig(10, 4, WithLogo(src, 0.2))
	out, err := qr.ToPNGBytes(cfg)
	assertNoError(t, err)
	assertNotEmpty(t, out)
}
