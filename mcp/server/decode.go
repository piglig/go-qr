package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/piglig/go-qr/mcp/internal/inspect"
	"github.com/piglig/go-qr/v2"
)

// DecodeInput selects the images to decode.
type DecodeInput struct {
	Paths       []string `json:"paths,omitempty" jsonschema:"local image file paths (PNG, JPEG or GIF); several paths are decoded together and joined if they form a structured append sequence"`
	ImageBase64 string   `json:"image_base64,omitempty" jsonschema:"alternatively, the image file contents as base64"`
}

// DecodeOutput is the result of decode_qr.
type DecodeOutput struct {
	Text       string         `json:"text" jsonschema:"the decoded content; for several unrelated images, their texts separated by newlines"`
	Symbols    []Symbol       `json:"symbols" jsonschema:"one entry per image"`
	Inspection inspect.Report `json:"inspection" jsonschema:"what the content does and its risks"`
}

// Symbol describes one decoded image.
type Symbol struct {
	Source           string            `json:"source"`
	Text             string            `json:"text"`
	Version          int               `json:"version"`
	ECC              string            `json:"ecc" jsonschema:"error correction level: L, M, Q or H"`
	Mask             int               `json:"mask"`
	Mirrored         bool              `json:"mirrored,omitempty"`
	GS1              bool              `json:"gs1,omitempty"`
	StructuredAppend *StructuredAppend `json:"structured_append,omitempty"`
	Segments         []Segment         `json:"segments"`
}

// StructuredAppend is a symbol's position in a sequence.
type StructuredAppend struct {
	Index  int  `json:"index"`
	Total  int  `json:"total"`
	Parity byte `json:"parity"`
}

// Segment summarizes one segment.
type Segment struct {
	Mode  string `json:"mode"`
	Chars int    `json:"chars"`
	ECI   int    `json:"eci" jsonschema:"ECI character set number, or -1 for none"`
}

// maxDecodeSide bounds the larger image dimension before decoding; larger
// images are downscaled, which is faster and usually more reliable.
const maxDecodeSide = 2000

func (f fileAccess) decode(_ context.Context, _ *mcp.CallToolRequest, in DecodeInput) (*mcp.CallToolResult, DecodeOutput, error) {
	type source struct {
		name string
		data []byte
	}
	var sources []source
	switch {
	case len(in.Paths) > 0 && in.ImageBase64 != "":
		return nil, DecodeOutput{}, errors.New("give either paths or image_base64, not both")
	case len(in.Paths) > 0:
		for _, p := range in.Paths {
			path, err := f.resolve(p)
			if err != nil {
				return nil, DecodeOutput{}, err
			}
			info, err := os.Stat(path)
			if err != nil {
				return nil, DecodeOutput{}, err
			}
			if info.Size() > maxImageBytes {
				return nil, DecodeOutput{}, fmt.Errorf("%s is larger than %d MB", p, maxImageBytes>>20)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, DecodeOutput{}, err
			}
			sources = append(sources, source{p, data})
		}
	case in.ImageBase64 != "":
		data, err := base64.StdEncoding.DecodeString(in.ImageBase64)
		if err != nil {
			return nil, DecodeOutput{}, fmt.Errorf("image_base64: %w", err)
		}
		if len(data) > maxImageBytes {
			return nil, DecodeOutput{}, fmt.Errorf("image is larger than %d MB", maxImageBytes>>20)
		}
		sources = append(sources, source{"image_base64", data})
	default:
		return nil, DecodeOutput{}, errors.New("give the image as paths or image_base64")
	}

	var out DecodeOutput
	results := make([]*qr.DecodeResult, len(sources))
	for i, src := range sources {
		img, _, err := image.Decode(bytes.NewReader(src.data))
		if err != nil {
			return nil, DecodeOutput{}, fmt.Errorf("%s: not a PNG, JPEG or GIF image: %w", src.name, err)
		}
		if results[i], err = qr.Decode(downscale(img, maxDecodeSide)); err != nil {
			return nil, DecodeOutput{}, fmt.Errorf("%s: %w%s", src.name, err, decodeHint(err))
		}
		out.Symbols = append(out.Symbols, symbol(src.name, results[i]))
	}

	gs1 := results[0].GS1
	switch {
	case len(results) > 1 && results[0].StructuredAppend != nil:
		text, err := qr.JoinStructuredAppend(results...)
		if err != nil {
			return nil, DecodeOutput{}, err
		}
		out.Text = text
	default:
		for i, r := range results {
			if i > 0 {
				out.Text += "\n"
			}
			out.Text += r.Text
		}
	}
	out.Inspection = inspect.Text(out.Text, gs1)
	return nil, out, nil
}

func decodeHint(err error) string {
	switch {
	case errors.Is(err, qr.ErrNotFound):
		return " (no QR Code found; the photo may be taken at an angle, too blurry, or the code too small; crop to the code and try again)"
	case errors.Is(err, qr.ErrUnsupported):
		return " (the code uses a feature this decoder does not support)"
	}
	return ""
}

func symbol(src string, r *qr.DecodeResult) Symbol {
	s := Symbol{
		Source: src, Text: r.Text, Version: r.Version, ECC: r.ECC.String(), Mask: r.Mask,
		Mirrored: r.Mirrored, GS1: r.GS1, Segments: []Segment{},
	}
	if sa := r.StructuredAppend; sa != nil {
		s.StructuredAppend = &StructuredAppend{Index: sa.Index, Total: sa.Total, Parity: sa.Parity}
	}
	for _, seg := range r.Segments {
		s.Segments = append(s.Segments, Segment{Mode: seg.Mode.String(), Chars: seg.NumChars, ECI: seg.ECI})
	}
	return s
}

// downscale returns img reduced by an integer box filter so that its longer
// side is at most maxSide pixels.
func downscale(img image.Image, maxSide int) image.Image {
	b := img.Bounds()
	f := (max(b.Dx(), b.Dy()) + maxSide - 1) / maxSide
	if f <= 1 {
		return img
	}
	src := image.NewRGBA(b)
	draw.Draw(src, b, img, b.Min, draw.Src)
	dst := image.NewGray(image.Rect(0, 0, b.Dx()/f, b.Dy()/f))
	for y := 0; y < dst.Rect.Dy(); y++ {
		for x := 0; x < dst.Rect.Dx(); x++ {
			var sum, n int
			for dy := 0; dy < f; dy++ {
				p := src.Pix[(y*f+dy)*src.Stride+x*f*4:]
				for dx := 0; dx < f; dx++ {
					q := p[dx*4:]
					// Composite over white, then take luminance.
					a := int(q[3])
					sum += (299*int(q[0])+587*int(q[1])+114*int(q[2]))/1000 + 255 - a
					n++
				}
			}
			dst.Pix[y*dst.Stride+x] = uint8(sum / n)
		}
	}
	return dst
}
