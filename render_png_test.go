package qr

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/png"
	"strings"
	"sync"
	"testing"
)

// paintPaletted renders code into a two-color paletted image. png.Encode of
// it is the reference writeTwoColorPNG matches byte for byte.
func paintPaletted(code *Code, c *renderConfig, side int) *image.Paletted {
	img := image.NewPaletted(image.Rect(0, 0, side, side), color.Palette{c.bg, c.fg})
	for my := -c.quietZone; my < code.Size()+c.quietZone; my++ {
		y0 := (my + c.quietZone) * c.scale
		line := img.Pix[y0*img.Stride : y0*img.Stride+side]
		moduleRow(line, code, c, my, 1, 0)
		for dy := 1; dy < c.scale; dy++ {
			copy(img.Pix[(y0+dy)*img.Stride:], line)
		}
	}
	return img
}

// referencePNG is the PNG png.Encode writes for code with opts.
func referencePNG(t *testing.T, code *Code, opts ...RenderOption) []byte {
	t.Helper()
	cfg, err := newRenderConfig(opts)
	assertNoError(t, err)
	side, err := cfg.rasterSide(code)
	assertNoError(t, err)
	var buf bytes.Buffer
	assertNoError(t, png.Encode(&buf, paintPaletted(code, &cfg, side)))
	return buf.Bytes()
}

// idatChunks counts the IDAT chunks of a PNG.
func idatChunks(t *testing.T, b []byte) int {
	t.Helper()
	n := 0
	for i := 8; i+8 <= len(b); {
		length := int(binary.BigEndian.Uint32(b[i:]))
		if string(b[i+4:i+8]) == "IDAT" {
			n++
		}
		i += 12 + length
	}
	return n
}

func TestPNGMatchesStandardEncoder(t *testing.T) {
	small, _ := Encode("https://github.com/piglig/go-qr")
	large, _ := EncodeBytes(bytes.Repeat([]byte{0x5a, 0xc3, 0x17}, 900), WithECC(ECCLow), WithVersionRange(40, 40))
	gray := color.Gray16{Y: 0x8000}
	tests := map[string]struct {
		code *Code
		opts []RenderOption
	}{
		"default":             {small, nil},
		"scale 1":             {small, []RenderOption{WithScale(1)}},
		"scale 3":             {small, []RenderOption{WithScale(3)}},
		"scale 8":             {small, []RenderOption{WithScale(8)}},
		"scale 13":            {small, []RenderOption{WithScale(13), WithQuietZone(1)}},
		"no quiet zone":       {small, []RenderOption{WithQuietZone(0), WithScale(5)}},
		"wide quiet zone":     {small, []RenderOption{WithQuietZone(11), WithScale(2)}},
		"colors":              {small, []RenderOption{WithForeground(color.RGBA{R: 0x1a, G: 0x23, B: 0x7e, A: 0xff}), WithBackground(color.NRGBA{R: 0xff, G: 0xf8, B: 0xe1, A: 0xff})}},
		"16-bit colors":       {small, []RenderOption{WithForeground(gray), WithBackground(color.White)}},
		"same colors":         {small, []RenderOption{WithForeground(color.Black), WithBackground(color.Black)}},
		"transparent bg":      {small, []RenderOption{WithBackground(color.Transparent)}},
		"translucent fg":      {small, []RenderOption{WithForeground(color.NRGBA{A: 0x80})}},
		"both translucent":    {small, []RenderOption{WithForeground(color.NRGBA{R: 9, A: 0x40}), WithBackground(color.NRGBA{G: 9, A: 0xc0})}},
		"version 40":          {large, []RenderOption{WithScale(9)}},
		"several IDAT chunks": {large, []RenderOption{WithScale(37)}},
		"version 40 scale 1":  {large, []RenderOption{WithScale(1), WithQuietZone(0)}},
	}
	multipleIDAT := false
	for name, tc := range tests {
		got, err := tc.code.PNG(tc.opts...)
		assertNoError(t, err)
		want := referencePNG(t, tc.code, tc.opts...)
		if !bytes.Equal(got, want) {
			t.Errorf("%s: PNG differs from png.Encode: %d bytes, want %d", name, len(got), len(want))
		}
		if idatChunks(t, want) > 1 {
			multipleIDAT = true
		}
	}
	if large.Version() != 40 {
		t.Errorf("large code is version %d, want 40", large.Version())
	}
	if !multipleIDAT {
		t.Error("no test case splits its data into several IDAT chunks")
	}
}

func TestPNGConcurrent(t *testing.T) {
	code, _ := Encode("concurrent")
	want := referencePNG(t, code)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				if got, err := code.PNG(); err != nil || !bytes.Equal(got, want) {
					t.Errorf("concurrent PNG differs or failed: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

// limitWriter fails once n bytes have been written.
type limitWriter struct{ n int }

func (w *limitWriter) Write(b []byte) (int, error) {
	if len(b) > w.n {
		k := w.n
		w.n = 0
		return k, errors.New("disk full")
	}
	w.n -= len(b)
	return len(b), nil
}

func TestWritePNGWriterErrors(t *testing.T) {
	code, _ := Encode("https://github.com/piglig/go-qr")
	full, _ := code.PNG()
	for n := 0; n < len(full); n++ {
		err := code.WritePNG(&limitWriter{n: n})
		if err == nil || !strings.Contains(err.Error(), "disk full") {
			t.Fatalf("writer failing after %d of %d bytes: error = %v", n, len(full), err)
		}
	}
	// The writer pool still produces correct output after failed writes.
	again, _ := code.PNG()
	if !bytes.Equal(again, full) {
		t.Error("PNG differs after failed writes")
	}
}
