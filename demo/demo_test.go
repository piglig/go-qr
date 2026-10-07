package main

import (
	"bytes"
	"encoding/base64"
	"image"
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
