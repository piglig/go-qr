package render

import (
	"bufio"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"io"
	"sync"
)

// RGBA renders the symbol, quiet zone and logo into a new RGBA image. The
// caller has checked the logo's error correction budget.
func RGBA(c *Symbol, cfg *Config) (*image.RGBA, error) {
	side, err := cfg.RasterSide(c)
	if err != nil {
		return nil, err
	}
	var img *image.RGBA
	switch {
	case cfg.multicolor():
		img = paintColored(c, cfg, side)
	case cfg.styled():
		img = paintStyled(c, cfg, side)
	default:
		img = paintRGBA(c, cfg, side)
	}
	if cfg.Logo != nil {
		cfg.Logo.overlay(img, c.Size(), cfg)
	}
	return img, nil
}

// writeTwoColorPNG writes the plain symbol as a 1-bit paletted PNG, without
// building an image first. Its bytes are those png.Encode writes for the
// two-color paletted image of the symbol: the same chunks, rows without
// filter, compressed at the default level and split into IDAT chunks of 32 KiB.
// png.Encode reads such an image one pixel at a time through ColorIndexAt,
// which made it most of the cost of PNG.
func writeTwoColorPNG(w io.Writer, code *Symbol, c *Config, side int) error {
	pw := pngWriter{w: w}
	pw.raw([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}) // signature

	var ihdr [13]byte
	binary.BigEndian.PutUint32(ihdr[0:], uint32(side))
	binary.BigEndian.PutUint32(ihdr[4:], uint32(side))
	ihdr[8], ihdr[9] = 1, 3 // bit depth 1, palette color; no interlace
	pw.chunk("IHDR", ihdr[:])

	// Palette index 0 is the background, 1 the foreground.
	var plte [6]byte
	var trns [2]byte
	last := -1
	for i, col := range [2]color.Color{c.BG, c.FG} {
		n := color.NRGBAModel.Convert(col).(color.NRGBA)
		plte[3*i], plte[3*i+1], plte[3*i+2], trns[i] = n.R, n.G, n.B, n.A
		if n.A != 0xff {
			last = i
		}
	}
	pw.chunk("PLTE", plte[:])
	if last >= 0 {
		pw.chunk("tRNS", trns[:last+1])
	}

	bw := bufio.NewWriterSize(idatWriter{&pw}, 1<<15)
	zw := pngZlib.Get().(*zlib.Writer)
	defer pngZlib.Put(zw)
	zw.Reset(bw)
	// Each row is a filter type byte, 0 (none), then the pixels at one bit
	// each, most significant bit first; the pixels of a module row repeat
	// scale times.
	pixels := make([]uint8, side)
	row := make([]byte, 1+(side+7)/8)
	for my := -c.QuietZone; my < code.Size()+c.QuietZone && pw.err == nil; my++ {
		moduleRow(pixels, code, c, my, 1, 0)
		clear(row)
		for x, v := range pixels {
			row[1+x/8] |= v << (7 - x%8)
		}
		for i := 0; i < c.Scale; i++ {
			if _, err := zw.Write(row); err != nil {
				break
			}
		}
	}
	if err := zw.Close(); err != nil && pw.err == nil {
		pw.err = err
	}
	if err := bw.Flush(); err != nil && pw.err == nil {
		pw.err = err
	}
	pw.chunk("IEND", nil)
	return pw.err
}

// pngZlib holds zlib writers for writeTwoColorPNG: each holds about 800 KB
// of compressor state, which would otherwise be allocated for every PNG.
var pngZlib = sync.Pool{New: func() any { return zlib.NewWriter(nil) }}

// pngWriter writes PNG chunks to w, keeping the first error.
type pngWriter struct {
	w   io.Writer
	err error
}

func (p *pngWriter) raw(b []byte) {
	if p.err == nil {
		_, p.err = p.w.Write(b)
	}
}

// chunk writes a chunk: the data length, the type, the data and the CRC of
// type and data.
func (p *pngWriter) chunk(typ string, data []byte) {
	var head [8]byte
	binary.BigEndian.PutUint32(head[:4], uint32(len(data)))
	copy(head[4:], typ)
	crc := crc32.NewIEEE()
	crc.Write(head[4:])
	crc.Write(data)
	var tail [4]byte
	binary.BigEndian.PutUint32(tail[:], crc.Sum32())
	p.raw(head[:])
	p.raw(data)
	p.raw(tail[:])
}

// idatWriter writes each buffer it is given as one IDAT chunk.
type idatWriter struct{ p *pngWriter }

func (w idatWriter) Write(b []byte) (int, error) {
	w.p.chunk("IDAT", b)
	if w.p.err != nil {
		return 0, w.p.err
	}
	return len(b), nil
}

// WritePNG writes the symbol as a PNG image to w. The caller has checked the
// logo's error correction budget.
func WritePNG(w io.Writer, c *Symbol, cfg *Config) error {
	var img image.Image
	if cfg.Logo != nil || cfg.multicolor() {
		var err error
		if img, err = RGBA(c, cfg); err != nil {
			return err
		}
	} else {
		side, err := cfg.RasterSide(c)
		if err != nil {
			return err
		}
		if !cfg.styled() {
			if err := writeTwoColorPNG(w, c, cfg, side); err != nil {
				return fmt.Errorf("qr: write PNG: %w", err)
			}
			return nil
		}
		img = paintStyledPaletted(c, cfg, side)
	}

	if err := png.Encode(w, img); err != nil {
		return fmt.Errorf("qr: write PNG: %w", err)
	}
	return nil
}
