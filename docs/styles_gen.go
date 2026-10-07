//go:build ignore

// styles_gen renders docs/images/styles.png, the style samples shown in the
// README. Run it from the repository root: go run docs/styles_gen.go
package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"

	"github.com/piglig/go-qr/v2"
)

func main() {
	code, err := qr.Encode("https://github.com/piglig/go-qr", qr.WithECC(qr.ECCQuartile))
	if err != nil {
		log.Fatal(err)
	}
	navy := color.RGBA{R: 0x1a, G: 0x23, B: 0x7e, A: 0xff}
	teal := color.RGBA{G: 0x69, B: 0x5c, A: 0xff}
	red := color.RGBA{R: 0xc6, G: 0x28, B: 0x28, A: 0xff}
	purple := color.RGBA{R: 0x4a, G: 0x14, B: 0x8c, A: 0xff}
	styles := [][]qr.RenderOption{
		nil,
		{qr.WithModuleShape(qr.ModuleDot), qr.WithFinderShape(qr.FinderCircle)},
		{qr.WithModuleShape(qr.ModuleRounded), qr.WithFinderShape(qr.FinderRounded)},
		{qr.WithGradient(navy, teal, 45), qr.WithModuleShape(qr.ModuleRounded), qr.WithFinderShape(qr.FinderRounded)},
		{qr.WithModuleShape(qr.ModuleDot), qr.WithFinderShape(qr.FinderCircle), qr.WithFinderColor(red, color.Black)},
		{qr.WithGradient(purple, red, 90), qr.WithModuleShape(qr.ModuleDot), qr.WithFinderShape(qr.FinderRounded), qr.WithFinderColor(purple, red)},
	}
	var tiles []*image.RGBA
	for _, s := range styles {
		opts := append(s, qr.WithScale(6), qr.WithQuietZone(2))
		if err := code.Verify(opts...); err != nil {
			log.Fatal(err)
		}
		img, err := code.Image(opts...)
		if err != nil {
			log.Fatal(err)
		}
		tiles = append(tiles, img)
	}
	w := tiles[0].Bounds().Dx()
	sheet := image.NewRGBA(image.Rect(0, 0, 3*w, 2*w))
	for i, t := range tiles {
		draw.Draw(sheet, image.Rect(i%3*w, i/3*w, (i%3+1)*w, (i/3+1)*w), t, image.Point{}, draw.Src)
	}
	f, err := os.Create("docs/images/styles.png")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, sheet); err != nil {
		log.Fatal(err)
	}
}
