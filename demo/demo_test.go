package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"
	"testing"
)

func TestEncodeDefaults(t *testing.T) {
	resp := encode(EncodeRequest{Text: "https://example.com 123456789", ECC: "M"})
	if resp.Error != "" {
		t.Fatal(resp.Error)
	}
	if !strings.HasPrefix(resp.SVG, "<svg") || !strings.HasPrefix(resp.PNG, "data:image/png;base64,") {
		t.Fatalf("missing outputs: %.40q %.40q", resp.SVG, resp.PNG)
	}
	if resp.Verify != "" || resp.Version == 0 || resp.Size != 4*resp.Version+17 || len(resp.Segments) == 0 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestEncodeStyled(t *testing.T) {
	resp := encode(EncodeRequest{
		Text: "styled", ECC: "Q", ModuleShape: "rounded", FinderShape: "circle",
		Gradient:   &Gradient{From: "#1a237e", To: "#00695c", Angle: 45},
		FinderRing: "#c62828", FinderCenter: "#000",
		Scale: 6, QuietZone: 2,
	})
	if resp.Error != "" || resp.Verify != "" {
		t.Fatalf("error %q, verify %q", resp.Error, resp.Verify)
	}
	if !strings.Contains(resp.SVG, "<linearGradient") || !strings.Contains(resp.SVG, `fill="#C62828"`) {
		t.Fatal("styled SVG lacks the gradient or finder color")
	}
}

func TestEncodeReportsProblems(t *testing.T) {
	for name, req := range map[string]EncodeRequest{
		"bad ECC":    {Text: "x", ECC: "Z"},
		"bad color":  {Text: "x", ECC: "L", Foreground: "#zzz"},
		"bad shape":  {Text: "x", ECC: "L", ModuleShape: "star"},
		"bad logo":   {Text: "x", ECC: "L", Logo: "not a url", LogoRatio: 0.2},
		"too long":   {Text: strings.Repeat("x", 4000), ECC: "H"},
		"bad finder": {Text: "x", ECC: "L", FinderRing: "#000"},
	} {
		if encode(req).Error == "" {
			t.Errorf("%s: no error", name)
		}
	}
	// Unreadable styles render but carry a Verify message.
	resp := encode(EncodeRequest{Text: "x", ECC: "L", Foreground: "#ccc"})
	if resp.Error != "" || resp.Verify == "" {
		t.Fatalf("low contrast: error %q, verify %q", resp.Error, resp.Verify)
	}
}

func TestEncodeLogo(t *testing.T) {
	logo := image.NewRGBA(image.Rect(0, 0, 8, 8))
	draw.Draw(logo, logo.Bounds(), image.Black, image.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, logo); err != nil {
		t.Fatal(err)
	}
	url := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	resp := encode(EncodeRequest{Text: "https://example.com", ECC: "H", Logo: url, LogoRatio: 0.2})
	if resp.Error != "" || resp.Verify != "" || !strings.Contains(resp.SVG, "<image") {
		t.Fatalf("error %q, verify %q", resp.Error, resp.Verify)
	}
}

func TestParseHex(t *testing.T) {
	for in, want := range map[string][4]uint32{
		"#000":      {0, 0, 0, 0xffff},
		"#ff8000":   {0xffff, 0x8080, 0, 0xffff},
		"#ffffff00": {0, 0, 0, 0},
	} {
		c, err := parseHex(in)
		if err != nil {
			t.Fatal(err)
		}
		r, g, b, a := c.RGBA()
		if [4]uint32{r, g, b, a} != want {
			t.Errorf("%s = %v, want %v", in, [4]uint32{r, g, b, a}, want)
		}
	}
	for _, bad := range []string{"", "#12", "#gggggg", "#1234567"} {
		if _, err := parseHex(bad); err == nil {
			t.Errorf("parseHex(%q) succeeded", bad)
		}
	}
}

// TestDecodeRGBA decodes the way the page does: from a canvas's
// non-premultiplied RGBA bytes.
func TestDecodeRGBA(t *testing.T) {
	resp := encode(EncodeRequest{Text: "WIFI:T:WPA;S:home;P:pw;;", ECC: "M", ModuleShape: "dot", Scale: 4})
	raw, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(resp.PNG, "data:image/png;base64,"))
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	nrgba := image.NewNRGBA(img.Bounds())
	draw.Draw(nrgba, nrgba.Bounds(), img, image.Point{}, draw.Src)

	got := decodeRGBA(nrgba.Pix, nrgba.Bounds().Dx(), nrgba.Bounds().Dy())
	if got.Error != "" || got.Text != "WIFI:T:WPA;S:home;P:pw;;" || got.Payload != "WiFi" {
		t.Fatalf("decode: %+v", got)
	}
	if bad := decodeRGBA(make([]byte, 10), 5, 5); bad.Error == "" {
		t.Error("short pixel buffer accepted")
	}
	if blank := decodeRGBA(make([]byte, 4*40*40), 40, 40); blank.Error == "" {
		t.Error("blank image decoded")
	}
}

// TestEncodeLogoScaled checks that a large logo is scaled down to the size it
// is drawn at before it is embedded in the SVG, and that the scaled logo is
// reused while only the style changes.
func TestEncodeLogoScaled(t *testing.T) {
	logo := image.NewRGBA(image.Rect(0, 0, 1200, 900))
	draw.Draw(logo, logo.Bounds(), image.NewUniform(color.RGBA{R: 200, A: 255}), image.Point{}, draw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, logo); err != nil {
		t.Fatal(err)
	}
	url := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	req := EncodeRequest{Text: "https://example.com", ECC: "H", Scale: 10, QuietZone: 4, Logo: url, LogoRatio: 0.2}
	resp := encode(req)
	if resp.Error != "" || resp.Verify != "" {
		t.Fatalf("error %q, verify %q", resp.Error, resp.Verify)
	}

	side := logoSide(resp.Size, req)
	_, href, _ := strings.Cut(resp.SVG, `href="data:image/png;base64,`)
	href, _, _ = strings.Cut(href, `"`)
	raw, err := base64.StdEncoding.DecodeString(href)
	if err != nil {
		t.Fatal(err)
	}
	embedded, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	if b := embedded.Bounds(); b.Dx() != side || b.Dy() != side {
		t.Fatalf("embedded logo is %v, want %d×%d", b.Size(), side, side)
	}
	if r, _, _, _ := embedded.At(side/2, side/2).RGBA(); r>>8 != 200 {
		t.Errorf("scaled logo red = %d, want 200", r>>8)
	}
	if !strings.Contains(resp.SVG, fmt.Sprintf(`width="%d"`, side)) {
		t.Errorf("SVG does not draw the logo %d units wide", side)
	}

	cached := logoCache.img
	req.ModuleShape, req.FinderShape = "dot", "circle"
	if resp := encode(req); resp.Error != "" {
		t.Fatal(resp.Error)
	}
	if logoCache.img != cached {
		t.Error("changing the style decoded the logo again")
	}
}

func TestShrink(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 4, 2))
	for x := 0; x < 4; x++ {
		c := color.NRGBA{A: 255}
		if x%2 == 0 {
			c = color.NRGBA{R: 255, G: 255, B: 255, A: 255}
		}
		src.Set(x, 0, c)
		src.Set(x, 1, c)
	}
	got := shrink(src, 2)
	for _, p := range [][2]int{{0, 0}, {1, 1}} {
		if c := got.RGBAAt(p[0], p[1]); c != (color.RGBA{R: 127, G: 127, B: 127, A: 255}) {
			t.Errorf("pixel %v = %v, want the average gray", p, c)
		}
	}
	// A dimension smaller than the target is stretched, not dropped.
	if got := shrink(image.NewRGBA(image.Rect(0, 0, 10, 1)), 4); got.Bounds().Dx() != 4 || got.Bounds().Dy() != 4 {
		t.Errorf("shrink of 10×1 to 4 = %v", got.Bounds())
	}
}
