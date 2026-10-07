package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"image/png"
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
