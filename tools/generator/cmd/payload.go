package cmd

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/piglig/go-qr/v2/payload"
)

// payloadUsage documents the keys of each -payload type.
const payloadUsage = `Payload keys (-payload TYPE, content "key=value,key=value"):
  wifi     ssid, password, auth (WPA, WEP, nopass), hidden (true)
  vcard    name, phone, email, url, address, org, note        (MECARD)
  contact  name, given, family, org, title, phone, email, url, address, note
           (vCard 3.0; separate several phones or emails with ;)
  event    summary, location, description, start, end, allday (true)
           (times: RFC 3339, 2006-01-02T15:04 in UTC, or 2006-01-02 for all-day)
  otp      issuer, account, secret, type (totp, hotp), algorithm, digits, period, counter
  epc      name, iban, bic, amount (euros, e.g. 12.50), purpose, reference, text, info
  email    to, subject, body, cc, bcc (separate several addresses with ;)
  sms      number, body
  tel      number
  geo      lat, lon, query
  url      href
Escape a literal comma or equals sign with a backslash.
`

// resolveContent returns content itself, or the payload string built from
// its key=value pairs when payloadType is set.
func resolveContent(content, payloadType string) (string, error) {
	if payloadType == "" {
		return content, nil
	}
	kv, err := parseKV(content)
	if err != nil {
		return "", err
	}
	list := func(key string) []string {
		if kv[key] == "" {
			return nil
		}
		return strings.Split(kv[key], ";")
	}
	atoi := func(key string) (int, error) {
		if kv[key] == "" {
			return 0, nil
		}
		n, err := strconv.Atoi(kv[key])
		if err != nil {
			return 0, fmt.Errorf("%s %q: not a number", key, kv[key])
		}
		return n, nil
	}

	switch strings.ToLower(payloadType) {
	case "wifi":
		return payload.WiFi{
			SSID:     kv["ssid"],
			Password: kv["password"],
			Auth:     payload.WiFiAuth(kv["auth"]),
			Hidden:   kv["hidden"] == "true",
		}.String(), nil
	case "vcard":
		return payload.VCard{
			Name: kv["name"], Phone: kv["phone"], Email: kv["email"],
			URL: kv["url"], Address: kv["address"], Org: kv["org"], Note: kv["note"],
		}.String(), nil
	case "contact":
		return payload.Contact{
			Name: kv["name"], GivenName: kv["given"], FamilyName: kv["family"],
			Org: kv["org"], Title: kv["title"], Phones: list("phone"), Emails: list("email"),
			URL: kv["url"], Address: kv["address"], Note: kv["note"],
		}.String(), nil
	case "event":
		e := payload.Event{Summary: kv["summary"], Location: kv["location"], Description: kv["description"]}
		var dateOnly bool
		if e.Start, dateOnly, err = parseTime(kv["start"]); err != nil {
			return "", fmt.Errorf("start: %w", err)
		}
		if kv["end"] != "" {
			if e.End, _, err = parseTime(kv["end"]); err != nil {
				return "", fmt.Errorf("end: %w", err)
			}
		}
		e.AllDay = dateOnly || kv["allday"] == "true"
		return e.String(), nil
	case "otp":
		o := payload.OTP{
			Type: payload.OTPType(strings.ToLower(kv["type"])), Issuer: kv["issuer"], Account: kv["account"],
			Secret: kv["secret"], Algorithm: payload.OTPAlgorithm(strings.ToUpper(kv["algorithm"])),
		}
		if o.Secret == "" {
			return "", fmt.Errorf("otp: secret is required")
		}
		if o.Type != "" && o.Type != payload.TOTP && o.Type != payload.HOTP {
			return "", fmt.Errorf("otp: type %q (expected totp or hotp)", kv["type"])
		}
		if o.Digits, err = atoi("digits"); err != nil {
			return "", err
		}
		if o.Period, err = atoi("period"); err != nil {
			return "", err
		}
		counter, err := atoi("counter")
		if err != nil || counter < 0 {
			return "", fmt.Errorf("counter %q: not a non-negative number", kv["counter"])
		}
		o.Counter = uint64(counter)
		return o.String(), nil
	case "epc":
		e := payload.EPC{
			Name: kv["name"], IBAN: kv["iban"], BIC: kv["bic"], Purpose: kv["purpose"],
			Reference: kv["reference"], Text: kv["text"], Info: kv["info"],
		}
		if kv["amount"] != "" {
			if e.Amount, err = parseEuros(kv["amount"]); err != nil {
				return "", err
			}
		}
		if err := e.Validate(); err != nil {
			return "", err
		}
		return e.String(), nil
	case "email":
		return payload.Email{To: kv["to"], Subject: kv["subject"], Body: kv["body"], CC: list("cc"), BCC: list("bcc")}.String(), nil
	case "sms":
		return payload.SMS{Number: kv["number"], Body: kv["body"]}.String(), nil
	case "tel":
		return payload.Tel{Number: kv["number"]}.String(), nil
	case "geo":
		lat, err := strconv.ParseFloat(kv["lat"], 64)
		if err != nil {
			return "", fmt.Errorf("geo lat %q: %w", kv["lat"], err)
		}
		lon, err := strconv.ParseFloat(kv["lon"], 64)
		if err != nil {
			return "", fmt.Errorf("geo lon %q: %w", kv["lon"], err)
		}
		return payload.Geo{Lat: lat, Lon: lon, Query: kv["query"]}.String(), nil
	case "url":
		return payload.URL{Href: kv["href"]}.String(), nil
	default:
		return "", fmt.Errorf("unknown payload type %q", payloadType)
	}
}

// parseTime parses an event time and reports whether it was a bare date.
func parseTime(s string) (time.Time, bool, error) {
	if s == "" {
		return time.Time{}, false, fmt.Errorf("missing time")
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, false, nil
	}
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02T15:04", "2006-01-02 15:04"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, false, nil
		}
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, true, nil
	}
	return time.Time{}, false, fmt.Errorf("%q is not RFC 3339, 2006-01-02T15:04 or 2006-01-02", s)
}

// parseEuros parses an amount such as "12", "12.5" or "12.50" into cents.
func parseEuros(s string) (int64, error) {
	bad := fmt.Errorf("amount %q: want euros with at most two decimals, such as 12.50", s)
	whole, frac, _ := strings.Cut(strings.TrimSpace(s), ".")
	if len(frac) > 2 {
		return 0, bad
	}
	frac += "00"[:2-len(frac)]
	w, err1 := strconv.ParseInt(whole, 10, 64)
	f, err2 := strconv.ParseInt(frac, 10, 64)
	if err1 != nil || err2 != nil || w < 0 || f < 0 || w > math.MaxInt64/100-1 {
		return 0, bad
	}
	return w*100 + f, nil
}

// parseKV parses `key=val,key=val`. Backslashes escape the next character,
// allowing literal commas and equals signs inside values.
func parseKV(s string) (map[string]string, error) {
	out := map[string]string{}
	if s == "" {
		return out, nil
	}
	var key, val strings.Builder
	onKey := true
	flush := func() {
		k := strings.ToLower(strings.TrimSpace(key.String()))
		if k != "" {
			out[k] = val.String()
		}
		key.Reset()
		val.Reset()
		onKey = true
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '\\' && i+1 < len(s) {
			i++
			if onKey {
				key.WriteByte(s[i])
			} else {
				val.WriteByte(s[i])
			}
			continue
		}
		switch c {
		case '=':
			if onKey {
				onKey = false
			} else {
				val.WriteByte(c)
			}
		case ',':
			flush()
		default:
			if onKey {
				key.WriteByte(c)
			} else {
				val.WriteByte(c)
			}
		}
	}
	flush()
	return out, nil
}
