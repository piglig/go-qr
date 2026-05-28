// Package bench holds the comparative decode benchmark harness: it pits any
// candidate QR decoder (today: gozxing; tomorrow: go-qr's native decoder)
// against a shared corpus and reports ns/op, B/op, allocs/op, and decode
// success rate.
//
// It lives in the tools submodule because it imports gozxing; the main go-qr
// library stays dependency-free. Once go_qr.Decode lands, add it to the
// `decoders` registry in decode_bench_test.go and every benchmark/accuracy
// case runs against both implementations with no further changes.
package bench

import (
	"image"
	"image/color"
	"math"
	"math/rand"

	go_qr "github.com/piglig/go-qr"
)

// Sample is one corpus entry: the ground-truth text plus a rendered image.
type Sample struct {
	Name string
	Text string
	Ecc  go_qr.Ecc
	Img  image.Image
}

// corpusInput describes a payload to encode. Lengths are chosen to land on
// small / medium / large QR versions so the benchmark spans the version range.
type corpusInput struct {
	name string
	text string
	ecc  go_qr.Ecc
}

func corpusInputs() []corpusInput {
	long := ""
	for i := 0; i < 12; i++ {
		long += "The quick brown fox jumps over the lazy dog 1234567890. "
	}
	return []corpusInput{
		{"numeric_short", "12345678", go_qr.Low},
		{"alnum_short", "HELLO WORLD 42", go_qr.Medium},
		{"url_medium", "https://github.com/piglig/go-qr?ref=bench&v=1", go_qr.Quartile},
		{"wifi_payload", "WIFI:T:WPA;S:home-network;P:s3cret-passphrase;;", go_qr.Medium},
		{"byte_long", long, go_qr.High}, // forces a high version
	}
}

// CleanCorpus renders each input to a crisp, axis-aligned image at the given
// module scale. This is the path the native decoder is expected to dominate.
func CleanCorpus(scale int) ([]Sample, error) {
	inputs := corpusInputs()
	out := make([]Sample, 0, len(inputs))
	for _, in := range inputs {
		qr, err := go_qr.EncodeText(in.text, in.ecc)
		if err != nil {
			return nil, err
		}
		img, err := qr.ToImage(go_qr.NewQrCodeImgConfig(scale, 4))
		if err != nil {
			return nil, err
		}
		out = append(out, Sample{Name: in.name, Text: in.text, Ecc: in.ecc, Img: img})
	}
	return out, nil
}

// DegradedCorpus applies a fixed, seeded set of real-world distortions
// (grayscale + additive noise + small rotation) to the clean corpus. This is
// the robustness column where ZXing-family decoders are expected to lead;
// success rate matters here more than ns/op.
func DegradedCorpus(scale int) ([]Sample, error) {
	clean, err := CleanCorpus(scale)
	if err != nil {
		return nil, err
	}
	rng := rand.New(rand.NewSource(1)) // deterministic across runs
	out := make([]Sample, 0, len(clean))
	for _, s := range clean {
		img := rotate(s.Img, 7*math.Pi/180)        // 7° skew
		img = addGaussianNoise(img, 18.0, rng)      // mild sensor noise
		out = append(out, Sample{Name: s.Name + "_degraded", Text: s.Text, Ecc: s.Ecc, Img: img})
	}
	return out, nil
}

// --- distortion helpers (stdlib only, no external deps) ---

func toGray(src image.Image) *image.Gray {
	b := src.Bounds()
	g := image.NewGray(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			g.Set(x, y, src.At(x, y))
		}
	}
	return g
}

// rotate does nearest-neighbour rotation about the image center, padding the
// exposed corners with white so the quiet zone stays light.
func rotate(src image.Image, theta float64) *image.Gray {
	g := toGray(src)
	b := g.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewGray(image.Rect(0, 0, w, h))
	cx, cy := float64(w)/2, float64(h)/2
	sin, cos := math.Sin(theta), math.Cos(theta)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy
			sx := int(math.Round(cos*dx+sin*dy + cx))
			sy := int(math.Round(-sin*dx+cos*dy + cy))
			if sx < 0 || sx >= w || sy < 0 || sy >= h {
				dst.SetGray(x, y, color.Gray{Y: 255})
				continue
			}
			dst.SetGray(x, y, g.GrayAt(sx, sy))
		}
	}
	return dst
}

func addGaussianNoise(src image.Image, sigma float64, rng *rand.Rand) *image.Gray {
	g := toGray(src)
	b := g.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			v := float64(g.GrayAt(x, y).Y) + rng.NormFloat64()*sigma
			if v < 0 {
				v = 0
			} else if v > 255 {
				v = 255
			}
			g.SetGray(x, y, color.Gray{Y: uint8(v)})
		}
	}
	return g
}
