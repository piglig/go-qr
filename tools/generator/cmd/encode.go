package cmd

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/piglig/go-qr/v2"
)

const (
	blackBlock = "\033[40m  \033[0m"
	whiteBlock = "\033[47m  \033[0m"
)

// encodeOpts holds the parsed flags for the encode subcommand.
type encodeOpts struct {
	Content string
	Payload string // payload type; Content holds key=value pairs

	ECC                    string
	Simple, UTF8ECI, GS1   bool
	NoBoost, Structured    bool
	Mask                   int
	MinVersion, MaxVersion int

	Style styleOpts

	PngOutput string
	SvgOutput string
	Stdout    string // png|svg|text

	Verify  bool
	Preview bool
	Quiet   bool
}

func runEncode(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("generator encode", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var o encodeOpts

	fs.StringVar(&o.Content, "content", "", "Content to encode; may also be given as a positional argument")
	fs.StringVar(&o.Payload, "payload", "", "Interpret the content as key=value pairs for a payload: wifi, vcard, contact, event, otp, epc, email, sms, tel, geo, url")

	fs.StringVar(&o.ECC, "ecc", "high", "Minimum error correction: low, medium, quartile, high")
	fs.BoolVar(&o.NoBoost, "no-boost", false, "Keep exactly the -ecc level instead of raising it into spare capacity")
	fs.IntVar(&o.MinVersion, "min-version", qr.MinVersion, "Smallest symbol version to use")
	fs.IntVar(&o.MaxVersion, "max-version", qr.MaxVersion, "Largest symbol version to use")
	fs.IntVar(&o.Mask, "mask", -1, "Mask pattern 0-7 (-1 picks the lowest penalty)")
	fs.BoolVar(&o.Simple, "simple", false, "Encode the whole text in one mode instead of switching modes optimally")
	fs.BoolVar(&o.UTF8ECI, "utf8-eci", false, "Declare UTF-8 with an ECI designator when the text is not ASCII")
	fs.BoolVar(&o.GS1, "gs1", false, "Encode the content as GS1 data, written as (01)09501101530003(10)ABC123 or as a raw element string")
	fs.BoolVar(&o.Structured, "structured", false, "Split content too long for one symbol over up to 16 linked symbols (files get -1, -2, ... suffixes)")

	fs.IntVar(&o.Style.Scale, "scale", 10, "Pixels (PNG) or units (SVG) per module")
	fs.IntVar(&o.Style.QuietZone, "quiet-zone", 4, "Light margin around the symbol, in modules")
	fs.StringVar(&o.Style.Module, "module", "square", "Module shape: square, dot, rounded")
	fs.StringVar(&o.Style.Finder, "finder", "square", "Finder pattern shape: square, rounded, circle")
	fs.StringVar(&o.Style.FG, "fg", "", "Foreground color: #rrggbb or #rrggbbaa (default black)")
	fs.StringVar(&o.Style.BG, "bg", "", "Background color: #rrggbb, #rrggbbaa or transparent (default white)")
	fs.StringVar(&o.Style.Gradient, "gradient", "", "Linear gradient over the dark modules: from,to[,angle in degrees]")
	fs.StringVar(&o.Style.FinderColor, "finder-color", "", "Finder pattern colors: ring[,center]")
	fs.StringVar(&o.Style.Logo, "logo", "", "Logo image (png/jpeg/gif) to draw in the center")
	fs.Float64Var(&o.Style.LogoRatio, "logo-ratio", 0.2, "Logo side as a fraction of the symbol side")

	fs.StringVar(&o.PngOutput, "png", "", "Output PNG file")
	fs.StringVar(&o.SvgOutput, "svg", "", "Output SVG file")
	fs.StringVar(&o.Stdout, "stdout", "", "Write to stdout instead of files: png, svg or text")
	fs.BoolVar(&o.Verify, "verify", false, "Render, decode and check the code, including color contrast; exit 1 if it is not readable")
	fs.BoolVar(&o.Preview, "preview", false, "Print an ANSI preview to stderr")
	fs.BoolVar(&o.Quiet, "quiet", false, "Suppress non-error output")
	fs.Usage = func() {
		fmt.Fprint(stderr, "Encode text or a structured payload into QR image(s).\n\nUsage:\n  generator encode [flags] [content]\n\nFlags come before the content.\n\nFlags:\n")
		fs.PrintDefaults()
		fmt.Fprint(stderr, "\n"+payloadUsage+`
Examples:
  generator encode -png hello.png hello
  generator encode -stdout png hello > hello.png
  generator encode -stdout text hello
  generator encode -module dot -finder circle -fg '#1a237e' -svg dots.svg -verify hello
  generator encode -gradient '#4a148c,#c62828,90' -finder-color '#4a148c' -png brand.png -verify hello
  generator encode -logo logo.png -png branded.png -verify https://example.com
  generator encode -payload wifi -png wifi.png "ssid=home,password=s3cret,auth=WPA"
  generator encode -payload otp -png 2fa.png "issuer=Example,account=alice@example.com,secret=JBSWY3DPEHPK3PXP"
  generator encode -gs1 -png label.png "(01)09501101530003(17)250101(10)ABC123"
  generator encode -structured -max-version 10 -png part.png -content "$(cat long.txt)"
`)
	}

	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}

	// A positional argument is shorthand for -content.
	if fs.NArg() > 0 {
		o.Content = strings.Join(fs.Args(), " ")
	}
	if o.Content == "" {
		fs.Usage()
		return fmt.Errorf("encode: missing content (positional argument or -content)")
	}
	if o.Stdout != "" {
		if bad := setFlagsAmong(fs, "png", "svg"); len(bad) > 0 {
			return fmt.Errorf("-stdout cannot be combined with file outputs (%s)", strings.Join(bad, ", "))
		}
	}

	text, err := resolveContent(o.Content, o.Payload)
	if err != nil {
		return fmt.Errorf("payload: %w", err)
	}
	if o.GS1 {
		if text, err = gs1FromHRI(text); err != nil {
			return err
		}
	}
	encOpts, err := o.encodeOptions()
	if err != nil {
		return err
	}
	renderOpts, err := o.Style.renderOptions()
	if err != nil {
		return err
	}

	var codes []*qr.Code
	if o.Structured {
		codes, err = qr.EncodeStructured(text, encOpts...)
	} else {
		var code *qr.Code
		code, err = qr.Encode(text, encOpts...)
		codes = []*qr.Code{code}
	}
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	if o.Verify {
		for i, code := range codes {
			if err := code.Verify(renderOpts...); err != nil {
				return fmt.Errorf("verify%s: %w", partLabel(i, len(codes)), err)
			}
		}
		if !o.Quiet {
			fmt.Fprintln(stderr, "verify: ok")
		}
	}

	if o.Stdout != "" {
		if len(codes) > 1 && o.Stdout != "text" {
			return fmt.Errorf("-stdout %s writes one image, but the content needs %d symbols; use -png or -svg", o.Stdout, len(codes))
		}
		for i, code := range codes {
			if i > 0 {
				fmt.Fprintln(stdout)
			}
			if err := writeStdout(code, renderOpts, o.Stdout, stdout); err != nil {
				return err
			}
		}
		return nil
	}

	for i, code := range codes {
		if o.PngOutput != "" {
			b, err := code.PNG(renderOpts...)
			if err != nil {
				return fmt.Errorf("png: %w", err)
			}
			if err := os.WriteFile(numbered(o.PngOutput, i, len(codes)), b, 0o644); err != nil {
				return err
			}
		}
		if o.SvgOutput != "" {
			b, err := code.SVG(renderOpts...)
			if err != nil {
				return fmt.Errorf("svg: %w", err)
			}
			if err := os.WriteFile(numbered(o.SvgOutput, i, len(codes)), b, 0o644); err != nil {
				return err
			}
		}
	}
	if len(codes) > 1 && !o.Quiet && (o.PngOutput != "" || o.SvgOutput != "") {
		fmt.Fprintf(stderr, "wrote %d symbols\n", len(codes))
	}

	noOutput := o.PngOutput == "" && o.SvgOutput == ""
	if o.Preview || noOutput && !o.Verify {
		// Nothing requested: fall back to a preview so the command is never silent.
		for _, code := range codes {
			fmt.Fprint(stderr, renderPreview(code))
		}
	}
	return nil
}

func (o encodeOpts) encodeOptions() ([]qr.EncodeOption, error) {
	ecc, err := parseECC(o.ECC)
	if err != nil {
		return nil, err
	}
	opts := []qr.EncodeOption{qr.WithECC(ecc), qr.WithVersionRange(o.MinVersion, o.MaxVersion)}
	if o.Mask >= 0 {
		opts = append(opts, qr.WithMask(o.Mask))
	}
	if o.NoBoost {
		opts = append(opts, qr.WithoutECCBoost())
	}
	if o.Simple {
		opts = append(opts, qr.WithSimpleSegmentation())
	}
	if o.UTF8ECI {
		opts = append(opts, qr.WithUTF8ECI())
	}
	if o.GS1 {
		opts = append(opts, qr.WithGS1())
	}
	return opts, nil
}

func parseECC(s string) (qr.ECC, error) {
	switch strings.ToLower(s) {
	case "low", "l":
		return qr.ECCLow, nil
	case "medium", "m":
		return qr.ECCMedium, nil
	case "quartile", "q":
		return qr.ECCQuartile, nil
	case "high", "h":
		return qr.ECCHigh, nil
	default:
		return 0, fmt.Errorf("unknown ecc level %q (expected low|medium|quartile|high)", s)
	}
}

// numbered inserts a 1-based part number before the extension of path when
// there is more than one symbol: out.png becomes out-1.png, out-2.png, ...
func numbered(path string, i, n int) string {
	if n == 1 {
		return path
	}
	ext := filepath.Ext(path)
	return fmt.Sprintf("%s-%d%s", strings.TrimSuffix(path, ext), i+1, ext)
}

func partLabel(i, n int) string {
	if n == 1 {
		return ""
	}
	return fmt.Sprintf(" (symbol %d of %d)", i+1, n)
}

func writeStdout(code *qr.Code, opts []qr.RenderOption, format string, w io.Writer) error {
	switch strings.ToLower(format) {
	case "png":
		return code.WritePNG(w, opts...)
	case "svg":
		return code.WriteSVG(w, opts...)
	case "text":
		return code.WriteText(w, opts...)
	default:
		return fmt.Errorf("unknown stdout format %q (expected png, svg, or text)", format)
	}
}

func renderPreview(code *qr.Code) string {
	var buf bytes.Buffer
	const border = 2
	for y := -border; y < code.Size()+border; y++ {
		for x := -border; x < code.Size()+border; x++ {
			if code.Module(x, y) {
				buf.WriteString(blackBlock)
			} else {
				buf.WriteString(whiteBlock)
			}
		}
		buf.WriteString("\n")
	}
	return buf.String()
}
