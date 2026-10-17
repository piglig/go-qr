// Command demo is the in-browser go-qr demo published on GitHub Pages. Built
// for js/wasm it exposes the library to the page in web/; built natively it
// serves that directory for local development:
//
//	cd demo
//	GOOS=js GOARCH=wasm go build -o web/qr.wasm .
//	cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" web/
//	go run .
package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"reflect"
	"strconv"
	"strings"

	"github.com/piglig/go-qr/v2"
	"github.com/piglig/go-qr/v2/payload"
)

// EncodeRequest is what the page sends to encode and render a code.
type EncodeRequest struct {
	Text         string    `json:"text"`
	ECC          string    `json:"ecc"` // L, M, Q or H
	Simple       bool      `json:"simple"`
	UTF8ECI      bool      `json:"utf8eci"`
	ModuleShape  string    `json:"moduleShape"` // square, dot or rounded
	FinderShape  string    `json:"finderShape"` // square, rounded or circle
	Foreground   string    `json:"fg"`          // #rgb, #rrggbb or #rrggbbaa
	Background   string    `json:"bg"`
	Gradient     *Gradient `json:"gradient"`
	FinderRing   string    `json:"finderRing"` // both empty: follow the data color
	FinderCenter string    `json:"finderCenter"`
	Scale        int       `json:"scale"`
	QuietZone    int       `json:"quietZone"`
	Logo         string    `json:"logo"` // data: URL of an image, optional
	LogoRatio    float64   `json:"logoRatio"`
}

// Gradient is a linear gradient over the dark modules.
type Gradient struct {
	From  string  `json:"from"`
	To    string  `json:"to"`
	Angle float64 `json:"angle"`
}

// EncodeResponse carries the rendered code and what the library reports.
type EncodeResponse struct {
	SVG      string    `json:"svg,omitempty"`
	PNG      string    `json:"png,omitempty"` // data: URL
	Version  int       `json:"version,omitempty"`
	ECC      string    `json:"ecc,omitempty"`
	Mask     int       `json:"mask"`
	Size     int       `json:"size,omitempty"`
	Segments []Segment `json:"segments,omitempty"`
	Verify   string    `json:"verify"` // empty when the rendering is readable
	Error    string    `json:"error,omitempty"`
}

// Segment summarizes one segment of a symbol.
type Segment struct {
	Mode  string `json:"mode"`
	Chars int    `json:"chars"`
}

// DecodeResponse is the result of reading a QR Code from an image.
type DecodeResponse struct {
	Text     string    `json:"text,omitempty"`
	Version  int       `json:"version,omitempty"`
	ECC      string    `json:"ecc,omitempty"`
	Mask     int       `json:"mask"`
	Mirrored bool      `json:"mirrored,omitempty"`
	GS1      bool      `json:"gs1,omitempty"`
	Segments []Segment `json:"segments,omitempty"`
	Payload  string    `json:"payload,omitempty"` // payload type, such as "WiFi"
	// ErrorsCorrected, UnusedEC and Soft report the error correction used.
	ErrorsCorrected int     `json:"errorsCorrected"`
	UnusedEC        float64 `json:"unusedEC"`
	Soft            bool    `json:"soft,omitempty"`
	// Quality is the print quality grade, for images other than camera
	// frames.
	Quality *Quality `json:"quality,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// Quality is the grade of a symbol and of each of its parameters.
type Quality struct {
	Grade           string      `json:"grade"`
	Parameters      [][2]string `json:"parameters"` // name and grade
	PixelsPerModule float64     `json:"pixelsPerModule"`
}

var (
	eccLevels    = map[string]qr.ECC{"L": qr.ECCLow, "M": qr.ECCMedium, "Q": qr.ECCQuartile, "H": qr.ECCHigh}
	moduleShapes = map[string]qr.ModuleShape{"square": qr.ModuleSquare, "dot": qr.ModuleDot, "rounded": qr.ModuleRounded}
	finderShapes = map[string]qr.FinderShape{"square": qr.FinderSquare, "rounded": qr.FinderRounded, "circle": qr.FinderCircle}
)

// encode encodes and renders req. Errors are reported in the response.
func encode(req EncodeRequest) EncodeResponse {
	resp, err := encodeErr(req)
	if err != nil {
		return EncodeResponse{Error: strings.TrimPrefix(err.Error(), "qr: ")}
	}
	return resp
}

func encodeErr(req EncodeRequest) (EncodeResponse, error) {
	ecc, ok := eccLevels[req.ECC]
	if !ok {
		return EncodeResponse{}, fmt.Errorf("unknown error correction level %q", req.ECC)
	}
	encOpts := []qr.EncodeOption{qr.WithECC(ecc)}
	if req.Simple {
		encOpts = append(encOpts, qr.WithSimpleSegmentation())
	}
	if req.UTF8ECI {
		encOpts = append(encOpts, qr.WithUTF8ECI())
	}
	code, err := qr.Encode(req.Text, encOpts...)
	if err != nil {
		return EncodeResponse{}, err
	}

	opts, err := renderOptions(req, code.Size())
	if err != nil {
		return EncodeResponse{}, err
	}
	svg, err := code.SVG(opts...)
	if err != nil {
		return EncodeResponse{}, err
	}
	png, err := code.PNG(opts...)
	if err != nil {
		return EncodeResponse{}, err
	}

	resp := EncodeResponse{
		SVG:     string(svg),
		PNG:     "data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
		Version: code.Version(),
		ECC:     code.ECC().String(),
		Mask:    code.Mask(),
		Size:    code.Size(),
	}
	if err := code.Verify(opts...); err != nil {
		resp.Verify = strings.TrimPrefix(err.Error(), "qr: ")
	}
	// Report the segments the encoder chose by reading the plain symbol.
	if img, err := code.Image(qr.WithScale(1)); err == nil {
		if res, err := qr.Decode(img, qr.WithFastPathOnly()); err == nil {
			resp.Segments = segments(res)
		}
	}
	return resp, nil
}

func renderOptions(req EncodeRequest, qrSize int) ([]qr.RenderOption, error) {
	var opts []qr.RenderOption
	if req.Scale > 0 {
		opts = append(opts, qr.WithScale(req.Scale))
	}
	opts = append(opts, qr.WithQuietZone(req.QuietZone))
	if s, ok := moduleShapes[req.ModuleShape]; ok {
		opts = append(opts, qr.WithModuleShape(s))
	} else if req.ModuleShape != "" {
		return nil, fmt.Errorf("unknown module shape %q", req.ModuleShape)
	}
	if s, ok := finderShapes[req.FinderShape]; ok {
		opts = append(opts, qr.WithFinderShape(s))
	} else if req.FinderShape != "" {
		return nil, fmt.Errorf("unknown finder shape %q", req.FinderShape)
	}

	for _, c := range []struct {
		hex string
		opt func(color.Color) qr.RenderOption
	}{{req.Foreground, qr.WithForeground}, {req.Background, qr.WithBackground}} {
		if c.hex == "" {
			continue
		}
		col, err := parseHex(c.hex)
		if err != nil {
			return nil, err
		}
		opts = append(opts, c.opt(col))
	}
	if g := req.Gradient; g != nil {
		from, err1 := parseHex(g.From)
		to, err2 := parseHex(g.To)
		if err := errors.Join(err1, err2); err != nil {
			return nil, err
		}
		opts = append(opts, qr.WithGradient(from, to, g.Angle))
	}
	if req.FinderRing != "" || req.FinderCenter != "" {
		ring, err1 := parseHex(req.FinderRing)
		center, err2 := parseHex(req.FinderCenter)
		if err := errors.Join(err1, err2); err != nil {
			return nil, err
		}
		opts = append(opts, qr.WithFinderColor(ring, center))
	}
	if req.Logo != "" {
		logo, err := loadLogo(req.Logo, logoSide(qrSize, req))
		if err != nil {
			return nil, fmt.Errorf("logo: %w", err)
		}
		opts = append(opts, qr.WithLogo(logo, req.LogoRatio))
	}
	return opts, nil
}

// parseHex parses #rgb, #rrggbb or #rrggbbaa.
func parseHex(s string) (color.Color, error) {
	h := strings.TrimPrefix(s, "#")
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	if len(h) == 6 {
		h += "ff"
	}
	v, err := strconv.ParseUint(h, 16, 32)
	if len(h) != 8 || err != nil {
		return nil, fmt.Errorf("invalid color %q", s)
	}
	return color.NRGBA{R: uint8(v >> 24), G: uint8(v >> 16), B: uint8(v >> 8), A: uint8(v)}, nil
}

// logoCache keeps the last logo, decoded and scaled, so that changing the
// style does not decode and scale the uploaded image again on every render.
var logoCache struct {
	url  string
	side int
	img  image.Image
}

// logoSide returns the side, in pixels, of the area qr.WithLogo draws the
// logo into: the logo box without its one-module padding. It returns 0 when
// the request leaves the scale to the library's default.
func logoSide(qrSize int, req EncodeRequest) int {
	if req.Scale < 1 {
		return 0
	}
	n := int(float64(qrSize) * req.LogoRatio)
	if n%2 != qrSize%2 {
		n--
	}
	return max(n, 1) * req.Scale
}

// loadLogo decodes the data URL and, when side is not 0, scales the image
// down to side×side pixels, the size it is drawn at. Embedded in the SVG, a logo at its uploaded size
// would be re-encoded as PNG on every render.
func loadLogo(url string, side int) (image.Image, error) {
	if logoCache.url == url && logoCache.side == side {
		return logoCache.img, nil
	}
	img, err := decodeDataURL(url)
	if err != nil {
		return nil, err
	}
	if b := img.Bounds(); side > 0 && (b.Dx() > side || b.Dy() > side) {
		img = shrink(img, side)
	}
	logoCache.url, logoCache.side, logoCache.img = url, side, img
	return img, nil
}

// shrink scales src down to side×side pixels by averaging the source pixels
// under each output pixel.
func shrink(src image.Image, side int) *image.RGBA {
	b := src.Bounds()
	in := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(in, in.Bounds(), src, b.Min, draw.Src)
	out := image.NewRGBA(image.Rect(0, 0, side, side))
	for y := range side {
		y0, y1 := y*b.Dy()/side, max((y+1)*b.Dy()/side, y*b.Dy()/side+1)
		for x := range side {
			x0, x1 := x*b.Dx()/side, max((x+1)*b.Dx()/side, x*b.Dx()/side+1)
			var sum [4]int
			for sy := y0; sy < y1; sy++ {
				row := in.Pix[sy*in.Stride+4*x0 : sy*in.Stride+4*x1]
				for i := 0; i < len(row); i += 4 {
					sum[0] += int(row[i])
					sum[1] += int(row[i+1])
					sum[2] += int(row[i+2])
					sum[3] += int(row[i+3])
				}
			}
			n := (y1 - y0) * (x1 - x0)
			o := out.PixOffset(x, y)
			for c := range sum {
				out.Pix[o+c] = uint8(sum[c] / n)
			}
		}
	}
	return out
}

// decodeDataURL decodes a base64 data: URL holding a PNG, JPEG or GIF.
func decodeDataURL(s string) (image.Image, error) {
	_, data, ok := strings.Cut(s, ";base64,")
	if !ok || !strings.HasPrefix(s, "data:") {
		return nil, errors.New("not a base64 data URL")
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	return img, err
}

// decodeRGBA reads a QR Code from non-premultiplied RGBA pixels, as returned
// by a canvas's getImageData. A frame of the camera is searched for dark
// codes on light only, and not graded.
func decodeRGBA(pix []byte, w, h int, frame bool) DecodeResponse {
	if w <= 0 || h <= 0 || len(pix) != w*h*4 {
		return DecodeResponse{Error: fmt.Sprintf("bad image data: %d bytes for %dx%d", len(pix), w, h)}
	}
	img := &image.NRGBA{Pix: pix, Stride: 4 * w, Rect: image.Rect(0, 0, w, h)}
	var opts []qr.DecodeOption
	if frame {
		opts = append(opts, qr.WithoutInverted())
	}
	res, err := qr.Decode(img, opts...)
	if err != nil {
		return DecodeResponse{Error: strings.TrimPrefix(err.Error(), "qr: ")}
	}
	out := DecodeResponse{
		Text:            res.Text,
		Version:         res.Version,
		ECC:             res.ECC.String(),
		Mask:            res.Mask,
		Mirrored:        res.Mirrored,
		GS1:             res.GS1,
		Segments:        segments(res),
		ErrorsCorrected: res.ErrorsCorrected,
		UnusedEC:        res.UnusedErrorCorrection,
		Soft:            res.SoftDecision,
	}
	if p, err := payload.Parse(res.Text); err == nil {
		out.Payload = reflect.TypeOf(p).Name()
	}
	if !frame {
		if q, err := qr.Grade(img); err == nil {
			out.Quality = quality(q)
		}
	}
	return out
}

func quality(q *qr.Quality) *Quality {
	return &Quality{
		Grade: q.Grade.String(),
		Parameters: [][2]string{
			{"Symbol contrast", q.SymbolContrast.Grade.String()},
			{"Modulation", q.Modulation.Grade.String()},
			{"Reflectance margin", q.ReflectanceMargin.Grade.String()},
			{"Fixed pattern damage", q.FixedPatternDamage.Grade.String()},
			{"Axial nonuniformity", q.AxialNonuniformity.Grade.String()},
			{"Grid nonuniformity", q.GridNonuniformity.Grade.String()},
			{"Unused error correction", q.UnusedErrorCorrection.Grade.String()},
		},
		PixelsPerModule: q.PixelsPerModule,
	}
}

func segments(res *qr.DecodeResult) []Segment {
	out := make([]Segment, 0, len(res.Segments))
	for _, s := range res.Segments {
		out = append(out, Segment{Mode: s.Mode.String(), Chars: s.NumChars})
	}
	return out
}
