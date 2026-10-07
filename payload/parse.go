package payload

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrUnrecognized reports text that is not in any format Parse knows.
	ErrUnrecognized = errors.New("payload: unrecognized format")

	// ErrMalformed reports text that starts like a known format but does not
	// follow it.
	ErrMalformed = errors.New("payload: malformed payload")
)

// Parse recognizes a payload string, typically the text of a decoded QR
// Code, and returns it as one of this package's types: WiFi, VCard (MECARD),
// Contact (vCard), Email, SMS (sms: and SMSTO:), Tel, Geo, OTP, Event, EPC or
// URL (http and https only). Scheme names are case-insensitive.
func Parse(s string) (Payload, error) {
	prefix := func(p string) bool { return len(s) >= len(p) && strings.EqualFold(s[:len(p)], p) }
	switch {
	case prefix("WIFI:"):
		return parseWiFi(s[5:])
	case prefix("MECARD:"):
		return parseMeCard(s[7:])
	case prefix("BEGIN:VCARD"):
		return parseContact(s)
	case prefix("BEGIN:VEVENT"), prefix("BEGIN:VCALENDAR"):
		return parseEvent(s)
	case prefix("BCD\n"), prefix("BCD\r\n"):
		return parseEPC(s)
	case prefix("mailto:"):
		return parseEmail(s)
	case prefix("smsto:"):
		number, body, _ := strings.Cut(s[6:], ":")
		return SMS{Number: number, Body: body}, nil
	case prefix("sms:"):
		return parseSMS(s[4:])
	case prefix("tel:"):
		return Tel{Number: s[4:]}, nil
	case prefix("geo:"):
		return parseGeo(s[4:])
	case prefix("otpauth://"):
		return parseOTP(s)
	case prefix("http://"), prefix("https://"):
		return URL{Href: s}, nil
	}
	return nil, ErrUnrecognized
}

func malformed(kind, format string, args ...any) error {
	return fmt.Errorf("%w: %s: %s", ErrMalformed, kind, fmt.Sprintf(format, args...))
}

// meCardFields splits "K:v;K:v;;" into key/value pairs, honoring backslash
// escapes in values. Keys are uppercased.
func meCardFields(body string) [][2]string {
	var fields [][2]string
	for body != "" && body != ";" {
		key, rest, ok := strings.Cut(body, ":")
		if !ok {
			break
		}
		var val strings.Builder
		i := 0
		for ; i < len(rest) && rest[i] != ';'; i++ {
			if rest[i] == '\\' && i+1 < len(rest) {
				i++
			}
			val.WriteByte(rest[i])
		}
		fields = append(fields, [2]string{strings.ToUpper(key), val.String()})
		if i >= len(rest) {
			break
		}
		body = rest[i+1:]
	}
	return fields
}

func parseWiFi(body string) (Payload, error) {
	var w WiFi
	for _, f := range meCardFields(body) {
		switch f[0] {
		case "T":
			w.Auth = WiFiAuth(f[1])
		case "S":
			w.SSID = f[1]
		case "P":
			w.Password = f[1]
		case "H":
			w.Hidden = strings.EqualFold(f[1], "true")
		}
	}
	if w.SSID == "" {
		return nil, malformed("wifi", "missing SSID")
	}
	return w, nil
}

func parseMeCard(body string) (Payload, error) {
	var v VCard
	for _, f := range meCardFields(body) {
		dst := map[string]*string{
			"N": &v.Name, "TEL": &v.Phone, "EMAIL": &v.Email, "URL": &v.URL,
			"ADR": &v.Address, "ORG": &v.Org, "NOTE": &v.Note,
		}[f[0]]
		if dst != nil && *dst == "" {
			*dst = f[1]
		}
	}
	return v, nil
}

func parseEmail(s string) (Payload, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, malformed("mailto", "%v", err)
	}
	to, err := url.PathUnescape(u.Opaque)
	if err != nil {
		return nil, malformed("mailto", "%v", err)
	}
	q := u.Query()
	e := Email{To: to, Subject: q.Get("subject"), Body: q.Get("body")}
	if cc := q.Get("cc"); cc != "" {
		e.CC = strings.Split(cc, ",")
	}
	if bcc := q.Get("bcc"); bcc != "" {
		e.BCC = strings.Split(bcc, ",")
	}
	return e, nil
}

func parseSMS(rest string) (Payload, error) {
	number, query, _ := strings.Cut(rest, "?")
	q, err := url.ParseQuery(query)
	if err != nil {
		return nil, malformed("sms", "%v", err)
	}
	return SMS{Number: number, Body: q.Get("body")}, nil
}

func parseGeo(rest string) (Payload, error) {
	coords, query, _ := strings.Cut(rest, "?")
	coords, _, _ = strings.Cut(coords, ";") // drop RFC 5870 parameters
	parts := strings.Split(coords, ",")
	if len(parts) < 2 {
		return nil, malformed("geo", "want lat,lon in %q", coords)
	}
	lat, err1 := strconv.ParseFloat(parts[0], 64)
	lon, err2 := strconv.ParseFloat(parts[1], 64)
	if err := errors.Join(err1, err2); err != nil {
		return nil, malformed("geo", "%v", err)
	}
	q, err := url.ParseQuery(query)
	if err != nil {
		return nil, malformed("geo", "%v", err)
	}
	return Geo{Lat: lat, Lon: lon, Query: q.Get("q")}, nil
}

func parseOTP(s string) (Payload, error) {
	u, err := url.Parse(s)
	if err != nil {
		return nil, malformed("otpauth", "%v", err)
	}
	o := OTP{Type: OTPType(strings.ToLower(u.Host))}
	if o.Type != TOTP && o.Type != HOTP {
		return nil, malformed("otpauth", "unknown type %q", u.Host)
	}
	label := strings.TrimPrefix(u.Path, "/")
	if issuer, account, ok := strings.Cut(label, ":"); ok {
		o.Issuer, o.Account = issuer, strings.TrimLeft(account, " ")
	} else {
		o.Account = label
	}

	q := u.Query()
	if o.Secret = q.Get("secret"); o.Secret == "" {
		return nil, malformed("otpauth", "missing secret")
	}
	if issuer := q.Get("issuer"); issuer != "" {
		o.Issuer = issuer
	}
	o.Algorithm = OTPAlgorithm(strings.ToUpper(q.Get("algorithm")))
	for key, dst := range map[string]*int{"digits": &o.Digits, "period": &o.Period} {
		if v := q.Get(key); v != "" {
			if *dst, err = strconv.Atoi(v); err != nil {
				return nil, malformed("otpauth", "%s: %v", key, err)
			}
		}
	}
	if v := q.Get("counter"); v != "" {
		if o.Counter, err = strconv.ParseUint(v, 10, 64); err != nil {
			return nil, malformed("otpauth", "counter: %v", err)
		}
	}
	return o, nil
}

// contentLines splits an RFC 2425 / RFC 5545 content block into
// (name with parameters, value) pairs, unfolding continuation lines.
func contentLines(s string) [][2]string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\n ", "")
	s = strings.ReplaceAll(s, "\n\t", "")
	var out [][2]string
	for _, l := range strings.Split(s, "\n") {
		if name, value, ok := strings.Cut(l, ":"); ok {
			out = append(out, [2]string{strings.ToUpper(name), value})
		}
	}
	return out
}

// unescapeText reverses vCard and iCalendar text escaping.
func unescapeText(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			if s[i] == 'n' || s[i] == 'N' {
				sb.WriteByte('\n')
				continue
			}
		}
		sb.WriteByte(s[i])
	}
	return sb.String()
}

// splitStructured splits a structured value on unescaped semicolons.
func splitStructured(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case ';':
			parts = append(parts, unescapeText(s[start:i]))
			start = i + 1
		}
	}
	return append(parts, unescapeText(s[start:]))
}

func parseContact(s string) (Payload, error) {
	var c Contact
	for _, l := range contentLines(s) {
		name, _, _ := strings.Cut(l[0], ";") // drop parameters such as TYPE=CELL
		switch name {
		case "FN":
			c.Name = unescapeText(l[1])
		case "N":
			if parts := splitStructured(l[1]); len(parts) > 1 {
				c.FamilyName, c.GivenName = parts[0], parts[1]
			} else {
				c.FamilyName = parts[0]
			}
		case "ORG":
			c.Org = unescapeText(l[1])
		case "TITLE":
			c.Title = unescapeText(l[1])
		case "TEL":
			c.Phones = append(c.Phones, unescapeText(l[1]))
		case "EMAIL":
			c.Emails = append(c.Emails, unescapeText(l[1]))
		case "URL":
			c.URL = unescapeText(l[1])
		case "ADR":
			var nonEmpty []string
			for _, p := range splitStructured(l[1]) {
				if p != "" {
					nonEmpty = append(nonEmpty, p)
				}
			}
			c.Address = strings.Join(nonEmpty, ", ")
		case "NOTE":
			c.Note = unescapeText(l[1])
		}
	}
	// A family name that only repeats the formatted name was synthesized
	// from it by Contact.String.
	if c.GivenName == "" && c.FamilyName == c.Name {
		c.FamilyName = ""
	}
	return c, nil
}

func parseEvent(s string) (Payload, error) {
	var e Event
	var hasStart bool
	for _, l := range contentLines(s) {
		name, params, _ := strings.Cut(l[0], ";")
		switch name {
		case "SUMMARY":
			e.Summary = unescapeText(l[1])
		case "LOCATION":
			e.Location = unescapeText(l[1])
		case "DESCRIPTION":
			e.Description = unescapeText(l[1])
		case "DTSTART", "DTEND":
			t, allDay, err := parseICalTime(l[1], params)
			if err != nil {
				return nil, malformed("event", "%s: %v", name, err)
			}
			if name == "DTSTART" {
				e.Start, e.AllDay, hasStart = t, allDay, true
			} else {
				e.End = t
			}
		}
	}
	if !hasStart {
		return nil, malformed("event", "missing DTSTART")
	}
	return e, nil
}

// parseICalTime parses a DATE or DATE-TIME value. Floating and TZID times
// are read as UTC.
func parseICalTime(v, params string) (t time.Time, allDay bool, err error) {
	if len(v) == len(icalDate) || strings.EqualFold(params, "VALUE=DATE") {
		t, err = time.Parse(icalDate, v)
		return t, true, err
	}
	t, err = time.Parse("20060102T150405", strings.TrimSuffix(v, "Z"))
	return t, false, err
}

func parseEPC(s string) (Payload, error) {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	field := func(i int) string {
		if i < len(lines) {
			return strings.TrimSpace(lines[i])
		}
		return ""
	}
	if v := field(1); v != "001" && v != "002" {
		return nil, malformed("EPC", "unsupported version %q", v)
	}
	if field(3) != "SCT" {
		return nil, malformed("EPC", "identification %q is not SCT", field(3))
	}
	e := EPC{
		BIC:       field(4),
		Name:      field(5),
		IBAN:      field(6),
		Purpose:   field(8),
		Reference: field(9),
		Text:      field(10),
		Info:      field(11),
	}
	if amount := field(7); amount != "" {
		cents, err := parseEuroCents(amount)
		if err != nil {
			return nil, malformed("EPC", "amount %q: %v", amount, err)
		}
		e.Amount = cents
	}
	return e, nil
}

// parseEuroCents parses "EUR12.3" or "EUR12.34" into cents.
func parseEuroCents(s string) (int64, error) {
	if !strings.HasPrefix(s, "EUR") {
		return 0, errors.New("missing EUR prefix")
	}
	whole, frac, _ := strings.Cut(s[3:], ".")
	if len(frac) > 2 {
		return 0, errors.New("more than two decimals")
	}
	frac += "00"[:2-len(frac)]
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, err
	}
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, err
	}
	return w*100 + f, nil
}
