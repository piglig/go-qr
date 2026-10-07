package qr

import (
	"flag"
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
		name   string
		text   string
		ecl    ECC
		config *QrCodeImgConfig
	}{
		{
			name:   "basic",
			text:   "Hello, world!",
			ecl:    ECCLow,
			config: NewQrCodeImgConfig(10, 4),
		},
		{
			name:   "with_xml_header",
			text:   "Hello, world!",
			ecl:    ECCLow,
			config: NewQrCodeImgConfig(10, 4, WithSVGXMLHeader()),
		},
		{
			name:   "optimal",
			text:   "Hello, world!",
			ecl:    ECCLow,
			config: NewQrCodeImgConfig(10, 4, WithOptimalSVG()),
		},
		{
			name:   "optimal_larger_payload",
			text:   "WIFI:S:mYwIfI;T:WPA;P:secret_passwordt;H:false;;",
			ecl:    ECCMedium,
			config: NewQrCodeImgConfig(8, 2, WithOptimalSVG()),
		},
		{
			name:   "optimal_high_ecc",
			text:   "The quick brown fox jumps over the lazy dog",
			ecl:    ECCHigh,
			config: NewQrCodeImgConfig(6, 4, WithOptimalSVG()),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			qr, err := encodeText(tc.text, tc.ecl)
			assertNoError(t, err)

			got, err := qr.ToSVGBytes(tc.config)
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

// TestGoldenDeterminism runs the optimized SVG generator many times and asserts
// identical output every run. This specifically guards against the map-iteration
// non-determinism that broke position-detection markers in earlier versions.
func TestGoldenDeterminism(t *testing.T) {
	qr, err := encodeText("Hello, world!", ECCLow)
	assertNoError(t, err)
	cfg := NewQrCodeImgConfig(10, 4, WithOptimalSVG())

	first, err := qr.ToSVGBytes(cfg)
	assertNoError(t, err)

	for i := 0; i < 50; i++ {
		got, err := qr.ToSVGBytes(cfg)
		assertNoError(t, err)
		assertEqual(t, first, got, "run %d differs from first run", i)
	}
}
