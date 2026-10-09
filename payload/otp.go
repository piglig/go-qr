package payload

import (
	"net/url"
	"strconv"
	"strings"
)

// OTPType selects time-based or counter-based one-time passwords.
type OTPType string

const (
	TOTP OTPType = "totp" // time-based (RFC 6238), the common case
	HOTP OTPType = "hotp" // counter-based (RFC 4226)
)

// OTPAlgorithm is the HMAC hash of an OTP.
type OTPAlgorithm string

const (
	SHA1   OTPAlgorithm = "SHA1"
	SHA256 OTPAlgorithm = "SHA256"
	SHA512 OTPAlgorithm = "SHA512"
)

// OTP is an otpauth:// URI that enrolls an account in an authenticator app
// such as Google Authenticator, 1Password or Authy.
//
// Zero values are omitted, so the app applies its defaults: TOTP, SHA1, six
// digits and a 30-second period. Issuer and Account must not contain ':'.
//
// Format reference: https://github.com/google/google-authenticator/wiki/Key-Uri-Format
type OTP struct {
	Type      OTPType // default TOTP
	Issuer    string  // service name, shown above the code
	Account   string  // user name or email
	Secret    string  // shared secret in base32; spaces and padding are dropped
	Algorithm OTPAlgorithm
	Digits    int    // code length, usually 6 or 8
	Period    int    // TOTP step in seconds
	Counter   uint64 // HOTP initial counter; always written for HOTP
}

func (o OTP) String() string {
	typ := o.Type
	if typ == "" {
		typ = TOTP
	}
	label := url.PathEscape(o.Account)
	if o.Issuer != "" {
		label = url.PathEscape(o.Issuer) + ":" + label
	}

	q := url.Values{}
	q.Set("secret", normalizeSecret(o.Secret))
	if o.Issuer != "" {
		q.Set("issuer", o.Issuer)
	}
	if o.Algorithm != "" {
		q.Set("algorithm", string(o.Algorithm))
	}
	if o.Digits != 0 {
		q.Set("digits", strconv.Itoa(o.Digits))
	}
	if o.Period != 0 && typ == TOTP {
		q.Set("period", strconv.Itoa(o.Period))
	}
	if typ == HOTP {
		q.Set("counter", strconv.FormatUint(o.Counter, 10))
	}
	return "otpauth://" + string(typ) + "/" + label + "?" + encodeQuery(q)
}

// normalizeSecret uppercases a base32 secret and drops spaces and padding,
// which authenticator apps reject.
func normalizeSecret(s string) string {
	s = strings.ToUpper(strings.ReplaceAll(s, " ", ""))
	return strings.TrimRight(s, "=")
}
