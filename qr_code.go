package go_qr

type Ecc int

const (
	Low      Ecc = 1
	Medium   Ecc = 0
	Quartile Ecc = 3
	High     Ecc = 2
)

type QrCode struct {
}

func (q *QrCode) EncodeText(text string, ecl Ecc) (QrCode, error) {
	return QrCode{}, nil
}

func GetBit(x, i int) bool {
	return ((x >> uint(i)) & 1) != 0
}
