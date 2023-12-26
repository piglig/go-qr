package cmd

import (
	"bytes"
	"flag"
	"fmt"
	go_qr "github.com/piglig/go-qr"
	"os"
)

const (
	pngType          = "png"
	svgType          = "svg"
	svgOptimizedType = "svg-optimized"
	textArtType      = "textArt"
)

const (
	blackBlock = "\033[40m  \033[0m"
	whiteBlock = "\033[47m  \033[0m"
)

type Command struct {
	Content            string
	PngOutput          string
	SvgOutput          string
	SvgOptimizedOutput string
}

func newCommand() *Command {
	cmd := &Command{}
	flag.StringVar(&cmd.Content, "content", "", "Content to encode in the QR code")
	flag.StringVar(&cmd.PngOutput, "png", "", "Output PNG file name")
	flag.StringVar(&cmd.SvgOutput, "svg", "", "Output SVG file name")
	flag.StringVar(&cmd.SvgOptimizedOutput, "svg-optimized", "", "Output SVG (optimized) file name - regions with connected black pixels are merged into a single path")

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

	if cmd.SvgOptimizedOutput != "" {
		err := generateQrCode(cmd.Content, svgOptimizedType, cmd.SvgOptimizedOutput)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
			return
		}
	}

	err := generateQrCode(cmd.Content, textArtType, "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}
}

func generateQrCode(content, outputType, outputFile string) error {
	qr, err := go_qr.EncodeText(content, go_qr.High)
	if err != nil {
		return err
	}

	switch outputType {
	case pngType:
		config := go_qr.NewQrCodeImgConfig(10, 4)
		err = qr.PNG(config, outputFile)
		if err != nil {
			return err
		}
	case svgType:
		config := go_qr.NewQrCodeImgConfig(10, 4)
		err = qr.SVG(config, outputFile, "#FFFFFF", "#000000")
		if err != nil {
			return err
		}
	case svgOptimizedType:
		svg := toSvgOptimizedString(qr, 4, 1, "#FFFFFF", "#000000")
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
	case textArtType:
		art := toString(qr)
		fmt.Println(art)
	default:
		return fmt.Errorf("unsupported file type: %s", outputType)
	}
	return nil
}

func toString(qr *go_qr.QrCode) string {
	buf := bytes.Buffer{}
	border := 4
	for y := -border; y < qr.GetSize()+border; y++ {
		for x := -border; x < qr.GetSize()+border; x++ {
			if !qr.GetModule(x, y) {
				buf.WriteString(blackBlock)
			} else {
				buf.WriteString(whiteBlock)
			}
		}
		buf.WriteString("\n")
	}
	return buf.String()
}
