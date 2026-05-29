// Package cmd implements the `generator` CLI, organized as subcommands:
//
//	generator encode  [flags] [content]
//	generator decode  [flags] <image>
//	generator version
//	generator help
package cmd

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"runtime/debug"
	"strconv"
	"strings"

	go_qr "github.com/piglig/go-qr"
	"github.com/piglig/go-qr/payload"
	"github.com/piglig/go-qr/tools/verify"
)

const (
	blackBlock = "\033[40m  \033[0m"
	whiteBlock = "\033[47m  \033[0m"
)

// errReported marks an error whose message was already written to stderr (e.g.
// by the flag package). Exec exits non-zero without printing it again.
var errReported = errors.New("error already reported")

// Exec is the CLI entrypoint.
func Exec() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if !errors.Is(err, errReported) {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}

// run dispatches to a subcommand. The first argument is the command name.
func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		topUsage(stderr)
		return errReported
	}

	cmd, rest := args[0], args[1:]
	switch cmd {
	case "encode":
		return runEncode(rest, stdout, stderr)
	case "decode":
		return runDecode(rest, stdout, stderr)
	case "version", "-version", "--version":
		fmt.Fprintln(stdout, version())
		return nil
	case "help", "-h", "-help", "--help":
		topUsage(stdout)
		return nil
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n", cmd)
		topUsage(stderr)
		return errReported
	}
}

// topUsage prints the list of subcommands.
func topUsage(w io.Writer) {
	fmt.Fprint(w, `generator — encode/decode QR codes.

Usage:
  generator <command> [flags] [args]

Commands:
  encode     Encode text or a structured payload into QR image(s)
  decode     Decode a QR code image into text
  version    Print version and exit
  help       Show this help

Run "generator <command> -h" for command-specific flags.

Examples:
  generator encode hello -png hello.png
  generator encode -payload wifi "ssid=home,password=s3cret,auth=WPA" -png wifi.png
  generator decode hello.png
`)
}

// encodeOpts holds the parsed flags for the encode subcommand.
type encodeOpts struct {
	Content string
	Payload string // wifi|vcard|email|sms|tel|geo|url; interprets Content as key=val pairs
	ECC     string // low|medium|quartile|high

	Scale, Border int

	PngOutput          string
	SvgOutput          string
	SvgOptimizedOutput string
	Stdout             string // png|svg|svg-optimized — write bytes to stdout

	Logo      string
	LogoRatio float64

	Verify  bool
	Preview bool
	Quiet   bool
}

func runEncode(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("generator encode", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var o encodeOpts
	fs.StringVar(&o.Content, "content", "", "Content to encode; may also be given as a positional argument")
	fs.StringVar(&o.Payload, "payload", "", "Structured payload type: wifi, vcard, email, sms, tel, geo, url")
	fs.StringVar(&o.ECC, "ecc", "high", "Error correction: low, medium, quartile, high")
	fs.IntVar(&o.Scale, "scale", 10, "Scale (pixels per module for PNG / units per module for SVG)")
	fs.IntVar(&o.Border, "border", 4, "Quiet-zone border, in modules")
	fs.StringVar(&o.PngOutput, "png", "", "Output PNG file path")
	fs.StringVar(&o.SvgOutput, "svg", "", "Output SVG file path")
	fs.StringVar(&o.SvgOptimizedOutput, "svg-optimized", "", "Output optimized SVG file path")
	fs.StringVar(&o.Stdout, "stdout", "", "Write to stdout instead of files: png, svg, or svg-optimized")
	fs.StringVar(&o.Logo, "logo", "", "Path to a logo image (png/jpeg/gif) to embed in the center")
	fs.Float64Var(&o.LogoRatio, "logo-ratio", 0.2, "Logo side length as fraction of QR module-area side")
	fs.BoolVar(&o.Verify, "verify", false, "Decode the generated PNG and assert it matches the input (exit 1 on mismatch)")
	fs.BoolVar(&o.Preview, "preview", false, "Print ANSI terminal preview to stderr")
	fs.BoolVar(&o.Quiet, "quiet", false, "Suppress non-error output")
	fs.Usage = func() {
		fmt.Fprint(stderr, "Encode text or a structured payload into QR image(s).\n\nUsage:\n  generator encode [flags] [content]\n\nFlags:\n")
		fs.PrintDefaults()
		fmt.Fprint(stderr, `
Examples:
  generator encode hello -png hello.png
  generator encode hello -stdout png > hello.png
  generator encode hello -svg-optimized hello.svg
  generator encode -payload wifi "ssid=home,password=s3cret,auth=WPA" -png wifi.png
  generator encode hello -png hello.png -verify
`)
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return errReported
	}

	// A positional argument is shorthand for -content.
	if fs.NArg() > 0 {
		o.Content = strings.Join(fs.Args(), " ")
	}
	if o.Content == "" {
		fs.Usage()
		return fmt.Errorf("encode: missing content (positional argument or -content)")
	}

	text, err := resolveContent(o.Content, o.Payload)
	if err != nil {
		return fmt.Errorf("payload: %w", err)
	}

	ecl, err := parseECC(o.ECC)
	if err != nil {
		return err
	}

	qr, err := go_qr.EncodeText(text, ecl)
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	imgOpts, err := buildImgOpts(o.Logo, o.LogoRatio)
	if err != nil {
		return err
	}

	baseCfg := func(extra ...go_qr.Option) *go_qr.QrCodeImgConfig {
		all := append([]go_qr.Option{}, imgOpts...)
		all = append(all, extra...)
		return go_qr.NewQrCodeImgConfig(o.Scale, o.Border, all...)
	}

	if o.Stdout != "" {
		if bad := setFlagsAmong(fs, "png", "svg", "svg-optimized"); len(bad) > 0 {
			return fmt.Errorf("-stdout cannot be combined with file outputs (%s)", strings.Join(bad, ", "))
		}
		return writeStdout(qr, baseCfg, o.Stdout, stdout)
	}

	if o.PngOutput != "" {
		b, err := qr.ToPNGBytes(baseCfg())
		if err != nil {
			return fmt.Errorf("png: %w", err)
		}
		if err := os.WriteFile(o.PngOutput, b, 0644); err != nil {
			return err
		}
	}
	if o.SvgOutput != "" {
		if err := qr.SVG(baseCfg(), o.SvgOutput); err != nil {
			return fmt.Errorf("svg: %w", err)
		}
	}
	if o.SvgOptimizedOutput != "" {
		if err := qr.SVG(baseCfg(go_qr.WithOptimalSVG()), o.SvgOptimizedOutput); err != nil {
			return fmt.Errorf("svg-optimized: %w", err)
		}
	}

	if o.Verify {
		b, err := qr.ToPNGBytes(baseCfg())
		if err != nil {
			return fmt.Errorf("verify: render: %w", err)
		}
		if err := verify.RoundTrip(b, text); err != nil {
			return fmt.Errorf("verify: %w", err)
		}
		if !o.Quiet {
			fmt.Fprintln(stderr, "verify: ok")
		}
	}

	if o.Preview {
		fmt.Fprint(stderr, renderPreview(qr))
	}

	noOutputRequested := o.PngOutput == "" && o.SvgOutput == "" && o.SvgOptimizedOutput == ""
	if noOutputRequested && !o.Preview && !o.Verify {
		// Nothing requested — fall back to preview so the command is never silent.
		fmt.Fprint(stderr, renderPreview(qr))
	}

	return nil
}

func runDecode(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("generator decode", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(stderr, `Decode a QR code image (png/jpeg/gif) and print its text.

Usage:
  generator decode [flags] <image-file>

Examples:
  generator decode hello.png
`)
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return errReported
	}

	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("decode: expected exactly one image path, got %d", fs.NArg())
	}

	img, err := loadImage(fs.Arg(0))
	if err != nil {
		return fmt.Errorf("decode: load image: %w", err)
	}
	text, err := verify.Decode(img)
	if err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	fmt.Fprintln(stdout, text)
	return nil
}

// version reports the module version recorded in the build (a tag for
// `go install ...@vX`, or "(devel)" for local builds).
func version() string {
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" {
		return bi.Main.Version
	}
	return "(devel)"
}

// setFlagsAmong returns which of the named flags were explicitly set, as "-name".
func setFlagsAmong(fs *flag.FlagSet, names ...string) []string {
	want := make(map[string]bool, len(names))
	for _, n := range names {
		want[n] = true
	}
	var hit []string
	fs.Visit(func(f *flag.Flag) {
		if want[f.Name] {
			hit = append(hit, "-"+f.Name)
		}
	})
	return hit
}

// resolveContent returns the literal string to encode. When payloadType is set,
// content is interpreted as comma-separated key=val pairs and rendered through
// the payload package into a canonical form.
func resolveContent(content, payloadType string) (string, error) {
	if payloadType == "" {
		return content, nil
	}
	kv, err := parseKV(content)
	if err != nil {
		return "", err
	}
	switch strings.ToLower(payloadType) {
	case "wifi":
		w := payload.WiFi{
			SSID:     kv["ssid"],
			Password: kv["password"],
			Auth:     payload.WiFiAuth(kv["auth"]),
			Hidden:   kv["hidden"] == "true",
		}
		return w.String(), nil
	case "vcard":
		return payload.VCard{
			Name: kv["name"], Phone: kv["phone"], Email: kv["email"],
			URL: kv["url"], Address: kv["address"], Org: kv["org"], Note: kv["note"],
		}.String(), nil
	case "email":
		return payload.Email{To: kv["to"], Subject: kv["subject"], Body: kv["body"]}.String(), nil
	case "sms":
		return payload.SMS{Number: kv["number"], Body: kv["body"]}.String(), nil
	case "tel":
		return payload.Tel{Number: kv["number"]}.String(), nil
	case "geo":
		lat, err := strconv.ParseFloat(kv["lat"], 64)
		if err != nil {
			return "", fmt.Errorf("geo lat %q: %w", kv["lat"], err)
		}
		lon, err := strconv.ParseFloat(kv["lon"], 64)
		if err != nil {
			return "", fmt.Errorf("geo lon %q: %w", kv["lon"], err)
		}
		return payload.Geo{Lat: lat, Lon: lon, Query: kv["query"]}.String(), nil
	case "url":
		return payload.URL{Href: kv["href"]}.String(), nil
	default:
		return "", fmt.Errorf("unknown payload type %q", payloadType)
	}
}

// parseKV parses `key=val,key=val`. Backslashes escape the next character,
// allowing literal commas and equals signs inside values.
func parseKV(s string) (map[string]string, error) {
	out := map[string]string{}
	if s == "" {
		return out, nil
	}
	var key, val strings.Builder
	onKey := true
	flush := func() {
		k := strings.ToLower(strings.TrimSpace(key.String()))
		if k != "" {
			out[k] = val.String()
		}
		key.Reset()
		val.Reset()
		onKey = true
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			i++
			if onKey {
				key.WriteByte(s[i])
			} else {
				val.WriteByte(s[i])
			}
			continue
		}
		switch c {
		case '=':
			if onKey {
				onKey = false
			} else {
				val.WriteByte(c)
			}
		case ',':
			flush()
		default:
			if onKey {
				key.WriteByte(c)
			} else {
				val.WriteByte(c)
			}
		}
	}
	flush()
	return out, nil
}

func parseECC(s string) (go_qr.Ecc, error) {
	switch strings.ToLower(s) {
	case "low", "l":
		return go_qr.Low, nil
	case "medium", "m":
		return go_qr.Medium, nil
	case "quartile", "q":
		return go_qr.Quartile, nil
	case "high", "h":
		return go_qr.High, nil
	default:
		return 0, fmt.Errorf("unknown ecc level %q (expected low|medium|quartile|high)", s)
	}
}

func buildImgOpts(logoPath string, logoRatio float64) ([]go_qr.Option, error) {
	var opts []go_qr.Option
	if logoPath != "" {
		img, err := loadImage(logoPath)
		if err != nil {
			return nil, fmt.Errorf("load logo: %w", err)
		}
		opts = append(opts, go_qr.WithLogo(img, logoRatio))
	}
	return opts, nil
}

func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

func writeStdout(qr *go_qr.QrCode, baseCfg func(...go_qr.Option) *go_qr.QrCodeImgConfig, format string, w io.Writer) error {
	switch strings.ToLower(format) {
	case "png":
		return qr.WriteAsPNG(baseCfg(), w)
	case "svg":
		return qr.WriteAsSVG(baseCfg(), w)
	case "svg-optimized":
		return qr.WriteAsSVG(baseCfg(go_qr.WithOptimalSVG()), w)
	default:
		return fmt.Errorf("unknown stdout format %q (expected png, svg, or svg-optimized)", format)
	}
}

func renderPreview(qr *go_qr.QrCode) string {
	buf := bytes.Buffer{}
	border := 2
	for y := -border; y < qr.Size()+border; y++ {
		for x := -border; x < qr.Size()+border; x++ {
			if qr.Module(x, y) {
				buf.WriteString(blackBlock)
			} else {
				buf.WriteString(whiteBlock)
			}
		}
		buf.WriteString("\n")
	}
	return buf.String()
}
