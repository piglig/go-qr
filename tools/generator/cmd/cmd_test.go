package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/piglig/go-qr/tools/verify"
)

func TestRun_StdoutPNG(t *testing.T) {
	var out, errOut bytes.Buffer
	err := run([]string{"-content", "Hello, world!", "-stdout", "png"}, &out, &errOut)
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

func TestRun_Verify(t *testing.T) {
	var out, errOut bytes.Buffer
	err := run([]string{"-content", "verify-me", "-verify"}, &out, &errOut)
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

func TestRun_PNGFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.png")
	var out, errOut bytes.Buffer
	err := run([]string{"-content", "file-test", "-png", path}, &out, &errOut)
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

func TestRun_NoContent(t *testing.T) {
	var out, errOut bytes.Buffer
	err := run([]string{}, &out, &errOut)
	if err == nil {
		t.Fatal("expected error when -content missing")
	}
}

func TestRun_UnknownECC(t *testing.T) {
	var out, errOut bytes.Buffer
	err := run([]string{"-content", "x", "-ecc", "ultra"}, &out, &errOut)
	if err == nil {
		t.Fatal("expected error for invalid ecc")
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
