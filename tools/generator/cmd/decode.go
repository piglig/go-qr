package cmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"reflect"

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
	fs.Usage = func() {
		fmt.Fprint(stderr, `Decode QR code images (png/jpeg/gif) and print their text.

Usage:
  generator decode [flags] <image-file>...

With -all, every code in each image is decoded, in reading order. Symbols
that form a structured append sequence are joined into one message, in any
order; otherwise each text is printed on its own line.

Flags:
`)
		fs.PrintDefaults()
		fmt.Fprint(stderr, `
Examples:
  generator decode hello.png
  generator decode -json hello.png
  generator decode part-*.png
  generator decode -all shelf.jpg
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
	for _, path := range fs.Args() {
		img, err := loadImage(path)
		if err != nil {
			return fmt.Errorf("decode %s: load image: %w", path, err)
		}
		var rs []*qr.DecodeResult
		if *all {
			rs, err = qr.DecodeAll(img)
		} else {
			var res *qr.DecodeResult
			res, err = qr.Decode(img)
			rs = []*qr.DecodeResult{res}
		}
		if err != nil {
			return fmt.Errorf("decode %s: %w", path, err)
		}
		for _, res := range rs {
			results = append(results, res)
			files = append(files, path)
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
			out.Symbols = append(out.Symbols, symbolJSON(files[i], res))
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
		return nil
	}
	for _, res := range results {
		fmt.Fprintln(stdout, res.Text)
	}
	return nil
}

func symbolJSON(file string, res *qr.DecodeResult) decodedSymbol {
	s := decodedSymbol{
		File: file, Text: res.Text, Version: res.Version, ECC: res.ECC.String(), Mask: res.Mask,
		Mirrored: res.Mirrored, Inverted: res.Inverted, GS1: res.GS1, Segments: []decodedSegment{},
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
