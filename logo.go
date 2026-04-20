package go_qr

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
)

// logoConfig holds the configuration for embedding a logo in the center of a QR code.
type logoConfig struct {
	img       image.Image
	sizeRatio float64
}

// WithLogo embeds the given image in the center of the QR code.
//
// sizeRatio is the logo side length as a fraction of the QR code's module-area
// side length (excluding the quiet-zone border). Typical values are 0.15–0.22.
// A 1-module-wide white padding is drawn between the logo and the surrounding
// QR modules to keep finder patterns readable.
//
// The logo occludes a portion of the QR modules and relies on error correction
// to remain scannable. Higher error correction levels tolerate larger logos;
// rendering will fail if the occluded area exceeds what the chosen ECC can
// realistically recover.
func WithLogo(img image.Image, sizeRatio float64) func(*QrCodeImgConfig) {
	return func(q *QrCodeImgConfig) {
		q.options.logo = &logoConfig{img: img, sizeRatio: sizeRatio}
	}
}

// eccRecoveryBudget returns the fraction of modules that can safely be occluded
// for the given ECC level. Values are conservative: the spec defines recovery
// capacity per codeword, but in practice finder-pattern position and masking
// make the usable budget smaller.
func eccRecoveryBudget(ecl Ecc) float64 {
	switch ecl {
	case Low:
		return 0.05
	case Medium:
		return 0.12
	case Quartile:
		return 0.20
	case High:
		return 0.25
	default:
		return 0.05
	}
}

// logoRect computes the logo's occluded rectangle in image-pixel coordinates,
// including the 1-module white padding. It also returns the occluded area as
// a fraction of the QR module area (excluding the border).
func (l *logoConfig) logoRect(qrSize, scale, border int) (image.Rectangle, float64, error) {
	if l.sizeRatio <= 0 || l.sizeRatio >= 1 {
		return image.Rectangle{}, 0, fmt.Errorf("logo sizeRatio must be in (0, 1), got %v", l.sizeRatio)
	}
	if l.img == nil {
		return image.Rectangle{}, 0, errors.New("logo image is nil")
	}

	// Logo side in modules, rounded down to an even integer so it centers cleanly.
	logoModules := int(float64(qrSize) * l.sizeRatio)
	if logoModules < 1 {
		logoModules = 1
	}
	if logoModules%2 != qrSize%2 {
		// Match parity with qrSize so the logo can be pixel-centered.
		logoModules--
		if logoModules < 1 {
			logoModules = 1
		}
	}

	const paddingModules = 1
	boxModules := logoModules + 2*paddingModules
	if boxModules >= qrSize {
		return image.Rectangle{}, 0, fmt.Errorf("logo too large: covers %d of %d modules", boxModules, qrSize)
	}

	// Center of the module area in image pixels. border is measured in modules.
	centerPx := border*scale + (qrSize*scale)/2
	halfPx := (boxModules * scale) / 2
	rect := image.Rect(centerPx-halfPx, centerPx-halfPx, centerPx+halfPx, centerPx+halfPx)

	occludedRatio := float64(boxModules*boxModules) / float64(qrSize*qrSize)
	return rect, occludedRatio, nil
}

// validate checks that the logo configuration is compatible with the QR code's
// error correction level.
func (l *logoConfig) validate(q *QrCode, scale, border int) error {
	_, ratio, err := l.logoRect(q.GetSize(), scale, border)
	if err != nil {
		return err
	}
	budget := eccRecoveryBudget(q.errorCorrectionLevel)
	if ratio > budget {
		return fmt.Errorf("logo occludes %.1f%% of QR modules, exceeds ECC %v budget of %.1f%% (use a smaller sizeRatio or a higher ECC)",
			ratio*100, q.errorCorrectionLevel, budget*100)
	}
	return nil
}

// overlayOnImage composites the logo (with white padding) onto the given RGBA image.
func (l *logoConfig) overlayOnImage(dst *image.RGBA, qrSize, scale, border int) error {
	rect, _, err := l.logoRect(qrSize, scale, border)
	if err != nil {
		return err
	}

	// White padding box.
	draw.Draw(dst, rect, &image.Uniform{C: image.White}, image.Point{}, draw.Src)

	// Inset for the actual logo (strip 1-module padding on each side).
	inset := scale
	logoRect := image.Rect(rect.Min.X+inset, rect.Min.Y+inset, rect.Max.X-inset, rect.Max.Y-inset)

	// Scale the source image into logoRect using nearest-neighbor. A high-quality
	// scaler would pull in golang.org/x/image; nearest is sufficient since logos
	// are typically pre-sized by the caller.
	drawScaled(dst, logoRect, l.img)
	return nil
}

// drawScaled performs nearest-neighbor scaling of src into dst's dstRect.
func drawScaled(dst *image.RGBA, dstRect image.Rectangle, src image.Image) {
	sb := src.Bounds()
	dw := dstRect.Dx()
	dh := dstRect.Dy()
	if dw <= 0 || dh <= 0 || sb.Dx() <= 0 || sb.Dy() <= 0 {
		return
	}
	for y := 0; y < dh; y++ {
		sy := sb.Min.Y + y*sb.Dy()/dh
		for x := 0; x < dw; x++ {
			sx := sb.Min.X + x*sb.Dx()/dw
			dst.Set(dstRect.Min.X+x, dstRect.Min.Y+y, src.At(sx, sy))
		}
	}
}

// svgEmbed returns the SVG fragment rendering the logo: a white background
// rect plus a base64-embedded <image>.
func (l *logoConfig) svgEmbed(qrSize, scale, border int) (string, error) {
	rect, _, err := l.logoRect(qrSize, scale, border)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, l.img); err != nil {
		return "", fmt.Errorf("failed to encode logo as PNG for SVG embedding: %w", err)
	}
	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())

	inset := scale
	logoX := rect.Min.X + inset
	logoY := rect.Min.Y + inset
	logoW := rect.Dx() - 2*inset
	logoH := rect.Dy() - 2*inset

	return fmt.Sprintf(
		"\t<rect x=\"%d\" y=\"%d\" width=\"%d\" height=\"%d\" fill=\"#FFFFFF\"/>\n"+
			"\t<image x=\"%d\" y=\"%d\" width=\"%d\" height=\"%d\" href=\"data:image/png;base64,%s\"/>\n",
		rect.Min.X, rect.Min.Y, rect.Dx(), rect.Dy(),
		logoX, logoY, logoW, logoH, encoded,
	), nil
}
