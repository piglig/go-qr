package go_qr

type BitSet interface {
	GetBit(i int) bool
	Set(i int, value bool)
	Len() int
}

type BitBuffer []bool

func (b *BitBuffer) Len() int {
	return len(*b)
}

func (b *BitBuffer) Set(i int, value bool) {
	if i >= len(*b) {
		b.grow(1 + i)
	}
	(*b)[i] = value
}

func (b *BitBuffer) GetBit(i int) bool {
	if i >= len(*b) {
		return false
	}
	return (*b)[i]
}

func (b *BitBuffer) grow(size int) {
	res := make(BitBuffer, size)
	copy(res, *b)
	*b = res
}
