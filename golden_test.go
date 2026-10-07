package qr

import (
	"flag"
	"image/color"
	"os"
	"path/filepath"
	"testing"
)

// update controls whether golden files are overwritten with the current output.
// Run `go test -update` after intentional output changes.
var update = flag.Bool("update", false, "update golden files")

// TestGoldenSVG asserts byte-level stability of SVG output. Failures here mean
// the renderer's output changed — either a real regression or an intentional
// change that needs the golden files regenerated (`go test -update`).
func TestGoldenSVG(t *testing.T) {
	cases := []struct {
		name string
		text string
		ecc  ECC
		opts []RenderOption
	}{
		{"basic", "Hello, world!", ECCLow, nil},
		{"with_xml_header", "Hello, world!", ECCLow, []RenderOption{WithSVGXMLHeader()}},
		{"larger_payload", "WIFI:S:mYwIfI;T:WPA;P:secret_passwordt;H:false;;", ECCMedium, []RenderOption{WithScale(8), WithQuietZone(2)}},
		{"high_ecc", "The quick brown fox jumps over the lazy dog", ECCHigh, []RenderOption{WithScale(6)}},
		{"dots_circle_finders", "Hello, world!", ECCLow, []RenderOption{WithModuleShape(ModuleDot), WithFinderShape(FinderCircle)}},
		{"rounded_rounded_finders", "Hello, world!", ECCLow, []RenderOption{WithModuleShape(ModuleRounded), WithFinderShape(FinderRounded), WithScale(8)}},
		{"gradient_rounded", "Hello, world!", ECCLow, []RenderOption{
			WithGradient(color.RGBA{R: 0x1a, G: 0x23, B: 0x7e, A: 0xff}, color.RGBA{G: 0x69, B: 0x5c, A: 0xff}, 45),
			WithModuleShape(ModuleRounded), WithFinderShape(FinderRounded),
		}},
		{"finder_colors", "Hello, world!", ECCLow, []RenderOption{
			WithModuleShape(ModuleDot), WithFinderShape(FinderCircle),
			WithFinderColor(color.RGBA{R: 0xc6, G: 0x28, B: 0x28, A: 0xff}, color.Black),
		}},
		{"colors", "Hello, world!", ECCLow, []RenderOption{
			WithForeground(color.RGBA{R: 0x1a, G: 0x2b, B: 0x3c, A: 0xff}),
			WithBackground(color.Transparent),
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			qr, err := encodeText(tc.text, tc.ecc)
			assertNoError(t, err)

			got, err := qr.SVG(tc.opts...)
			assertNoError(t, err)

			path := filepath.Join("testdata", "golden", tc.name+".svg")
			if *update {
				assertNoError(t, os.WriteFile(path, got, 0644))
				return
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("missing golden file %s (run with -update to create)", path)
			}
			assertEqual(t, string(want), string(got))
		})
	}
}

// TestGoldenDeterminism renders the SVG many times and asserts identical
// output every run, guarding against map-iteration order leaking into the
// path data.
func TestGoldenDeterminism(t *testing.T) {
	qr, err := encodeText("Hello, world!", ECCLow)
	assertNoError(t, err)

	first, err := qr.SVG()
	assertNoError(t, err)
	for i := 0; i < 50; i++ {
		got, err := qr.SVG()
		assertNoError(t, err)
		assertEqual(t, first, got, "run %d differs from first run", i)
	}
}
