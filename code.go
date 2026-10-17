package qr

import (
	"fmt"

	"github.com/piglig/go-qr/v2/internal/symbol"
)

// ECC is an error correction level. Higher levels recover from more damage at
// the cost of capacity.
type ECC int8

const (
	ECCLow      ECC = iota // recovers about 7% of codewords
	ECCMedium              // recovers about 15% of codewords
	ECCQuartile            // recovers about 25% of codewords
	ECCHigh                // recovers about 30% of codewords
)

// String returns "L", "M", "Q" or "H".
func (e ECC) String() string {
	switch e {
	case ECCLow:
		return "L"
	case ECCMedium:
		return "M"
	case ECCQuartile:
		return "Q"
	case ECCHigh:
		return "H"
	}
	return fmt.Sprintf("ECC(%d)", int8(e))
}

func (e ECC) valid() bool { return ECCLow <= e && e <= ECCHigh }

// MinVersion and MaxVersion bound the QR Code Model 2 versions (symbol sizes
// 21×21 to 177×177 modules).
const (
	MinVersion = symbol.MinVersion
	MaxVersion = symbol.MaxVersion
)

// Code is an immutable QR Code symbol. Create one with Encode, EncodeBytes or
// EncodeSegments; query it with Size and Module; render it with the Write*
// methods.
type Code struct {
	version int
	size    int
	ecc     ECC
	mask    int
	modules [][]bool // modules[y][x] reports whether the module is dark
}

// newCode encodes the data codewords into a finished Code at the given version
// and ECC level. mask selects the mask pattern; -1 chooses the lowest-penalty
// mask.
func newCode(ver int, ecc ECC, dataCodewords []byte, mask int) (*Code, error) {
	b := symbol.NewBuilder(ver, symbol.Level(ecc))
	b.DrawFunctionPatterns()

	allCodewords, err := b.AddECCAndInterleave(dataCodewords)
	if err != nil {
		return nil, err
	}
	if err := b.DrawCodewords(allCodewords); err != nil {
		return nil, err
	}

	if mask == -1 {
		mask = b.ChooseBestMask()
	}
	if err := b.ApplyMask(mask); err != nil {
		return nil, err
	}
	b.DrawFormatBits(mask)

	// The module grid is handed over; the builder is not used after.
	return &Code{version: ver, size: b.Size, ecc: ecc, mask: mask, modules: b.Modules}, nil
}

// Size returns the side length of the symbol in modules (4*Version()+17). It
// does not include the quiet zone.
func (c *Code) Size() int { return c.size }

// Version returns the symbol version, from MinVersion to MaxVersion.
func (c *Code) Version() int { return c.version }

// ECC returns the error correction level of the symbol. It can be higher than
// the requested level, see WithoutECCBoost.
func (c *Code) ECC() ECC { return c.ecc }

// Mask returns the mask pattern applied to the symbol, from 0 to 7.
func (c *Code) Mask() int { return c.mask }

// Module reports whether the module at (x, y) is dark. Coordinates outside the
// symbol report false (light), which matches the quiet zone.
func (c *Code) Module(x, y int) bool {
	return 0 <= x && x < c.size && 0 <= y && y < c.size && c.modules[y][x]
}
