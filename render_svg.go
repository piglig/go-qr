package go_qr

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// SVG renders the QR code to the given file path. Colors are taken from the
// config (WithLight / WithDark). Default colors are white/black.
func (q *QrCode) SVG(config *QrCodeImgConfig, filePath string) error {
	if err := config.Valid(); err != nil {
		return err
	}

	if ext := filepath.Ext(filePath); ext != ".svg" {
		return fmt.Errorf("%w: expected .svg extension, got %q", ErrInvalidImageOutput, ext)
	}

	svgFile, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("error creating SVG file: %w", err)
	}
	defer svgFile.Close()

	return q.doWriteAsSVG(config, svgFile)
}

// WriteAsSVG renders the QR code as SVG to the provided io.Writer.
// Colors are taken from the config.
func (q *QrCode) WriteAsSVG(config *QrCodeImgConfig, writer io.Writer) error {
	if err := config.Valid(); err != nil {
		return err
	}
	return q.doWriteAsSVG(config, writer)
}

// ToSVGBytes renders the QR code as SVG and returns the bytes in memory.
// Colors are taken from the config.
func (q *QrCode) ToSVGBytes(config *QrCodeImgConfig) ([]byte, error) {
	if err := config.Valid(); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := q.doWriteAsSVG(config, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// doWriteAsSVG writes the QR code as SVG, reading colors from the config.
// A transparent light color omits the background rectangle.
func (q *QrCode) doWriteAsSVG(config *QrCodeImgConfig, writer io.Writer) error {
	var light string
	if !colorIsTransparent(config.Light()) {
		light = colorToSVGHex(config.Light())
	}
	dark := colorToSVGHex(config.Dark())

	svg := ""
	if config.options.optimalSVG {
		svg = q.toSvgOptimizedString(config, light, dark)
	} else {
		svg = q.toSVGString(config, light, dark)
	}

	if logo := config.options.logo; logo != nil {
		if err := logo.validate(q, config.scale, config.border); err != nil {
			return err
		}
		fragment, err := logo.svgEmbed(q.GetSize(), config.scale, config.border)
		if err != nil {
			return err
		}
		svg = injectSVGFragment(svg, fragment)
	}

	if _, err := writer.Write([]byte(svg)); err != nil {
		return fmt.Errorf("error writing SVG: %w", err)
	}
	return nil
}

// injectSVGFragment inserts the given fragment just before the closing </svg> tag.
func injectSVGFragment(svg, fragment string) string {
	const closing = "</svg>"
	idx := strings.LastIndex(svg, closing)
	if idx < 0 {
		return svg + fragment
	}
	return svg[:idx] + fragment + svg[idx:]
}

// toSVGString generates an SVG string using the given config and pre-formatted
// light/dark color strings. One <rect> background, one <path> of per-module
// subpaths. Written to avoid fmt.Sprintf and intermediate copies; the builder
// is pre-sized to the worst-case module count.
func (q *QrCode) toSVGString(config *QrCodeImgConfig, lightColor, darkColor string) string {
	brd := config.border
	scl := config.scale
	size := q.GetSize()
	dim := size*scl + brd*2
	dimStr := strconv.Itoa(dim)
	sclStr := strconv.Itoa(scl)

	sb := strings.Builder{}
	// Header + rect + path wrapper ≈ 200 bytes; each dark module emits
	// roughly 20 bytes of path data. size*size is the upper bound on
	// dark modules, so this over-allocates but avoids repeated regrowth.
	sb.Grow(256 + size*size*20)

	if config.options.svgXMLHeader {
		sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
		sb.WriteString("<!DOCTYPE svg PUBLIC \"-//W3C//DTD SVG 1.1//EN\" \"http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd\">\n")
	}
	sb.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" version="1.1" viewBox="0 0 `)
	sb.WriteString(dimStr)
	sb.WriteByte(' ')
	sb.WriteString(dimStr)
	sb.WriteString("\" stroke=\"none\">\n")

	sb.WriteString("\t<rect width=\"")
	sb.WriteString(dimStr)
	sb.WriteString("\" height=\"")
	sb.WriteString(dimStr)
	sb.WriteString("\" fill=\"")
	sb.WriteString(lightColor)
	sb.WriteString("\"/>\n")

	sb.WriteString("\t<path d=\"")
	// Scratch buffer for strconv.AppendInt — avoids the per-call allocation
	// that strconv.Itoa makes for each coordinate.
	var scratch [20]byte
	first := true
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if !q.GetModule(x, y) {
				continue
			}
			if !first {
				sb.WriteByte(' ')
			}
			first = false
			sb.WriteByte('M')
			sb.Write(strconv.AppendInt(scratch[:0], int64(x*scl+brd), 10))
			sb.WriteByte(',')
			sb.Write(strconv.AppendInt(scratch[:0], int64(y*scl+brd), 10))
			sb.WriteByte('h')
			sb.WriteString(sclStr)
			sb.WriteByte('v')
			sb.WriteString(sclStr)
			sb.WriteString("h-")
			sb.WriteString(sclStr)
			sb.WriteByte('z')
		}
	}
	sb.WriteString("\" fill=\"")
	sb.WriteString(darkColor)
	sb.WriteString("\"/>\n</svg>\n")

	return sb.String()
}
