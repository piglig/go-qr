# Payloads

Phone scanners act on certain text formats: they join a Wi-Fi network,
offer to save a contact, or open an authenticator app. The `payload` package
builds these strings and parses them back.

```go
import "github.com/piglig/go-qr/v2/payload"

wifi := payload.WiFi{SSID: "home", Password: "s3cret", Auth: payload.WPA}
code, err := qr.Encode(wifi.String())
```

Every type implements `payload.Payload` (a `String() string` method) and
escapes reserved characters for you.

## Types

| Type | Format | Scanner action |
| --- | --- | --- |
| `WiFi` | `WIFI:` | Join a network. `Auth` is `WPA`, `WEP` or `NoPass`; `Hidden` for hidden networks. |
| `VCard` | `MECARD:` | Save a contact. Compact and widely supported, one phone and one email. |
| `Contact` *Since v2.1* | vCard 3.0 | Save a contact with several phones and emails, a title and an address. |
| `Event` *Since v2.1* | iCalendar `VEVENT` | Add a calendar event; timed or all-day. |
| `OTP` *Since v2.1* | `otpauth://` | Enroll a 2FA account in an authenticator app (TOTP or HOTP). |
| `EPC` *Since v2.1* | EPC069-12 ("GiroCode") | Prefill a SEPA credit transfer in a banking app. |
| `Email` | `mailto:` | Compose an email with subject, body, CC and BCC. |
| `SMS` | `sms:` | Compose a text message. |
| `Tel` | `tel:` | Call a number. |
| `Geo` | `geo:` | Open a map at coordinates, optionally with a search query. |
| `URL` | the URL itself | Open a page. |

## Examples

**Two-factor authentication.** Zero values are omitted, so the app applies
its defaults (TOTP, SHA-1, six digits, 30 seconds):

```go
otp := payload.OTP{Issuer: "Example", Account: "alice@example.com", Secret: secretBase32}
code, err := qr.Encode(otp.String(), qr.WithECC(qr.ECCQuartile))
```

**Calendar event.** Times are written in UTC:

```go
event := payload.Event{
	Summary:  "Launch",
	Location: "Room 4",
	Start:    time.Date(2026, 10, 7, 18, 0, 0, 0, time.UTC),
	End:      time.Date(2026, 10, 7, 21, 0, 0, 0, time.UTC),
}
```

**SEPA payment.** Amounts are in cents to avoid floating point. The
specification requires at least error correction level M, and `Validate`
checks its field rules and the IBAN's check digits, so a mistyped IBAN is
caught before it is printed:

```go
pay := payload.EPC{Name: "Red Cross", IBAN: "BE72 0000 0000 1616", Amount: 2500, Text: "Donation"}
if err := pay.Validate(); err != nil {
	return err // wraps payload.ErrInvalidEPC
}
code, err := qr.Encode(pay.String(), qr.WithECC(qr.ECCMedium))
```

## Parsing decoded text

*Since v2.1.* `payload.Parse` recognizes every format above, plus common
variants written by other generators (`SMSTO:`, vCard parameters and folded
lines, iCalendar wrappers, EPC version 001):

```go
res, err := qr.Decode(img)
if err != nil {
	return err
}
p, err := payload.Parse(res.Text)
switch v := p.(type) {
case payload.WiFi:
	join(v.SSID, v.Password)
case payload.URL:
	open(v.Href)
}
```

`Parse` returns an error wrapping `payload.ErrUnrecognized` for plain text
and `payload.ErrMalformed` for a known format with broken content.
