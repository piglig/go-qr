package qr

import (
	"fmt"
	"image"

	"github.com/piglig/go-qr/v2/internal/render"
	"github.com/piglig/go-qr/v2/internal/symbol"
)

// WithLogo draws img over the center of the symbol, on a background-colored
// pad one module wide.
//
// ratio is the logo side as a fraction of the symbol side (quiet zone
// excluded); 0.15-0.22 is typical. The covered modules are lost, so the
// symbol depends on error correction to stay readable: rendering fails with
// ErrLogoTooLarge when the codewords under the logo would use more than 75%
// of the correction capacity of any error correction block, leaving the rest
// for real-world damage. Encode with WithECC(ECCHigh) to allow the largest
// logos.
func WithLogo(img image.Image, ratio float64) RenderOption {
	return func(c *renderConfig) { c.Logo = &render.Logo{Img: img, Ratio: ratio} }
}

// validateLogoOptions checks the logo options.
func validateLogoOptions(l *render.Logo) error {
	switch {
	case l.Img == nil:
		return fmt.Errorf("%w: nil logo image", ErrInvalidArgument)
	case l.Img.Bounds().Empty():
		return fmt.Errorf("%w: empty logo image", ErrInvalidArgument)
	case !(l.Ratio > 0 && l.Ratio < 1):
		return fmt.Errorf("%w: logo ratio %v must be in (0, 1)", ErrInvalidArgument, l.Ratio)
	}
	return nil
}

// maxLogoDamage is the share of each error correction block's capacity that
// a logo may consume. The rest is left for print defects, glare and blur.
const maxLogoDamage = 0.75

// validateLogo checks that the logo leaves enough error correction capacity.
func validateLogo(l *render.Logo, code *Code) error {
	box := l.BoxModules(code.Size())
	if box >= code.Size() {
		return fmt.Errorf("%w: logo box of %d modules covers the whole %d-module symbol", ErrLogoTooLarge, box, code.Size())
	}
	if damage := logoDamage(code, box); damage > maxLogoDamage {
		return fmt.Errorf("%w: logo uses %.0f%% of the error correction capacity of ECC %v, over the %.0f%% limit (use a smaller ratio or a higher ECC level)",
			ErrLogoTooLarge, damage*100, code.ECC(), maxLogoDamage*100)
	}
	return nil
}

// logoDamage returns the largest fraction of any error correction block's
// capacity used up by a centered logo box of the given side. Every codeword
// with a module under the box counts as lost, whatever the logo looks like.
func logoDamage(code *Code, box int) float64 {
	ver, ecc := code.Version(), code.ECC()
	l := symbol.Level(ecc)
	blockOf := symbol.CodewordBlocks(ver, l)
	bitAt := symbol.BitAt(ver)
	lost := make([]bool, len(blockOf))
	perBlock := make([]int, symbol.NumErrorCorrectionBlocks[l][ver])
	start := (code.Size() - box) / 2
	for y := start; y < start+box; y++ {
		for x := start; x < start+box; x++ {
			if bit := bitAt[y][x]; bit >= 0 && !lost[bit/8] {
				cw := bit / 8
				lost[cw] = true
				perBlock[blockOf[cw]]++
			}
		}
	}
	capacity := float64(symbol.CorrectableErrors(ver, l))
	worst := 0.0
	for _, n := range perBlock {
		worst = max(worst, float64(n)/capacity)
	}
	return worst
}
