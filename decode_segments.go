package go_qr

import "fmt"

// bitReader reads big-endian bits out of the corrected data codewords.
type bitReader struct {
	data []byte
	pos  int // bit position
}

func (r *bitReader) remaining() int { return len(r.data)*8 - r.pos }

// read returns the next n bits as an int (n <= 32), or false if exhausted.
func (r *bitReader) read(n int) (int, bool) {
	if n > r.remaining() {
		return 0, false
	}
	v := 0
	for i := 0; i < n; i++ {
		bit := 0
		if (r.data[r.pos>>3]>>uint(7-(r.pos&7)))&1 != 0 {
			bit = 1
		}
		v = v<<1 | bit
		r.pos++
	}
	return v, true
}

// parseBitstream walks the segment structure (reverse of EncodeSegments) and
// reconstructs the original string. Supports numeric, alphanumeric, byte, and
// ECI (skipped) modes; kanji is reported as unsupported for now.
func parseBitstream(data []byte, ver int) (string, []SegmentInfo, error) {
	r := &bitReader{data: data}
	var out []byte
	var segs []SegmentInfo

	for r.remaining() >= 4 {
		modeBits, ok := r.read(4)
		if !ok || modeBits == 0 {
			break // terminator or exhausted
		}

		var mode Mode
		switch modeBits {
		case Numeric.modeBits:
			mode = Numeric
		case Alphanumeric.modeBits:
			mode = Alphanumeric
		case Byte.modeBits:
			mode = Byte
		case Eci.modeBits:
			// ECI: read the assignment number and ignore it (byte mode here is
			// already raw bytes / UTF-8 from the encoder).
			if _, err := readECI(r); err != nil {
				return "", nil, err
			}
			continue
		case Kanji.modeBits:
			return "", nil, fmt.Errorf("%w: kanji segment decode not yet implemented", ErrUnsupportedSymbol)
		default:
			return "", nil, fmt.Errorf("%w: unknown mode 0x%x", ErrDecodeFailed, modeBits)
		}

		count, ok := r.read(mode.numCharCountBits(ver))
		if !ok {
			return "", nil, fmt.Errorf("%w: truncated char count", ErrDecodeFailed)
		}

		start := len(out)
		var err error
		switch {
		case mode.isNumeric():
			out, err = readNumeric(r, count, out)
		case mode.isAlphanumeric():
			out, err = readAlphanumeric(r, count, out)
		case mode.isByte():
			out, err = readByte(r, count, out)
		}
		if err != nil {
			return "", nil, err
		}
		segs = append(segs, SegmentInfo{Mode: modeBits, NumChars: count, Bytes: append([]byte(nil), out[start:]...)})
	}
	return string(out), segs, nil
}

func readECI(r *bitReader) (int, error) {
	first, ok := r.read(8)
	if !ok {
		return 0, fmt.Errorf("%w: truncated ECI", ErrDecodeFailed)
	}
	switch {
	case first < 0x80:
		return first, nil
	case first < 0xC0:
		rest, ok := r.read(8)
		if !ok {
			return 0, fmt.Errorf("%w: truncated ECI", ErrDecodeFailed)
		}
		return (first&0x3F)<<8 | rest, nil
	default:
		rest, ok := r.read(16)
		if !ok {
			return 0, fmt.Errorf("%w: truncated ECI", ErrDecodeFailed)
		}
		return (first&0x1F)<<16 | rest, nil
	}
}

func readNumeric(r *bitReader, count int, out []byte) ([]byte, error) {
	for count > 0 {
		n := count
		if n > 3 {
			n = 3
		}
		bits := n*3 + 1
		v, ok := r.read(bits)
		if !ok {
			return nil, fmt.Errorf("%w: truncated numeric", ErrDecodeFailed)
		}
		// zero-pad to n digits
		digits := []byte(fmt.Sprintf("%0*d", n, v))
		out = append(out, digits...)
		count -= n
	}
	return out, nil
}

func readAlphanumeric(r *bitReader, count int, out []byte) ([]byte, error) {
	for count >= 2 {
		v, ok := r.read(11)
		if !ok {
			return nil, fmt.Errorf("%w: truncated alphanumeric", ErrDecodeFailed)
		}
		out = append(out, alphanumericCharset[v/45], alphanumericCharset[v%45])
		count -= 2
	}
	if count == 1 {
		v, ok := r.read(6)
		if !ok {
			return nil, fmt.Errorf("%w: truncated alphanumeric", ErrDecodeFailed)
		}
		out = append(out, alphanumericCharset[v])
	}
	return out, nil
}

func readByte(r *bitReader, count int, out []byte) ([]byte, error) {
	for i := 0; i < count; i++ {
		v, ok := r.read(8)
		if !ok {
			return nil, fmt.Errorf("%w: truncated byte", ErrDecodeFailed)
		}
		out = append(out, byte(v))
	}
	return out, nil
}
