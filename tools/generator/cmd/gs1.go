package cmd

import (
	"fmt"
	"strings"
)

// gs1FixedLength lists the two-digit prefixes of application identifiers
// whose element strings have a predefined length, so no separator follows
// them (GS1 General Specifications, figure 7.8.5-2).
var gs1FixedLength = map[string]bool{
	"00": true, "01": true, "02": true, "03": true, "04": true,
	"11": true, "12": true, "13": true, "14": true, "15": true, "16": true, "17": true, "18": true, "19": true,
	"20": true, "31": true, "32": true, "33": true, "34": true, "35": true, "36": true, "41": true,
}

// gs1FromHRI converts the human-readable form printed under GS1 barcodes,
// such as "(01)09501101530003(10)ABC123(21)XYZ", into the element string
// QR Codes carry: identifiers and values concatenated, with the GS
// separator after every variable-length element except the last. Content
// without parentheses is returned unchanged.
func gs1FromHRI(s string) (string, error) {
	if !strings.HasPrefix(s, "(") {
		return s, nil
	}
	var sb strings.Builder
	for rest := s; rest != ""; {
		if rest[0] != '(' {
			return "", fmt.Errorf("gs1: expected '(' at %q", rest)
		}
		ai, after, ok := strings.Cut(rest[1:], ")")
		if !ok || len(ai) < 2 || len(ai) > 4 || strings.Trim(ai, "0123456789") != "" {
			return "", fmt.Errorf("gs1: invalid application identifier in %q", rest)
		}
		value := after
		if i := strings.IndexByte(after, '('); i >= 0 {
			value = after[:i]
		}
		if value == "" {
			return "", fmt.Errorf("gs1: empty value for AI (%s)", ai)
		}
		rest = after[len(value):]
		sb.WriteString(ai)
		sb.WriteString(value)
		if rest != "" && !gs1FixedLength[ai[:2]] {
			sb.WriteByte(0x1D)
		}
	}
	return sb.String(), nil
}
