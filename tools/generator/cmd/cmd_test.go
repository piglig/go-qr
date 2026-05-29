package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/piglig/go-qr/tools/verify"
)

func TestRun_EncodeStdoutPNG(t *testing.T) {
	var out, errOut bytes.Buffer
	err := run([]string{"encode", "-content", "Hello, world!", "-stdout", "png"}, &out, &errOut)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	sig := out.Bytes()
	if len(sig) < 8 || !bytes.Equal(sig[:8], []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}) {
		t.Fatalf("stdout is not a PNG (first bytes: % x)", sig[:min(8, len(sig))])
	}
	got, err := verify.DecodePNG(sig)
	if err != nil {
		t.Fatalf("decode piped PNG: %v", err)
	}
	if got != "Hello, world!" {
		t.Fatalf("want %q, got %q", "Hello, world!", got)
	}
}

func TestRun_EncodePositionalContent(t *testing.T) {
	var out, errOut bytes.Buffer
	// content as a positional argument (after flags) instead of -content.
	err := run([]string{"encode", "-stdout", "png", "positional-content"}, &out, &errOut)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	got, err := verify.DecodePNG(out.Bytes())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got != "positional-content" {
		t.Fatalf("want %q, got %q", "positional-content", got)
	}
}

func TestRun_Decode(t *testing.T) {
	dir := t.TempDir()
	pngPath := filepath.Join(dir, "qr.png")

	var out, errOut bytes.Buffer
	if err := run([]string{"encode", "-content", "decode round trip 7", "-png", pngPath, "-quiet"}, &out, &errOut); err != nil {
		t.Fatalf("encode run: %v", err)
	}

	out.Reset()
	if err := run([]string{"decode", pngPath}, &out, &errOut); err != nil {
		t.Fatalf("decode run: %v", err)
	}
	if got := strings.TrimRight(out.String(), "\n"); got != "decode round trip 7" {
		t.Fatalf("want %q, got %q", "decode round trip 7", got)
	}
}

func TestRun_Verify(t *testing.T) {
	var out, errOut bytes.Buffer
	err := run([]string{"encode", "-content", "verify-me", "-verify"}, &out, &errOut)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(errOut.String(), "verify: ok") {
		t.Fatalf("expected verify: ok in stderr, got %q", errOut.String())
	}
}

func TestRun_PayloadWiFi(t *testing.T) {
	var out, errOut bytes.Buffer
	err := run([]string{
		"encode",
		"-payload", "wifi",
		"-content", "SSID=home,Password=s3cret,Auth=WPA",
		"-stdout", "png",
	}, &out, &errOut)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	decoded, err := verify.DecodePNG(out.Bytes())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.HasPrefix(decoded, "WIFI:") || !strings.Contains(decoded, "S:home") {
		t.Fatalf("unexpected WIFI payload %q", decoded)
	}
}

func TestRun_EncodePNGFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.png")
	var out, errOut bytes.Buffer
	err := run([]string{"encode", "-content", "file-test", "-png", path}, &out, &errOut)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	decoded, err := verify.DecodePNG(b)
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if decoded != "file-test" {
		t.Fatalf("want %q, got %q", "file-test", decoded)
	}
}

func TestRun_NoArgsShowsTopUsage(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{}, &out, &errOut); err == nil {
		t.Fatal("expected error when no command given")
	}
	if !strings.Contains(errOut.String(), "Commands:") {
		t.Fatalf("expected command list on stderr, got %q", errOut.String())
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{"frobnicate"}, &out, &errOut); err == nil {
		t.Fatal("expected error for unknown command")
	}
	if !strings.Contains(errOut.String(), "unknown command") {
		t.Fatalf("expected 'unknown command' on stderr, got %q", errOut.String())
	}
}

func TestRun_EncodeUnknownECC(t *testing.T) {
	var out, errOut bytes.Buffer
	err := run([]string{"encode", "-content", "x", "-ecc", "ultra"}, &out, &errOut)
	if err == nil {
		t.Fatal("expected error for invalid ecc")
	}
}

func TestRun_HelpIsClean(t *testing.T) {
	var out, errOut bytes.Buffer
	// help must exit cleanly (run returns nil) and print the command list to stdout.
	if err := run([]string{"help"}, &out, &errOut); err != nil {
		t.Fatalf("help should return nil, got %v", err)
	}
	if !strings.Contains(out.String(), "Commands:") {
		t.Fatalf("expected command list on stdout, got %q", out.String())
	}
}

func TestRun_EncodeHelpIsClean(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{"encode", "-h"}, &out, &errOut); err != nil {
		t.Fatalf("encode -h should return nil, got %v", err)
	}
	if !strings.Contains(errOut.String(), "Examples:") {
		t.Fatalf("expected examples on stderr, got %q", errOut.String())
	}
}

func TestRun_Version(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{"version"}, &out, &errOut); err != nil {
		t.Fatalf("run: %v", err)
	}
	if strings.TrimSpace(out.String()) == "" {
		t.Fatal("expected a version string on stdout")
	}
}

func TestRun_EncodeNoContentShowsUsage(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{"encode"}, &out, &errOut); err == nil {
		t.Fatal("expected error when encode has no content")
	}
	if !strings.Contains(errOut.String(), "Usage:") {
		t.Fatalf("expected usage on stderr, got %q", errOut.String())
	}
}

func TestRun_DecodeRequiresExactlyOnePath(t *testing.T) {
	var out, errOut bytes.Buffer
	if err := run([]string{"decode"}, &out, &errOut); err == nil {
		t.Fatal("expected error when decode has no path")
	}
}

func TestRun_StdoutConflictsWithFileOutput(t *testing.T) {
	var out, errOut bytes.Buffer
	err := run([]string{"encode", "-content", "x", "-stdout", "png", "-png", "y.png"}, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "-stdout cannot be combined") {
		t.Fatalf("expected stdout/file conflict error, got %v", err)
	}
}

func TestRun_GeoInvalidCoordinate(t *testing.T) {
	var out, errOut bytes.Buffer
	err := run([]string{"encode", "-payload", "geo", "-content", "lat=notanumber,lon=2", "-stdout", "png"}, &out, &errOut)
	if err == nil || !strings.Contains(err.Error(), "geo lat") {
		t.Fatalf("expected geo lat parse error, got %v", err)
	}
}

func TestParseKV_EscapesCommasAndEquals(t *testing.T) {
	kv, err := parseKV(`a=1\,2,b=x\=y`)
	if err != nil {
		t.Fatal(err)
	}
	if kv["a"] != "1,2" || kv["b"] != "x=y" {
		t.Fatalf("unexpected parse: %v", kv)
	}
}
