package bench

import (
	"fmt"
	"image"
	"testing"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	go_qr "github.com/piglig/go-qr"
)

// decoderImpl is a candidate QR decoder under comparison.
type decoderImpl struct {
	name   string
	decode func(image.Image) (string, error)
}

// decoders is the registry every benchmark and accuracy case iterates over.
//
// To add the native decoder once it exists, append:
//
//	{"native", go_qr.Decode},
//
// Nothing else needs to change — baselines, allocs, and success-rate reports
// will all pick it up automatically.
var decoders = []decoderImpl{
	{"gozxing", decodeGozxing},
	{"native", go_qr.Decode},
}

func decodeGozxing(img image.Image) (string, error) {
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return "", err
	}
	res, err := qrcode.NewQRCodeReader().Decode(bmp, nil)
	if err != nil {
		return "", err
	}
	return res.GetText(), nil
}

const benchScale = 8

// BenchmarkDecodeClean measures throughput and allocations on crisp,
// self-generated images. Run with:
//
//	go test -run=^$ -bench=BenchmarkDecodeClean -benchmem ./...
func BenchmarkDecodeClean(b *testing.B) {
	corpus, err := CleanCorpus(benchScale)
	if err != nil {
		b.Fatal(err)
	}
	for _, d := range decoders {
		for _, s := range corpus {
			b.Run(fmt.Sprintf("%s/%s", d.name, s.Name), func(b *testing.B) {
				// sanity: fail loudly if the baseline can't even read it
				if got, err := d.decode(s.Img); err != nil || got != s.Text {
					b.Fatalf("%s precheck failed: got %q err %v", d.name, got, err)
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_, _ = d.decode(s.Img)
				}
			})
		}
	}
}

// TestDecodeAccuracy reports per-decoder success rate over clean and degraded
// corpora. It is a Test, not a Benchmark, because we care about the
// pass/total ratio, not latency. It does not fail on misses — it tabulates so
// candidates can be compared on robustness.
func TestDecodeAccuracy(t *testing.T) {
	clean, err := CleanCorpus(benchScale)
	if err != nil {
		t.Fatal(err)
	}
	degraded, err := DegradedCorpus(benchScale)
	if err != nil {
		t.Fatal(err)
	}
	corpora := map[string][]Sample{"clean": clean, "degraded": degraded}

	for _, d := range decoders {
		for name, corpus := range corpora {
			pass := 0
			for _, s := range corpus {
				if got, err := d.decode(s.Img); err == nil && got == s.Text {
					pass++
				}
			}
			t.Logf("accuracy %-8s %-9s %d/%d (%.0f%%)",
				d.name, name, pass, len(corpus),
				100*float64(pass)/float64(len(corpus)))
		}
	}
}
