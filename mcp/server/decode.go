package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/piglig/go-qr/mcp/internal/inspect"
	"github.com/piglig/go-qr/v2"
)

// DecodeInput selects the images to decode.
type DecodeInput struct {
	Paths       []string `json:"paths,omitempty" jsonschema:"local image file paths (PNG, JPEG or GIF); several paths are decoded together and joined if they form a structured append sequence"`
	ImageBase64 string   `json:"image_base64,omitempty" jsonschema:"alternatively, the image file contents as base64"`
	Grade       bool     `json:"grade,omitempty" jsonschema:"also grade each code's print quality, A to F, by the parameters of ISO/IEC 15415 (an estimate from the image); for checking printed labels"`
}

// DecodeOutput is the result of decode_qr.
type DecodeOutput struct {
	Text       string         `json:"text" jsonschema:"the decoded content; for several unrelated codes, their texts separated by newlines"`
	Symbols    []Symbol       `json:"symbols" jsonschema:"one entry per code, every code of each image in reading order"`
	Inspection inspect.Report `json:"inspection" jsonschema:"what the content does and its risks; for several unrelated codes, the signals of all of them"`
}

// Symbol describes one decoded image.
type Symbol struct {
	Source           string            `json:"source"`
	Text             string            `json:"text"`
	Version          int               `json:"version"`
	ECC              string            `json:"ecc" jsonschema:"error correction level: L, M, Q or H"`
	Mask             int               `json:"mask"`
	Mirrored         bool              `json:"mirrored,omitempty"`
	Inverted         bool              `json:"inverted,omitempty" jsonschema:"light modules on a dark background"`
	Corners          [4][2]int         `json:"corners" jsonschema:"the code's corners in the image, in pixels: top-left, top-right, bottom-right, bottom-left"`
	GS1              bool              `json:"gs1,omitempty"`
	StructuredAppend *StructuredAppend `json:"structured_append,omitempty"`
	Segments         []Segment         `json:"segments"`
	Inspection       *inspect.Report   `json:"inspection,omitempty" jsonschema:"what this code does and its risks, when several unrelated codes were found"`

	ErrorsCorrected       int      `json:"errors_corrected" jsonschema:"codewords that error correction repaired"`
	UnusedErrorCorrection float64  `json:"unused_error_correction" jsonschema:"share of the error correction capacity left unused, from 1 (no damage) to 0 (barely readable), as ISO/IEC 15415 defines it"`
	SoftDecision          bool     `json:"soft_decision,omitempty" jsonschema:"only soft decisions read the code; many phone scanners may not"`
	Quality               *Quality `json:"quality,omitempty" jsonschema:"print quality grade, with grade set"`
}

// Quality is a code's print quality by the parameters of ISO/IEC 15415.
type Quality struct {
	Grade                 string     `json:"grade,omitempty" jsonschema:"A (best) to F (fail), the lowest of the parameters' grades; C or better is commonly asked of prints"`
	SymbolContrast        *Parameter `json:"symbol_contrast,omitempty"`
	Modulation            *Parameter `json:"modulation,omitempty"`
	ReflectanceMargin     *Parameter `json:"reflectance_margin,omitempty"`
	FixedPatternDamage    *Parameter `json:"fixed_pattern_damage,omitempty"`
	AxialNonuniformity    *Parameter `json:"axial_nonuniformity,omitempty"`
	GridNonuniformity     *Parameter `json:"grid_nonuniformity,omitempty"`
	UnusedErrorCorrection *Parameter `json:"unused_error_correction,omitempty"`
	PixelsPerModule       float64    `json:"pixels_per_module,omitempty"`
	Error                 string     `json:"error,omitempty" jsonschema:"why the code could not be graded"`
}

// Parameter is one graded parameter.
type Parameter struct {
	Value float64 `json:"value"`
	Grade string  `json:"grade"`
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

func (f fileAccess) decode(ctx context.Context, _ *mcp.CallToolRequest, in DecodeInput) (*mcp.CallToolResult, DecodeOutput, error) {
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
	var results []*qr.DecodeResult
	for _, src := range sources {
		// A small file can declare a huge image; check its dimensions
		// before allocating its pixels.
		if cfg, _, err := image.DecodeConfig(bytes.NewReader(src.data)); err == nil && int64(cfg.Width)*int64(cfg.Height) > maxImagePixels {
			return nil, DecodeOutput{}, fmt.Errorf("%s: image of %dx%d pixels is larger than %d megapixels",
				src.name, cfg.Width, cfg.Height, maxImagePixels/1_000_000)
		}
		img, _, err := image.Decode(bytes.NewReader(src.data))
		if err != nil {
			return nil, DecodeOutput{}, fmt.Errorf("%s: not a PNG, JPEG or GIF image: %w", src.name, err)
		}
		// The search stops when the client cancels the call.
		rs, err := qr.DecodeAllContext(ctx, img)
		if err != nil {
			return nil, DecodeOutput{}, fmt.Errorf("%s: %w%s", src.name, err, decodeHint(err))
		}
		for _, r := range rs {
			results = append(results, r)
			s := symbol(src.name, r)
			if in.Grade {
				s.Quality = grade(symbolWindow(img, r))
			}
			out.Symbols = append(out.Symbols, s)
		}
	}

	switch {
	case len(results) > 1 && results[0].StructuredAppend != nil:
		text, err := qr.JoinStructuredAppend(results...)
		if err != nil {
			return nil, DecodeOutput{}, err
		}
		out.Text = text
		out.Inspection = inspect.Text(text, results[0].GS1)
	case len(results) == 1:
		out.Text = results[0].Text
		out.Inspection = inspect.Text(out.Text, results[0].GS1)
	default:
		// Unrelated codes do different things, so each is inspected on its
		// own and the summary carries every signal.
		labels := make([]string, len(results))
		reports := make([]inspect.Report, len(results))
		for i, r := range results {
			if i > 0 {
				out.Text += "\n"
			}
			out.Text += r.Text
			reports[i] = inspect.Text(r.Text, r.GS1)
			out.Symbols[i].Inspection = &reports[i]
			labels[i] = out.Symbols[i].Source
		}
		out.Inspection = inspect.Combine(labels, reports)
	}
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
		Mirrored: r.Mirrored, Inverted: r.Inverted, GS1: r.GS1, Segments: []Segment{},
		ErrorsCorrected: r.ErrorsCorrected, UnusedErrorCorrection: r.UnusedErrorCorrection, SoftDecision: r.SoftDecision,
	}
	for i, p := range r.Corners {
		s.Corners[i] = [2]int{p.X, p.Y}
	}
	if sa := r.StructuredAppend; sa != nil {
		s.StructuredAppend = &StructuredAppend{Index: sa.Index, Total: sa.Total, Parity: sa.Parity}
	}
	for _, seg := range r.Segments {
		s.Segments = append(s.Segments, Segment{Mode: seg.Mode.String(), Chars: seg.NumChars, ECI: seg.ECI})
	}
	return s
}

// grade grades the print quality of the code in img.
func grade(img image.Image) *Quality {
	q, err := qr.Grade(img)
	if errors.Is(err, qr.ErrDecodeFailed) {
		return &Quality{Error: "only soft decisions read the code, which most readers do not have; it grades F"}
	}
	if err != nil {
		return &Quality{Error: err.Error()}
	}
	p := func(v qr.QualityParameter) *Parameter { return &Parameter{Value: v.Value, Grade: v.Grade.String()} }
	return &Quality{
		Grade: q.Grade.String(), SymbolContrast: p(q.SymbolContrast), Modulation: p(q.Modulation),
		ReflectanceMargin: p(q.ReflectanceMargin), FixedPatternDamage: p(q.FixedPatternDamage),
		AxialNonuniformity: p(q.AxialNonuniformity), GridNonuniformity: p(q.GridNonuniformity),
		UnusedErrorCorrection: p(q.UnusedErrorCorrection), PixelsPerModule: q.PixelsPerModule,
	}
}

// symbolWindow returns the part of img that holds r's symbol and a margin
// of eight modules, the quiet zone with room for error, so that the other
// codes of a photo do not interfere with grading it; or img itself if it
// cannot be cropped.
func symbolWindow(img image.Image, r *qr.DecodeResult) image.Image {
	sub, ok := img.(interface {
		SubImage(image.Rectangle) image.Image
	})
	if !ok {
		return img
	}
	box := image.Rectangle{Min: r.Corners[0], Max: r.Corners[0]}
	side := 0.0
	for i, p := range r.Corners {
		box = box.Union(image.Rectangle{Min: p, Max: p.Add(image.Pt(1, 1))})
		q := r.Corners[(i+1)%4]
		side = max(side, math.Hypot(float64(q.X-p.X), float64(q.Y-p.Y)))
	}
	margin := int(math.Ceil(8 * side / float64(4*r.Version+17)))
	return sub.SubImage(box.Inset(-margin).Intersect(img.Bounds()))
}
