# Encoding

`Encode` turns text into a `*qr.Code`, an immutable symbol you can render or
inspect. With no options it makes every choice for you:

```go
code, err := qr.Encode("https://example.com/order/12345")
fmt.Println(code.Version(), code.ECC(), code.Mask(), code.Size())
```

- **Version:** the smallest of the 40 sizes (21×21 to 177×177 modules) that
  holds the data.
- **Segments:** the text is split into numeric, alphanumeric, byte and Kanji
  segments wherever switching mode saves bits. "ORDER-12345678" is cheaper as
  an alphanumeric and a numeric segment than as bytes.
- **Error correction:** starts at level M and is raised to Q or H when the
  chosen version has room to spare, at no cost in size.
- **Mask:** the pattern with the lowest ISO/IEC 18004 penalty score.

`Encode` returns an error wrapping `ErrDataTooLong` when the data does not
fit in version 40 at the requested level; see [Errors](../reference/errors.md).

## Options

| Option | Default | Effect |
| --- | --- | --- |
| `WithECC(level)` | `ECCMedium` | Minimum error correction level: `ECCLow` (~7% recovery), `ECCMedium` (~15%), `ECCQuartile` (~25%), `ECCHigh` (~30%). |
| `WithoutECCBoost()` | boost on | Keep exactly the `WithECC` level instead of raising it into spare capacity. |
| `WithVersionRange(min, max)` | 1, 40 | Restrict the version; pass the same value twice to fix it. |
| `WithMask(m)` | lowest penalty | Force mask pattern 0–7. |
| `WithSimpleSegmentation()` | optimal | Encode the whole text in one mode. Faster, but larger for mixed content. |
| `WithUTF8ECI()` | off | Declare UTF-8 with an ECI designator when the text is not ASCII. |
| `WithGS1()` *Since v2.1* | off | Mark the data as a GS1 element string. |

### Choosing an error correction level

Higher levels survive more damage but need a larger symbol for the same data.

- **L** for clean digital displays and maximum capacity.
- **M**, the default, for most printed codes.
- **Q** or **H** for codes that may be scratched, partly covered, or carry a
  logo. A logo needs H to be more than small; see
  [Rendering](rendering.md#logos).

### Text encodings and ECI

Byte segments carry UTF-8. Nearly every phone scanner reads them as UTF-8,
but ISO/IEC 18004 says readers should assume ISO-8859-1 unless an ECI
designator says otherwise. `WithUTF8ECI()` adds that designator (12 extra
bits) when the text is not plain ASCII. Use it for codes read by industrial
or strict scanners.

## Binary data and explicit segments

`EncodeBytes` stores arbitrary bytes in a single byte segment:

```go
code, err := qr.EncodeBytes(payload, qr.WithECC(qr.ECCQuartile))
```

To control the encoding exactly, build the segments yourself and pass them
to `EncodeSegments`, which encodes them in order without re-segmenting:

```go
prefix, _ := qr.AlphanumericSegment("ORDER-")
number, _ := qr.NumericSegment("0123456789")
name, _ := qr.KanjiSegment("山田")
code, err := qr.EncodeSegments([]qr.Segment{prefix, number, name})
```

| Constructor | Accepts |
| --- | --- |
| `NumericSegment(s)` | ASCII digits |
| `AlphanumericSegment(s)` | `0-9`, `A-Z` (uppercase), space and `$%*+-./:` |
| `BytesSegment(b)` | any bytes |
| `KanjiSegment(s)` | characters in the Shift_JIS double-byte range of QR Kanji mode |
| `ECISegment(n)` | an ECI assignment number, such as 26 for UTF-8 |

The constructors return `ErrUnencodableChar` for characters their mode
cannot hold.

## GS1 element strings

*Since v2.1.* Logistics and retail systems read GS1 data: application
identifiers followed by their values, such as a GTIN, an expiry date and a
batch number. `WithGS1()` writes the FNC1 indicator that marks the symbol as
GS1. Concatenate the elements without the human-readable parentheses, and end
each variable-length element that is not last with the ASCII GS character
(`\x1d`):

```go
// (01) 09501101530003, (17) 250101, (10) ABC123, (21) XYZ
data := "0109501101530003" + "17250101" + "10ABC123\x1d" + "21XYZ"
code, err := qr.Encode(data, qr.WithGS1())
```

The decoder reports these symbols with `DecodeResult.GS1` set.

## Data too long for one symbol

*Since v2.1.* `EncodeStructured` spreads text over up to 16 symbols that each
fit the options, and links them with structured append headers:

```go
codes, err := qr.EncodeStructured(longText, qr.WithECC(qr.ECCMedium), qr.WithVersionRange(1, 10))
```

Readers that support structured append combine the symbols themselves. With
this library's decoder, decode every symbol, in any order, and join them:

```go
results := make([]*qr.DecodeResult, len(images))
for i, img := range images {
	if results[i], err = qr.Decode(img); err != nil {
		return err
	}
}
text, err := qr.JoinStructuredAppend(results...)
```

Text that fits in one symbol yields a single code without a header. The split
happens between characters, so every symbol holds valid UTF-8.

## Inspecting a code

A `*qr.Code` reports `Version`, `ECC`, `Mask` and `Size`, and
`Module(x, y)` tells whether a module is dark, so you can draw the symbol
with any graphics library:

```go
for y := 0; y < code.Size(); y++ {
	for x := 0; x < code.Size(); x++ {
		if code.Module(x, y) {
			canvas.FillRect(x, y, 1, 1)
		}
	}
}
```

Coordinates outside the symbol report light modules, which matches the
quiet zone.
