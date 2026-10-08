package bench

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"math/rand"
	"os"
	"strings"
	"testing"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	"github.com/piglig/go-qr/v2"
)

var (
	sweep       = flag.Bool("sweep", false, "run the decoder robustness sweeps")
	sweepNative = flag.Bool("sweep-native", false, "run the sweeps for go-qr only")
	sweepOut    = flag.String("sweep-out", "", "write go-qr's decode counts per sweep point as JSON to this file")
)

// sweepSymbols spans versions with no alignment pattern (1), one (3), version
// information (7) and several alignment patterns (14).
var sweepSymbols = []string{
	"HELLO WORLD",
	"https://example.com/robustness/v3",
	strings.Repeat("https://example.com/", 6),
	strings.Repeat("The quick brown fox jumps over the lazy dog. ", 6),
}

// samplesPerPoint is the number of random images per decoder, symbol and
// sweep value.
const samplesPerPoint = 8

type sweepAxis struct {
	name   string
	values []float64
	set    func(d *Distortion, v float64)
}

// TestRobustness reports decode rates over controlled distortion sweeps for
// go-qr and gozxing. It is skipped unless -sweep is given:
//
//	go test -run=TestRobustness -sweep -v ./bench/
//
// Each row varies one parameter around a mild baseline (6 px per module,
// slight blur and noise, random in-plane rotation) and, where noted, a
// random tilt axis.
func TestRobustness(t *testing.T) {
	if !*sweep {
		t.Skip("pass -sweep to run")
	}
	axes := []sweepAxis{
		{"tilt (deg)", []float64{0, 10, 20, 30, 40, 50, 60}, func(d *Distortion, v float64) { d.Tilt = v }},
		{"px/module", []float64{1.5, 2, 2.5, 3, 4, 5}, func(d *Distortion, v float64) { d.PxPerModule = v; d.Tilt = 0 }},
		{"blur sigma", []float64{0, 0.5, 1, 1.5, 2, 2.5}, func(d *Distortion, v float64) { d.Blur = v; d.Tilt = 0 }},
		{"barrel k1", []float64{0, -0.05, -0.1, -0.15, -0.2}, func(d *Distortion, v float64) { d.K1 = v; d.Tilt = 0 }},
		{"phone mix", []float64{1}, func(d *Distortion, v float64) {}}, // random everything, see below
	}

	type impl struct {
		name   string
		decode func(image.Image) (string, error)
	}
	impls := []impl{{"go-qr", decodeNative}, {"gozxing", decodeGozxingHarder}}
	if *sweepNative {
		impls = impls[:1]
	}
	// counts holds go-qr's correct decodes per "axis=value" and its wrong
	// decodes per "axis wrong", for comparing two versions.
	counts := map[string]int{}

	srcs := make([]*image.Gray, len(sweepSymbols))
	versions := make([]int, len(sweepSymbols))
	for i, text := range sweepSymbols {
		code, err := qr.Encode(text, qr.WithECC(qr.ECCMedium))
		if err != nil {
			t.Fatal(err)
		}
		img, err := code.Image(qr.WithScale(sweepScale))
		if err != nil {
			t.Fatal(err)
		}
		srcs[i], versions[i] = grayOf(img), code.Version()
	}

	for _, ax := range axes {
		var b strings.Builder
		fmt.Fprintf(&b, "\n%-11s %-8s", ax.name, "")
		for _, v := range ax.values {
			fmt.Fprintf(&b, "%7g", v)
		}
		// results[impl][value] = passes, plus wrong decodes
		pass := make([][]int, len(impls))
		perVersion := make([][][]int, len(impls))
		wrong := make([]int, len(impls))
		for i := range impls {
			pass[i] = make([]int, len(ax.values))
			perVersion[i] = make([][]int, len(srcs))
			for s := range srcs {
				perVersion[i][s] = make([]int, len(ax.values))
			}
		}
		total := len(srcs) * samplesPerPoint
		if ax.name == "phone mix" {
			total *= 8
		}
		for vi, v := range ax.values {
			rng := rand.New(rand.NewSource(int64(1000*vi + 7)))
			for s, src := range srcs {
				n := total / len(srcs)
				for k := 0; k < n; k++ {
					d := Distortion{
						TiltAxis:    rng.Float64() * 360,
						Rotate:      rng.Float64() * 360,
						PxPerModule: 6,
						Blur:        0.6,
						Noise:       4,
					}
					ax.set(&d, v)
					if ax.name == "phone mix" {
						d.Tilt = rng.Float64() * 35
						d.PxPerModule = 3 + rng.Float64()*5
						d.Blur = 0.3 + rng.Float64()*1.0
						d.Noise = 2 + rng.Float64()*6
						d.K1 = -rng.Float64() * 0.08
					}
					img := Distort(src, sweepScale, d, rng)
					for i, im := range impls {
						got, err := im.decode(img)
						switch {
						case err == nil && got == sweepSymbols[s]:
							pass[i][vi]++
							perVersion[i][s][vi]++
						case err == nil:
							wrong[i]++
						}
					}
				}
			}
		}
		for vi, v := range ax.values {
			counts[fmt.Sprintf("%s=%g", ax.name, v)] = pass[0][vi]
		}
		counts[ax.name+" wrong"] = wrong[0]
		for i, im := range impls {
			fmt.Fprintf(&b, "\n  %-17s", im.name)
			for vi := range ax.values {
				fmt.Fprintf(&b, "%6.0f%%", 100*float64(pass[i][vi])/float64(total))
			}
			if wrong[i] > 0 {
				fmt.Fprintf(&b, "   (%d wrong decodes)", wrong[i])
			}
			for s := range srcs {
				fmt.Fprintf(&b, "\n    v%-2d %-11s", versions[s], "")
				for vi := range ax.values {
					fmt.Fprintf(&b, "%6.0f%%", 100*float64(perVersion[i][s][vi])/float64(total/len(srcs)))
				}
			}
		}
		t.Log(b.String())
	}
	if *sweepOut != "" {
		out, err := json.MarshalIndent(counts, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(*sweepOut, out, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// sweepScale is the pixels per module of the source rendering that Distort
// resamples; it must comfortably exceed the largest PxPerModule.
const sweepScale = 12

func decodeGozxingHarder(img image.Image) (string, error) {
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return "", err
	}
	hints := map[gozxing.DecodeHintType]interface{}{gozxing.DecodeHintType_TRY_HARDER: true}
	res, err := qrcode.NewQRCodeReader().Decode(bmp, hints)
	if err != nil {
		return "", err
	}
	return res.GetText(), nil
}
