package bench

import (
	"fmt"
	"strings"
	"testing"

	"github.com/boombuler/barcode/qr"
	go_qr "github.com/piglig/go-qr"
	skip2 "github.com/skip2/go-qrcode"
)

// encodeCase is one payload encoded by every candidate library. Lengths span
// small / medium / large QR versions.
type encodeCase struct {
	name string
	text string
}

func encodeCases() []encodeCase {
	long := strings.Repeat("The quick brown fox jumps over the lazy dog 1234567890. ", 12)
	return []encodeCase{
		{"numeric_short", "12345678"},
		{"alnum_short", "HELLO WORLD 42"},
		{"url_medium", "https://github.com/piglig/go-qr?ref=bench&v=1"},
		{"byte_long", long},
	}
}

// BenchmarkEncodeCompare pits go-qr's core text->symbol encode against the two
// most popular Go QR generators. All three encode to the in-memory symbol (no
// image rendering) at Medium ECC so the comparison is encoder-to-encoder.
//
//	go test -run=^$ -bench=BenchmarkEncodeCompare -benchmem ./...
func BenchmarkEncodeCompare(b *testing.B) {
	encoders := []struct {
		name string
		fn   func(string) error
	}{
		{"go-qr", func(s string) error {
			_, err := go_qr.EncodeText(s, go_qr.Medium)
			return err
		}},
		{"skip2/go-qrcode", func(s string) error {
			q, err := skip2.New(s, skip2.Medium)
			if err != nil {
				return err
			}
			// New() only encodes data + picks a version; the symbol build and
			// 8-mask penalty selection are deferred until Bitmap()/PNG(). Force
			// the full encode so the comparison is build-to-build.
			_ = q.Bitmap()
			return nil
		}},
		{"boombuler/barcode", func(s string) error {
			_, err := qr.Encode(s, qr.M, qr.Auto)
			return err
		}},
	}

	for _, enc := range encoders {
		for _, c := range encodeCases() {
			b.Run(fmt.Sprintf("%s/%s", enc.name, c.name), func(b *testing.B) {
				if err := enc.fn(c.text); err != nil {
					b.Fatalf("%s precheck failed: %v", enc.name, err)
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					_ = enc.fn(c.text)
				}
			})
		}
	}
}
