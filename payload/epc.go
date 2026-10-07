package payload

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// EPC is a European Payments Council QR code ("GiroCode") that banking apps
// read to prefill a SEPA credit transfer. Encode it with at least
// qr.ECCMedium, as the specification requires.
//
// Format reference: EPC069-12, version 002.
type EPC struct {
	Name      string // beneficiary, up to 70 characters
	IBAN      string // spaces are removed
	BIC       string // optional within the EEA
	Amount    int64  // in euro cents; 0 lets the payer enter the amount
	Purpose   string // optional 4-letter purpose code, such as "CHAR"
	Reference string // structured creditor reference (ISO 11649); excludes Text
	Text      string // unstructured remittance information; excludes Reference
	Info      string // note to the payer, up to 70 characters
}

// ErrInvalidEPC reports EPC fields that violate the specification.
var ErrInvalidEPC = errors.New("payload: invalid EPC payment")

// maxEPCAmount is 999,999,999.99 EUR in cents.
const maxEPCAmount = 99999999999

// Validate checks the field lengths and constraints of the specification.
func (e EPC) Validate() error {
	n := utf8.RuneCountInString
	switch iban := normalizeIBAN(e.IBAN); {
	case e.Name == "" || n(e.Name) > 70:
		return fmt.Errorf("%w: name must be 1 to 70 characters", ErrInvalidEPC)
	case len(iban) < 15 || len(iban) > 34:
		return fmt.Errorf("%w: IBAN %q has an invalid length", ErrInvalidEPC, iban)
	case e.BIC != "" && len(e.BIC) != 8 && len(e.BIC) != 11:
		return fmt.Errorf("%w: BIC must be 8 or 11 characters", ErrInvalidEPC)
	case e.Amount < 0 || e.Amount > maxEPCAmount:
		return fmt.Errorf("%w: amount %d cents out of range", ErrInvalidEPC, e.Amount)
	case len(e.Purpose) > 4:
		return fmt.Errorf("%w: purpose code %q is longer than 4 characters", ErrInvalidEPC, e.Purpose)
	case e.Reference != "" && e.Text != "":
		return fmt.Errorf("%w: reference and text are mutually exclusive", ErrInvalidEPC)
	case n(e.Reference) > 35:
		return fmt.Errorf("%w: reference is longer than 35 characters", ErrInvalidEPC)
	case n(e.Text) > 140:
		return fmt.Errorf("%w: text is longer than 140 characters", ErrInvalidEPC)
	case n(e.Info) > 70:
		return fmt.Errorf("%w: info is longer than 70 characters", ErrInvalidEPC)
	}
	if len(e.String()) > 331 {
		return fmt.Errorf("%w: payload exceeds 331 bytes", ErrInvalidEPC)
	}
	return nil
}

func (e EPC) String() string {
	amount := ""
	if e.Amount > 0 {
		amount = fmt.Sprintf("EUR%d.%02d", e.Amount/100, e.Amount%100)
	}
	fields := []string{
		"BCD", "002", "1", "SCT",
		strings.ToUpper(strings.TrimSpace(e.BIC)),
		e.Name,
		normalizeIBAN(e.IBAN),
		amount,
		e.Purpose,
		e.Reference,
		e.Text,
		e.Info,
	}
	// Trailing empty fields are omitted.
	for len(fields) > 0 && fields[len(fields)-1] == "" {
		fields = fields[:len(fields)-1]
	}
	return strings.Join(fields, "\n")
}

func normalizeIBAN(s string) string {
	return strings.ToUpper(strings.ReplaceAll(s, " ", ""))
}
