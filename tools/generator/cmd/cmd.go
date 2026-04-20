// Package cmd implements the `generator` CLI.
package cmd

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"strings"

	go_qr "github.com/piglig/go-qr"
	"github.com/piglig/go-qr/payload"
	"github.com/piglig/go-qr/tools/verify"
)

const (
	blackBlock = "\033[40m  \033[0m"
	whiteBlock = "\033[47m  \033[0m"
)

type Command struct {
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

func newCommand(args []string) (*Command, error) {
	fs := flag.NewFlagSet("generator", flag.ContinueOnError)
	cmd := &Command{}
	fs.StringVar(&cmd.Content, "content", "", "Content to encode (raw text, or key=val,... when -payload is set)")
	fs.StringVar(&cmd.Payload, "payload", "", "Structured payload type: wifi, vcard, email, sms, tel, geo, url")
	fs.StringVar(&cmd.ECC, "ecc", "high", "Error correction: low, medium, quartile, high")
	fs.IntVar(&cmd.Scale, "scale", 10, "Scale (pixels per module for PNG / units per module for SVG)")
	fs.IntVar(&cmd.Border, "border", 4, "Quiet-zone border, in modules")
	fs.StringVar(&cmd.PngOutput, "png", "", "Output PNG file path")
	fs.StringVar(&cmd.SvgOutput, "svg", "", "Output SVG file path")
	fs.StringVar(&cmd.SvgOptimizedOutput, "svg-optimized", "", "Output optimized SVG file path")
	fs.StringVar(&cmd.Stdout, "stdout", "", "Write to stdout instead of files: png, svg, or svg-optimized")
	fs.StringVar(&cmd.Logo, "logo", "", "Path to a logo image (png/jpeg/gif) to embed in the center")
	fs.Float64Var(&cmd.LogoRatio, "logo-ratio", 0.2, "Logo side length as fraction of QR module-area side")
	fs.BoolVar(&cmd.Verify, "verify", false, "Decode the generated PNG and assert it matches the input (exit 1 on mismatch)")
	fs.BoolVar(&cmd.Preview, "preview", false, "Print ANSI terminal preview to stderr")
	fs.BoolVar(&cmd.Quiet, "quiet", false, "Suppress non-error output")

	if err := fs.Parse(args); err != nil {
		return nil, err
	}
	return cmd, nil
}

// Exec is the CLI entrypoint.
func Exec() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	cmd, err := newCommand(args)
	if err != nil {
		return err
	}
	if cmd.Content == "" {
		return fmt.Errorf("please provide content to encode via -content")
	}

	text, err := resolveContent(cmd)
	if err != nil {
		return fmt.Errorf("payload: %w", err)
	}

	ecl, err := parseECC(cmd.ECC)
	if err != nil {
		return err
	}

	qr, err := go_qr.EncodeText(text, ecl)
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}

	imgOpts, err := buildImgOpts(cmd)
	if err != nil {
		return err
	}

	baseCfg := func(extra ...func(*go_qr.QrCodeImgConfig)) *go_qr.QrCodeImgConfig {
		all := append([]func(*go_qr.QrCodeImgConfig){}, imgOpts...)
		all = append(all, extra...)
		return go_qr.NewQrCodeImgConfig(cmd.Scale, cmd.Border, all...)
	}

	if cmd.Stdout != "" {
		return writeStdout(qr, baseCfg, cmd.Stdout, stdout)
	}

	if cmd.PngOutput != "" {
		b, err := qr.ToPNGBytes(baseCfg())
		if err != nil {
			return fmt.Errorf("png: %w", err)
		}
		if err := os.WriteFile(cmd.PngOutput, b, 0644); err != nil {
			return err
		}
	}
	if cmd.SvgOutput != "" {
		if err := qr.SVG(baseCfg(), cmd.SvgOutput); err != nil {
			return fmt.Errorf("svg: %w", err)
		}
	}
	if cmd.SvgOptimizedOutput != "" {
		if err := qr.SVG(baseCfg(go_qr.WithOptimalSVG()), cmd.SvgOptimizedOutput); err != nil {
			return fmt.Errorf("svg-optimized: %w", err)
		}
	}

	if cmd.Verify {
		b, err := qr.ToPNGBytes(baseCfg())
		if err != nil {
			return fmt.Errorf("verify: render: %w", err)
		}
		if err := verify.RoundTrip(b, text); err != nil {
			return fmt.Errorf("verify: %w", err)
		}
		if !cmd.Quiet {
			fmt.Fprintln(stderr, "verify: ok")
		}
	}

	if cmd.Preview {
		fmt.Fprint(stderr, renderPreview(qr))
	}

	noOutputRequested := cmd.PngOutput == "" && cmd.SvgOutput == "" && cmd.SvgOptimizedOutput == ""
	if noOutputRequested && !cmd.Preview && !cmd.Verify {
		// No output requested — fall back to preview so the command is never silent.
		fmt.Fprint(stderr, renderPreview(qr))
	}

	return nil
}

// resolveContent returns the literal string to encode. When -payload is set,
// the content is interpreted as comma-separated key=val pairs and rendered
// through the payload package into a canonical form.
func resolveContent(cmd *Command) (string, error) {
	if cmd.Payload == "" {
		return cmd.Content, nil
	}
	kv, err := parseKV(cmd.Content)
	if err != nil {
		return "", err
	}
	switch strings.ToLower(cmd.Payload) {
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
		var lat, lon float64
		fmt.Sscanf(kv["lat"], "%f", &lat)
		fmt.Sscanf(kv["lon"], "%f", &lon)
		return payload.Geo{Lat: lat, Lon: lon, Query: kv["query"]}.String(), nil
	case "url":
		return payload.URL{Href: kv["href"]}.String(), nil
	default:
		return "", fmt.Errorf("unknown payload type %q", cmd.Payload)
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

func buildImgOpts(cmd *Command) ([]func(*go_qr.QrCodeImgConfig), error) {
	var opts []func(*go_qr.QrCodeImgConfig)
	if cmd.Logo != "" {
		img, err := loadImage(cmd.Logo)
		if err != nil {
			return nil, fmt.Errorf("load logo: %w", err)
		}
		opts = append(opts, go_qr.WithLogo(img, cmd.LogoRatio))
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

func writeStdout(qr *go_qr.QrCode, baseCfg func(...func(*go_qr.QrCodeImgConfig)) *go_qr.QrCodeImgConfig, format string, w io.Writer) error {
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
	for y := -border; y < qr.GetSize()+border; y++ {
		for x := -border; x < qr.GetSize()+border; x++ {
			if qr.GetModule(x, y) {
				buf.WriteString(blackBlock)
			} else {
				buf.WriteString(whiteBlock)
			}
		}
		buf.WriteString("\n")
	}
	return buf.String()
}
