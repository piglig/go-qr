package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/piglig/go-qr/v2"
	"github.com/piglig/go-qr/v2/payload"
)

// encodeToPNG runs encode with args and -stdout png, and decodes the image.
func encodeToPNG(t *testing.T, args ...string) *qr.DecodeResult {
	t.Helper()
	var out, errOut bytes.Buffer
	full := append([]string{"encode", "-stdout", "png"}, args...)
	if err := run(full, &out, &errOut); err != nil {
		t.Fatalf("run %v: %v (stderr %q)", full, err, errOut.String())
	}
	img, err := png.Decode(bytes.NewReader(out.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	res, err := qr.Decode(img)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return res
}

func TestEncodeOptionsFlags(t *testing.T) {
	res := encodeToPNG(t, "-ecc", "low", "-no-boost", "-mask", "5", "-min-version", "3", "hello")
	if res.ECC != qr.ECCLow || res.Mask != 5 || res.Version != 3 || res.Text != "hello" {
		t.Fatalf("got %+v", res)
	}

	res = encodeToPNG(t, "-utf8-eci", "héllo")
	if res.Segments[0].Mode != qr.ModeECI || res.Segments[0].ECI != 26 || res.Text != "héllo" {
		t.Fatalf("utf8-eci: %+v", res.Segments)
	}

	gs1 := "0109501101530003" + "10ABC\x1d" + "21XYZ"
	res = encodeToPNG(t, "-gs1", "-content", gs1)
	if !res.GS1 || res.Text != gs1 {
		t.Fatalf("gs1: %+v", res)
	}
}

func TestEncodeStyleFlags(t *testing.T) {
	for _, args := range [][]string{
		{"-module", "dot", "-finder", "circle"},
		{"-module", "rounded", "-finder", "rounded", "-gradient", "#1a237e,#00695c,45"},
		{"-fg", "#1a237e", "-bg", "#fffde7", "-finder-color", "#c62828,#000"},
		{"-bg", "transparent", "-finder-color", "#4a148c"},
	} {
		var out, errOut bytes.Buffer
		full := append([]string{"encode", "-verify", "-stdout", "svg"}, append(args, "styled")...)
		if err := run(full, &out, &errOut); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		if !strings.HasPrefix(out.String(), "<svg") {
			t.Fatalf("%v: no SVG on stdout", args)
		}
	}
	if res := encodeToPNG(t, "-module", "dot", "-gradient", "#1a237e,#00695c", "styled png"); res.Text != "styled png" {
		t.Fatalf("styled PNG decoded as %q", res.Text)
	}
}

func TestEncodeStyleErrors(t *testing.T) {
	for _, args := range [][]string{
		{"-module", "star"},
		{"-finder", "hexagon"},
		{"-fg", "#12"},
		{"-bg", "blue"},
		{"-gradient", "#000"},
		{"-gradient", "#000,#111,sideways"},
		{"-finder-color", "#000,#111,#222"},
	} {
		var out, errOut bytes.Buffer
		if err := run(append([]string{"encode", "-stdout", "png"}, append(args, "x")...), &out, &errOut); err == nil {
			t.Errorf("%v: no error", args)
		}
	}
}

func TestVerifyRejectsLowContrast(t *testing.T) {
	var out, errOut bytes.Buffer
	err := run([]string{"encode", "-verify", "-fg", "#cccccc", "-png", filepath.Join(t.TempDir(), "x.png"), "x"}, &out, &errOut)
	if !errors.Is(err, qr.ErrUnreadable) {
		t.Fatalf("error = %v, want ErrUnreadable", err)
	}
}

func TestEncodeStructuredFiles(t *testing.T) {
	dir := t.TempDir()
	text := strings.Repeat("structured append across several symbols 0123456789. ", 12)
	var out, errOut bytes.Buffer
	err := run([]string{"encode", "-structured", "-ecc", "low", "-max-version", "5", "-verify",
		"-png", filepath.Join(dir, "part.png"), "-svg", filepath.Join(dir, "part.svg"), "-content", text}, &out, &errOut)
	if err != nil {
		t.Fatal(err)
	}
	pngs, _ := filepath.Glob(filepath.Join(dir, "part-*.png"))
	svgs, _ := filepath.Glob(filepath.Join(dir, "part-*.svg"))
	if len(pngs) < 2 || len(pngs) != len(svgs) {
		t.Fatalf("got %d PNGs and %d SVGs", len(pngs), len(svgs))
	}
	assertContains(t, errOut.String(), "wrote ")

	// decode joins the sequence, given in any order.
	out.Reset()
	reversed := make([]string, len(pngs))
	for i, p := range pngs {
		reversed[len(pngs)-1-i] = p
	}
	if err := run(append([]string{"decode"}, reversed...), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSuffix(out.String(), "\n"); got != text {
		t.Fatalf("joined text = %q", got)
	}

	// Missing a part is an error.
	if err := run(append([]string{"decode"}, pngs[1:]...), &out, &errOut); err == nil {
		t.Fatal("incomplete sequence decoded without error")
	}

	// A sequence cannot go to -stdout png.
	if err := run([]string{"encode", "-structured", "-ecc", "low", "-max-version", "5", "-stdout", "png", "-content", text}, &out, &errOut); err == nil {
		t.Fatal("-stdout png accepted several symbols")
	}
}

func TestEncodeStructuredSingleSymbolKeepsName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "one.png")
	var out, errOut bytes.Buffer
	if err := run([]string{"encode", "-structured", "-png", path, "short"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestPayloadTypes(t *testing.T) {
	tests := []struct {
		kind, content string
		want          payload.Payload
	}{
		{"contact", "name=Jane Doe,given=Jane,family=Doe,phone=+1 555;+1 556,email=j@x.test,org=ACME",
			payload.Contact{Name: "Jane Doe", GivenName: "Jane", FamilyName: "Doe", Phones: []string{"+1 555", "+1 556"}, Emails: []string{"j@x.test"}, Org: "ACME"}},
		{"otp", "issuer=Example,account=alice@example.com,secret=JBSWY3DPEHPK3PXP,digits=8",
			payload.OTP{Type: payload.TOTP, Issuer: "Example", Account: "alice@example.com", Secret: "JBSWY3DPEHPK3PXP", Digits: 8}},
		{"otp", "type=hotp,account=bob,secret=ABC,counter=7",
			payload.OTP{Type: payload.HOTP, Account: "bob", Secret: "ABC", Counter: 7}},
		{"epc", "name=Red Cross,iban=BE72 0000 0000 1616,amount=25.5,text=Donation",
			payload.EPC{Name: "Red Cross", IBAN: "BE72000000001616", Amount: 2550, Text: "Donation"}},
		{"email", "to=a@b.c,subject=Hi,cc=c@d.e;f@g.h",
			payload.Email{To: "a@b.c", Subject: "Hi", CC: []string{"c@d.e", "f@g.h"}}},
	}
	for _, tt := range tests {
		res := encodeToPNG(t, "-payload", tt.kind, "-content", tt.content)
		got, err := payload.Parse(res.Text)
		if err != nil {
			t.Fatalf("%s: %v", tt.kind, err)
		}
		if !jsonEqual(got, tt.want) {
			t.Errorf("%s:\n got %#v\nwant %#v", tt.kind, got, tt.want)
		}
	}
}

func TestPayloadEvent(t *testing.T) {
	res := encodeToPNG(t, "-payload", "event", "-content", "summary=Launch,start=2026-10-07T18:00,end=2026-10-07T21:00:00Z")
	assertContains(t, res.Text, "DTSTART:20261007T180000Z")
	assertContains(t, res.Text, "DTEND:20261007T210000Z")

	res = encodeToPNG(t, "-payload", "event", "-content", "summary=Holiday,start=2026-10-01")
	assertContains(t, res.Text, "DTSTART;VALUE=DATE:20261001")
}

func TestPayloadErrors(t *testing.T) {
	for kind, content := range map[string]string{
		"event": "summary=x",
		"otp":   "issuer=x",
		"epc":   "name=x,iban=DE89370400440532013000,amount=1.234",
		"bogus": "a=b",
	} {
		var out, errOut bytes.Buffer
		args := []string{"encode", "-stdout", "png", "-payload", kind, "-content", content + ",x=y"}
		if err := run(args, &out, &errOut); err == nil {
			t.Errorf("%s %q: no error", kind, content)
		}
	}
	for _, s := range []string{"1.234", "-1", "abc", "1,5"} {
		if _, err := parseEuros(s); err == nil {
			t.Errorf("parseEuros(%q) succeeded", s)
		}
	}
	if c, err := parseEuros("12"); err != nil || c != 1200 {
		t.Errorf("parseEuros(12) = %d, %v", c, err)
	}
}

func TestDecodeJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "wifi.png")
	var out, errOut bytes.Buffer
	if err := run([]string{"encode", "-payload", "wifi", "-png", path, "ssid=home,password=pw"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := run([]string{"decode", "-json", path}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Text    string          `json:"text"`
		Symbols []decodedSymbol `json:"symbols"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("%v in %s", err, out.String())
	}
	if got.Text != "WIFI:T:WPA;S:home;P:pw;;" || len(got.Symbols) != 1 {
		t.Fatalf("got %+v", got)
	}
	s := got.Symbols[0]
	if s.Payload != "WiFi" || s.Version == 0 || s.ECC != "H" || s.File != path || len(s.Segments) == 0 {
		t.Fatalf("symbol %+v", s)
	}
}

func TestDecodeSeveralUnrelated(t *testing.T) {
	dir := t.TempDir()
	var paths []string
	for _, text := range []string{"first", "second"} {
		p := filepath.Join(dir, text+".png")
		var out, errOut bytes.Buffer
		if err := run([]string{"encode", "-png", p, text}, &out, &errOut); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	var out, errOut bytes.Buffer
	if err := run(append([]string{"decode"}, paths...), &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if out.String() != "first\nsecond\n" {
		t.Fatalf("output %q", out.String())
	}
}

func jsonEqual(a, b any) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return bytes.Equal(x, y)
}

func assertContains(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Fatalf("%q does not contain %q", s, sub)
	}
}

func TestGS1FromHRI(t *testing.T) {
	got, err := gs1FromHRI("(01)09501101530003(17)250101(10)ABC123(21)XYZ")
	if err != nil {
		t.Fatal(err)
	}
	// (01) and (17) have predefined lengths; (10) is variable and not last.
	if want := "0109501101530003" + "17250101" + "10ABC123\x1d" + "21XYZ"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got, _ := gs1FromHRI("0109501101530003"); got != "0109501101530003" {
		t.Fatalf("raw element string changed: %q", got)
	}
	for _, bad := range []string{"(01", "(0A)123", "(01)", "(01)123x(", "(1)2"} {
		if _, err := gs1FromHRI(bad); err == nil {
			t.Errorf("gs1FromHRI(%q) succeeded", bad)
		}
	}
	res := encodeToPNG(t, "-gs1", "(01)09501101530003(10)ABC(21)XYZ")
	if !res.GS1 || res.Text != "0109501101530003"+"10ABC\x1d"+"21XYZ" {
		t.Fatalf("decoded %+v", res)
	}
}

func TestPayloadEPCRejectsBadIBAN(t *testing.T) {
	var out, errOut bytes.Buffer
	args := []string{"encode", "-stdout", "png", "-payload", "epc", "-content", "name=x,iban=DE88370400440532013000"}
	err := run(args, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "check digits") {
		t.Fatalf("error = %v, want the IBAN check digits to be rejected", err)
	}
	if strings.Count(err.Error(), "payload:") != 1 {
		t.Errorf("error = %q, want a single \"payload:\" prefix", err)
	}
	err = run([]string{"encode", "-stdout", "png", "-payload", "otp", "-content", "issuer=x"}, &out, &errOut)
	if err == nil || err.Error() != "payload: otp: secret is required" {
		t.Errorf("error = %v, want CLI errors prefixed once", err)
	}
}

// writeSheet writes a PNG of the codes side by side and returns its path.
func writeSheet(t *testing.T, codes []*qr.Code) string {
	t.Helper()
	sheet := image.NewRGBA(image.Rect(0, 0, 220*len(codes), 220))
	draw.Draw(sheet, sheet.Bounds(), image.White, image.Point{}, draw.Src)
	for i, code := range codes {
		img, err := code.Image(qr.WithScale(5))
		if err != nil {
			t.Fatal(err)
		}
		draw.Draw(sheet, img.Bounds().Add(image.Pt(i*220, 0)), img, image.Point{}, draw.Src)
	}
	path := filepath.Join(t.TempDir(), "sheet.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, sheet); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDecodeAll(t *testing.T) {
	var codes []*qr.Code
	for _, text := range []string{"left", "middle", "right"} {
		code, _ := qr.Encode(text)
		codes = append(codes, code)
	}
	path := writeSheet(t, codes)

	var out, errOut bytes.Buffer
	if err := run([]string{"decode", "-all", path}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if out.String() != "left\nmiddle\nright\n" {
		t.Fatalf("-all output %q", out.String())
	}
	// Without -all, one code is read.
	out.Reset()
	if err := run([]string{"decode", path}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "\n") != 1 {
		t.Fatalf("output %q", out.String())
	}

	out.Reset()
	if err := run([]string{"decode", "-all", "-json", path}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Text    *string         `json:"text"`
		Symbols []decodedSymbol `json:"symbols"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("%v in %s", err, out.String())
	}
	if got.Text != nil || len(got.Symbols) != 3 {
		t.Fatalf("got %+v", got)
	}
	// The middle symbol's top-left corner is 220 px from the left one's, past
	// a 4-module quiet zone of 5 px modules.
	if s := got.Symbols[1]; s.File != path || s.Text != "middle" || s.Corners[0] != [2]int{240, 20} {
		t.Fatalf("symbol %+v", s)
	}
}

// TestDecodeAllSequence joins a structured append sequence photographed in
// one image.
func TestDecodeAllSequence(t *testing.T) {
	text := strings.Repeat("A message split over several symbols. ", 3)
	codes, err := qr.EncodeStructured(text, qr.WithECC(qr.ECCLow), qr.WithVersionRange(1, 2))
	if err != nil || len(codes) < 2 {
		t.Fatalf("%d codes, %v", len(codes), err)
	}
	path := writeSheet(t, codes)
	var out, errOut bytes.Buffer
	if err := run([]string{"decode", "-all", path}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if out.String() != text+"\n" {
		t.Fatalf("output %q", out.String())
	}
}

// TestDecodeGrade grades each code of a sheet in its own window: a code
// with a logo grades lower on its unused error correction, the plain ones
// grade A.
func TestDecodeGrade(t *testing.T) {
	plain, _ := qr.Encode("plain", qr.WithECC(qr.ECCHigh))
	logo, _ := qr.Encode("logo", qr.WithECC(qr.ECCHigh))
	path := writeSheet(t, []*qr.Code{plain, logo, plain})
	// Draw a logo over the middle code, as WithLogo would.
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	src, err := png.Decode(f)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	sheet := image.NewRGBA(src.Bounds())
	draw.Draw(sheet, sheet.Bounds(), src, image.Point{}, draw.Src)
	withLogo, err := logo.Image(qr.WithScale(5), qr.WithLogo(image.NewUniform(color.RGBA{R: 0xc0, A: 0xff}), 0.2))
	if err != nil {
		t.Fatal(err)
	}
	draw.Draw(sheet, withLogo.Bounds().Add(image.Pt(220, 0)), withLogo, image.Point{}, draw.Src)
	if f, err = os.Create(path); err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, sheet); err != nil {
		t.Fatal(err)
	}
	f.Close()

	var out, errOut bytes.Buffer
	if err := run([]string{"decode", "-all", "-grade", "-json", path}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Symbols []decodedSymbol `json:"symbols"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("%v in %s", err, out.String())
	}
	if len(got.Symbols) != 3 {
		t.Fatalf("got %+v", got)
	}
	for i, s := range got.Symbols {
		q := s.Quality
		if q == nil || q.Error != "" || q.UnusedErrorCorrection == nil {
			t.Fatalf("symbol %d: quality %+v", i, q)
		}
		want := "A"
		if s.Text == "logo" {
			want = "C"
		}
		if q.Grade != want || q.PixelsPerModule != 5 {
			t.Errorf("%s: grade %s at %v px per module, want %s at 5", s.Text, q.Grade, q.PixelsPerModule, want)
		}
		if (s.Text == "logo") != (s.ErrorsCorrected > 0) || (s.Text == "logo") != (s.UnusedErrorCorrection < 1) {
			t.Errorf("%s: %d codewords corrected, %v unused", s.Text, s.ErrorsCorrected, s.UnusedErrorCorrection)
		}
	}

	// Without -json, a line per code follows the texts.
	out.Reset()
	if err := run([]string{"decode", "-all", "-grade", path}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 6 || lines[0] != "plain" || !strings.HasPrefix(lines[3], path+": grade A (symbol contrast A,") ||
		!strings.HasPrefix(lines[4], path+": grade C (") {
		t.Fatalf("output %q", out.String())
	}
}

func TestDecodeTimeout(t *testing.T) {
	// An image without a code is searched at every scale, which takes far
	// longer than a nanosecond.
	noise := image.NewGray(image.Rect(0, 0, 800, 600))
	rng := rand.New(rand.NewSource(1))
	for i := range noise.Pix {
		noise.Pix[i] = uint8(150 + rng.Intn(60))
	}
	path := filepath.Join(t.TempDir(), "noise.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, noise); err != nil {
		t.Fatal(err)
	}
	f.Close()
	var out, errOut bytes.Buffer
	err = run([]string{"decode", "-timeout", "1ns", path}, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "no code read within 1ns") {
		t.Errorf("error %v, want a timeout", err)
	}
	// A code is read within a generous limit.
	code, _ := qr.Encode("in time")
	path = writeSheet(t, []*qr.Code{code})
	out.Reset()
	if err := run([]string{"decode", "-timeout", "1m", path}, &out, &errOut); err != nil || out.String() != "in time\n" {
		t.Errorf("output %q, %v", out.String(), err)
	}
}
