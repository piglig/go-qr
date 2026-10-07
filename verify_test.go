package qr

import (
	"errors"
	"image/color"
	"testing"
)

func TestVerify(t *testing.T) {
	code := mustEncode(t, "https://example.com/verify", WithECC(ECCHigh))
	logo := makeTestLogo(16, 16, red)
	navy := color.RGBA{R: 0x10, G: 0x20, B: 0x50, A: 0xff}
	beige := color.RGBA{R: 0xf5, G: 0xf0, B: 0xe1, A: 0xff}

	tests := []struct {
		name string
		opts []RenderOption
		want error
	}{
		{"defaults", nil, nil},
		{"small scale, no quiet zone", []RenderOption{WithScale(1), WithQuietZone(0)}, nil},
		{"brand colors", []RenderOption{WithForeground(navy), WithBackground(beige)}, nil},
		{"transparent background", []RenderOption{WithBackground(color.Transparent)}, nil},
		{"logo", []RenderOption{WithLogo(logo, 0.2)}, nil},
		{"low contrast", []RenderOption{WithForeground(color.Gray{Y: 150}), WithBackground(color.Gray{Y: 200})}, ErrUnreadable},
		{"inverted", []RenderOption{WithForeground(color.White), WithBackground(color.Black)}, ErrUnreadable},
		{"translucent foreground", []RenderOption{WithForeground(color.NRGBA{A: 60})}, ErrUnreadable},
		{"logo too large", []RenderOption{WithLogo(logo, 0.6)}, ErrLogoTooLarge},
		{"invalid option", []RenderOption{WithScale(0)}, ErrInvalidArgument},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := code.Verify(tt.opts...)
			if tt.want == nil && err != nil {
				t.Fatalf("Verify: %v", err)
			}
			if tt.want != nil && !errors.Is(err, tt.want) {
				t.Fatalf("Verify error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestVerifyDetectsDamage(t *testing.T) {
	code := mustEncode(t, "damaged beyond repair", WithECC(ECCLow), WithoutECCBoost())
	// Flip a band of data modules, far more than ECC L can correct.
	damaged := *code
	damaged.modules = make([][]bool, code.size)
	for y := range damaged.modules {
		damaged.modules[y] = append([]bool(nil), code.modules[y]...)
	}
	for y := 9; y < code.size; y++ {
		for x := 9; x < code.size; x += 2 {
			damaged.modules[y][x] = !damaged.modules[y][x]
		}
	}
	if err := damaged.Verify(); !errors.Is(err, ErrUnreadable) {
		t.Fatalf("Verify error = %v, want ErrUnreadable", err)
	}
	if err := code.Verify(); err != nil {
		t.Fatalf("undamaged code: %v", err)
	}
}

func TestLuminanceOverWhite(t *testing.T) {
	assertInDelta(t, 0, luminanceOverWhite(color.Black), 1e-9)
	assertInDelta(t, 1, luminanceOverWhite(color.White), 1e-9)
	assertInDelta(t, 1, luminanceOverWhite(color.Transparent), 1e-9)
}
