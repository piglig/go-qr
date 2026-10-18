package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"io"
	"math"
	"reflect"
	"time"

	"github.com/piglig/go-qr/v2"
	"github.com/piglig/go-qr/v2/payload"
)

// decodedSymbol is the -json form of one decoded image.
type decodedSymbol struct {
	File             string           `json:"file"`
	Text             string           `json:"text"`
	Version          int              `json:"version"`
	ECC              string           `json:"ecc"`
	Mask             int              `json:"mask"`
	Mirrored         bool             `json:"mirrored,omitempty"`
	Inverted         bool             `json:"inverted,omitempty"`
	Corners          [4][2]int        `json:"corners"`
	GS1              bool             `json:"gs1,omitempty"`
	StructuredAppend *structuredJSON  `json:"structuredAppend,omitempty"`
	Payload          string           `json:"payload,omitempty"`
	Segments         []decodedSegment `json:"segments"`

	ErrorsCorrected       int          `json:"errorsCorrected"`
	UnusedErrorCorrection float64      `json:"unusedErrorCorrection"`
	SoftDecision          bool         `json:"softDecision,omitempty"`
	Quality               *qualityJSON `json:"quality,omitempty"` // with -grade
}

// qualityJSON is the -json form of a symbol's print quality grade.
type qualityJSON struct {
	Grade                 string     `json:"grade,omitempty"`
	SymbolContrast        *paramJSON `json:"symbolContrast,omitempty"`
	Modulation            *paramJSON `json:"modulation,omitempty"`
	ReflectanceMargin     *paramJSON `json:"reflectanceMargin,omitempty"`
	FixedPatternDamage    *paramJSON `json:"fixedPatternDamage,omitempty"`
	AxialNonuniformity    *paramJSON `json:"axialNonuniformity,omitempty"`
	GridNonuniformity     *paramJSON `json:"gridNonuniformity,omitempty"`
	UnusedErrorCorrection *paramJSON `json:"unusedErrorCorrection,omitempty"`
	PixelsPerModule       float64    `json:"pixelsPerModule,omitempty"`
	Error                 string     `json:"error,omitempty"` // why the symbol could not be graded
}

type paramJSON struct {
	Value float64 `json:"value"`
	Grade string  `json:"grade"`
}

type structuredJSON struct {
	Index  int  `json:"index"`
	Total  int  `json:"total"`
	Parity byte `json:"parity"`
}

type decodedSegment struct {
	Mode  string `json:"mode"`
	Chars int    `json:"chars"`
	ECI   int    `json:"eci"`
}

func runDecode(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("generator decode", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "Print the result as JSON, with version, error correction, mask, corners, segments and payload type")
	all := fs.Bool("all", false, "Decode every QR code in each image, not only one")
	grade := fs.Bool("grade", false, "Also grade each code's print quality by the parameters of ISO/IEC 15415 (an estimate from the image)")
	timeout := fs.Duration("timeout", 0, "Give up on an image after this long, such as 2s (default: no limit)")
	fs.Usage = func() {
		fmt.Fprint(stderr, `Decode QR code images (png/jpeg/gif) and print their text.

Usage:
  generator decode [flags] <image-file>...

With -all, every code in each image is decoded, in reading order. Symbols
that form a structured append sequence are joined into one message, in any
order; otherwise each text is printed on its own line. With -grade, a line
per code follows with its print quality grade, A to F, and the grade of
each parameter.

Flags:
`)
		fs.PrintDefaults()
		fmt.Fprint(stderr, `
Examples:
  generator decode hello.png
  generator decode -json hello.png
  generator decode part-*.png
  generator decode -all shelf.jpg
  generator decode -grade -timeout 2s label.jpg
`)
	}
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	if fs.NArg() == 0 {
		fs.Usage()
		return fmt.Errorf("decode: missing image path")
	}

	var results []*qr.DecodeResult
	var files []string // the image of each result
	var grades []gradeResult
	for _, path := range fs.Args() {
		img, err := loadImage(path)
		if err != nil {
			return fmt.Errorf("decode %s: load image: %w", path, err)
		}
		rs, err := decodeImage(img, *all, *timeout)
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("decode %s: no code read within %v", path, *timeout)
		}
		if err != nil {
			return fmt.Errorf("decode %s: %w", path, err)
		}
		for _, res := range rs {
			results = append(results, res)
			files = append(files, path)
			if *grade {
				grades = append(grades, gradeSymbol(img, res))
			}
		}
	}

	// A complete structured append sequence prints as one message.
	joined, sequence := "", len(results) > 1 && results[0].StructuredAppend != nil
	if sequence {
		var err error
		if joined, err = qr.JoinStructuredAppend(results...); err != nil {
			return fmt.Errorf("decode: %w", err)
		}
	}

	if *jsonOut {
		out := struct {
			Text    *string         `json:"text,omitempty"`
			Symbols []decodedSymbol `json:"symbols"`
		}{}
		for i, res := range results {
			s := symbolJSON(files[i], res)
			if *grade {
				s.Quality = grades[i].json()
			}
			out.Symbols = append(out.Symbols, s)
		}
		switch {
		case sequence:
			out.Text = &joined
		case len(results) == 1:
			out.Text = &results[0].Text
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		return enc.Encode(out)
	}

	if sequence {
		fmt.Fprintln(stdout, joined)
	} else {
		for _, res := range results {
			fmt.Fprintln(stdout, res.Text)
		}
	}
	for i, g := range grades {
		fmt.Fprintf(stdout, "%s: %s\n", files[i], g)
	}
	return nil
}

// decodeImage decodes one code of img, or with all every code, giving up
// after timeout if it is not zero.
func decodeImage(img image.Image, all bool, timeout time.Duration) ([]*qr.DecodeResult, error) {
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	if all {
		rs, err := qr.DecodeAllContext(ctx, img)
		if errors.Is(err, context.DeadlineExceeded) && len(rs) > 0 {
			err = nil // keep the codes read in time
		}
		return rs, err
	}
	res, err := qr.DecodeContext(ctx, img)
	if err != nil {
		return nil, err
	}
	return []*qr.DecodeResult{res}, nil
}

// gradeResult is the print quality of one decoded code, or why it could not
// be graded.
type gradeResult struct {
	q   *qr.Quality
	err error
}

// gradeSymbol grades the code res was read from in img, in a window around
// its corners so that the other codes of the image do not interfere.
func gradeSymbol(img image.Image, res *qr.DecodeResult) gradeResult {
	q, err := qr.Grade(symbolWindow(img, res))
	if res.SoftDecision && errors.Is(err, qr.ErrDecodeFailed) {
		err = errors.New("only soft decisions read it, which most readers do not have")
	}
	return gradeResult{q, err}
}

// symbolWindow returns the part of img that holds res's symbol and a
// margin of eight modules, the quiet zone with room for error, or img
// itself if it cannot be cropped.
func symbolWindow(img image.Image, res *qr.DecodeResult) image.Image {
	sub, ok := img.(interface {
		SubImage(image.Rectangle) image.Image
	})
	if !ok {
		return img
	}
	r := image.Rectangle{Min: res.Corners[0], Max: res.Corners[0]}
	side := 0.0
	for i, p := range res.Corners {
		r = r.Union(image.Rectangle{Min: p, Max: p.Add(image.Pt(1, 1))})
		q := res.Corners[(i+1)%4]
		side = max(side, math.Hypot(float64(q.X-p.X), float64(q.Y-p.Y)))
	}
	margin := int(math.Ceil(8 * side / float64(4*res.Version+17)))
	return sub.SubImage(r.Inset(-margin).Intersect(img.Bounds()))
}

func (g gradeResult) String() string {
	if g.err != nil {
		return "not graded: " + g.err.Error()
	}
	q := g.q
	return fmt.Sprintf("grade %v (symbol contrast %v, modulation %v, reflectance margin %v, fixed pattern damage %v, axial nonuniformity %v, grid nonuniformity %v, unused error correction %v; %.1f px per module)",
		q.Grade, q.SymbolContrast.Grade, q.Modulation.Grade, q.ReflectanceMargin.Grade, q.FixedPatternDamage.Grade,
		q.AxialNonuniformity.Grade, q.GridNonuniformity.Grade, q.UnusedErrorCorrection.Grade, q.PixelsPerModule)
}

func (g gradeResult) json() *qualityJSON {
	if g.err != nil {
		return &qualityJSON{Error: g.err.Error()}
	}
	p := func(v qr.QualityParameter) *paramJSON { return &paramJSON{Value: v.Value, Grade: v.Grade.String()} }
	q := g.q
	return &qualityJSON{
		Grade: q.Grade.String(), SymbolContrast: p(q.SymbolContrast), Modulation: p(q.Modulation),
		ReflectanceMargin: p(q.ReflectanceMargin), FixedPatternDamage: p(q.FixedPatternDamage),
		AxialNonuniformity: p(q.AxialNonuniformity), GridNonuniformity: p(q.GridNonuniformity),
		UnusedErrorCorrection: p(q.UnusedErrorCorrection), PixelsPerModule: q.PixelsPerModule,
	}
}

func symbolJSON(file string, res *qr.DecodeResult) decodedSymbol {
	s := decodedSymbol{
		File: file, Text: res.Text, Version: res.Version, ECC: res.ECC.String(), Mask: res.Mask,
		Mirrored: res.Mirrored, Inverted: res.Inverted, GS1: res.GS1, Segments: []decodedSegment{},
		ErrorsCorrected: res.ErrorsCorrected, UnusedErrorCorrection: res.UnusedErrorCorrection, SoftDecision: res.SoftDecision,
	}
	for i, p := range res.Corners {
		s.Corners[i] = [2]int{p.X, p.Y}
	}
	if sa := res.StructuredAppend; sa != nil {
		s.StructuredAppend = &structuredJSON{Index: sa.Index, Total: sa.Total, Parity: sa.Parity}
	}
	for _, seg := range res.Segments {
		s.Segments = append(s.Segments, decodedSegment{Mode: seg.Mode.String(), Chars: seg.NumChars, ECI: seg.ECI})
	}
	if p, err := payload.Parse(res.Text); err == nil {
		s.Payload = reflect.TypeOf(p).Name()
	} else if !errors.Is(err, payload.ErrUnrecognized) {
		s.Payload = "malformed"
	}
	return s
}
