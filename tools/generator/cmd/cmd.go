package cmd

import (
	"errors"
	"flag"
	"fmt"
	go_qr "github.com/piglig/go-qr"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"strings"
)

const (
	pngType = "png"
	svgType = "svg"
)

type Command struct {
	Content   string
	PngOutput string
	SvgOutput string
}

func newCommand() *Command {
	cmd := &Command{}
	flag.StringVar(&cmd.Content, "content", "", "Content to encode in the QR code")
	flag.StringVar(&cmd.PngOutput, "png", "", "Output PNG file name")
	flag.StringVar(&cmd.SvgOutput, "svg", "", "Output SVG file name")

	flag.Parse()
	return cmd
}

func Exec() {
	cmd := newCommand()

	if cmd.Content == "" {
		fmt.Println("Error: Please provide content to encode using the -content flag.")
		return
	}

	if cmd.PngOutput != "" {
		err := generateQrCode(cmd.Content, pngType, cmd.PngOutput)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			return
		}
	}

	if cmd.SvgOutput != "" {
		err := generateQrCode(cmd.Content, svgType, cmd.SvgOutput)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			return
		}
	}
}

func generateQrCode(content, outputType, outputFile string) error {
	qr, err := go_qr.EncodeText(content, go_qr.High)
	if err != nil {
		return err
	}

	switch outputType {
	case pngType:
		img := toImageStandard(qr, 10, 4)
		err = writePng(img, outputFile)
		if err != nil {
			return err
		}
	case svgType:
		svg, err := toSvgString(qr, 4, "#FFFFFF", "#000000")
		if err != nil {
			return err
		}

		svgFile, err := os.Create(outputFile)
		if err != nil {
			return err
		}
		defer svgFile.Close()
		_, err = svgFile.WriteString(svg)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported file type: %s", outputType)
	}
	return nil
}

func toImageStandard(qr *go_qr.QrCode, scale, border int) *image.RGBA {
	return toImage(qr, scale, border, color.White, color.Black)
}

func toImage(qr *go_qr.QrCode, scale, border int, lightColor, darkColor color.Color) *image.RGBA {
	if scale <= 0 || border < 0 || qr == nil {
		panic("Invalid input")
	}

	if border > (math.MaxInt32/2) || int64(qr.GetSize())+int64(border)*2 > math.MaxInt32/int64(scale) {
		panic("Scale or border too large")
	}

	size := qr.GetSize() + border*2
	imageWidth := size * scale
	imageHeight := size * scale
	result := image.NewRGBA(image.Rect(0, 0, imageWidth, imageHeight))
	for y := 0; y < imageHeight; y++ {
		for x := 0; x < imageWidth; x++ {
			moduleX := x/scale - border
			moduleY := y/scale - border
			isDark := qr.GetModule(moduleX, moduleY)
			if isDark {
				result.Set(x, y, darkColor)
			} else {
				result.Set(x, y, lightColor)
			}
		}
	}
	return result
}

func writePng(img *image.RGBA, filepath string) error {
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	err = png.Encode(file, img)
	if err != nil {
		return err
	}

	return nil
}

func toSvgString(qr *go_qr.QrCode, border int, lightColor, darkColor string) (string, error) {
	if border < 0 {
		return "", errors.New("border must be non-negative")
	}

	if qr == nil {
		return "", errors.New("qr is nil")
	}

	var brd = int64(border)
	sb := strings.Builder{}
	sb.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	sb.WriteString("<!DOCTYPE svg PUBLIC \"-//W3C//DTD SVG 1.1//EN\" \"http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd\">\n")
	sb.WriteString(fmt.Sprintf("<svg xmlns=\"http://www.w3.org/2000/svg\" version=\"1.1\" viewBox=\"0 0 %d %d\" stroke=\"none\">\n",
		int64(qr.GetSize())+brd*2, int64(qr.GetSize())+brd*2))
	sb.WriteString("\t<rect width=\"100%\" height=\"100%\" fill=\"" + lightColor + "\"/>\n")
	sb.WriteString("\t<path d=\"")

	for y := 0; y < qr.GetSize(); y++ {
		for x := 0; x < qr.GetSize(); x++ {
			if qr.GetModule(x, y) {
				if x != 0 || y != 0 {
					sb.WriteString(" ")
				}
				sb.WriteString(fmt.Sprintf("M%d,%dh1v1h-1z", int64(x)+brd, int64(y)+brd))
			}
		}
	}
	sb.WriteString("\" fill=\"" + darkColor + "\"/>\n")
	sb.WriteString("</svg>\n")

	return sb.String(), nil
}
