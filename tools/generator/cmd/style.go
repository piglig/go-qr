package cmd

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"github.com/piglig/go-qr/v2"
)

// styleOpts holds the rendering flags.
type styleOpts struct {
	Scale, QuietZone int
	Module, Finder   string
	FG, BG           string
	Gradient         string // from,to[,angle]
	FinderColor      string // ring[,center]
	Logo             string
	LogoRatio        float64
}

var (
	moduleShapes = map[string]qr.ModuleShape{"square": qr.ModuleSquare, "dot": qr.ModuleDot, "rounded": qr.ModuleRounded}
	finderShapes = map[string]qr.FinderShape{"square": qr.FinderSquare, "rounded": qr.FinderRounded, "circle": qr.FinderCircle}
)

// renderOptions turns the style flags into render options.
func (s styleOpts) renderOptions() ([]qr.RenderOption, error) {
	opts := []qr.RenderOption{qr.WithScale(s.Scale), qr.WithQuietZone(s.QuietZone)}

	shape, ok := moduleShapes[strings.ToLower(s.Module)]
	if !ok {
		return nil, fmt.Errorf("unknown module shape %q (expected square, dot or rounded)", s.Module)
	}
	finder, ok := finderShapes[strings.ToLower(s.Finder)]
	if !ok {
		return nil, fmt.Errorf("unknown finder shape %q (expected square, rounded or circle)", s.Finder)
	}
	opts = append(opts, qr.WithModuleShape(shape), qr.WithFinderShape(finder))

	if s.FG != "" {
		c, err := parseColor(s.FG)
		if err != nil {
			return nil, fmt.Errorf("-fg: %w", err)
		}
		opts = append(opts, qr.WithForeground(c))
	}
	if s.BG != "" {
		c, err := parseColor(s.BG)
		if err != nil {
			return nil, fmt.Errorf("-bg: %w", err)
		}
		opts = append(opts, qr.WithBackground(c))
	}
	if s.Gradient != "" {
		parts := strings.Split(s.Gradient, ",")
		if len(parts) < 2 || len(parts) > 3 {
			return nil, fmt.Errorf("-gradient: want from,to[,angle], got %q", s.Gradient)
		}
		from, err1 := parseColor(parts[0])
		to, err2 := parseColor(parts[1])
		angle := 0.0
		var err3 error
		if len(parts) == 3 {
			angle, err3 = strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
		}
		for _, err := range []error{err1, err2, err3} {
			if err != nil {
				return nil, fmt.Errorf("-gradient: %w", err)
			}
		}
		opts = append(opts, qr.WithGradient(from, to, angle))
	}
	if s.FinderColor != "" {
		parts := strings.Split(s.FinderColor, ",")
		if len(parts) > 2 {
			return nil, fmt.Errorf("-finder-color: want ring[,center], got %q", s.FinderColor)
		}
		ring, err := parseColor(parts[0])
		if err != nil {
			return nil, fmt.Errorf("-finder-color: %w", err)
		}
		center := ring
		if len(parts) == 2 {
			if center, err = parseColor(parts[1]); err != nil {
				return nil, fmt.Errorf("-finder-color: %w", err)
			}
		}
		opts = append(opts, qr.WithFinderColor(ring, center))
	}
	if s.Logo != "" {
		img, err := loadImage(s.Logo)
		if err != nil {
			return nil, fmt.Errorf("load logo: %w", err)
		}
		opts = append(opts, qr.WithLogo(img, s.LogoRatio))
	}
	return opts, nil
}

// parseColor parses #rgb, #rrggbb, #rrggbbaa (the # is optional) or
// "transparent".
func parseColor(s string) (color.Color, error) {
	s = strings.TrimSpace(s)
	if strings.EqualFold(s, "transparent") {
		return color.Transparent, nil
	}
	h := strings.TrimPrefix(s, "#")
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	if len(h) == 6 {
		h += "ff"
	}
	v, err := strconv.ParseUint(h, 16, 32)
	if len(h) != 8 || err != nil {
		return nil, fmt.Errorf("invalid color %q (want #rrggbb, #rrggbbaa or transparent)", s)
	}
	return color.NRGBA{R: uint8(v >> 24), G: uint8(v >> 16), B: uint8(v >> 8), A: uint8(v)}, nil
}
