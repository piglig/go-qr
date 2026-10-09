// Package payload builds and parses the structured strings that phone
// scanners act on: joining a Wi-Fi network, saving a contact, adding a
// calendar event, enrolling a 2FA authenticator, starting a SEPA transfer,
// and so on. Encode the result of String with qr.Encode:
//
//	wifi := payload.WiFi{SSID: "home", Password: "s3cret", Auth: payload.WPA}
//	code, err := qr.Encode(wifi.String())
//
// Parse turns decoded text back into one of these types.
package payload

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// Payload is implemented by every payload type in this package: WiFi, VCard,
// Contact, Email, SMS, Tel, Geo, URL, OTP, Event and EPC.
type Payload interface {
	String() string
}

// meCardEscaper escapes the characters reserved in MECARD and Wi-Fi
// payloads: \ ; , " :
var meCardEscaper = strings.NewReplacer(
	`\`, `\\`,
	`;`, `\;`,
	`,`, `\,`,
	`"`, `\"`,
	`:`, `\:`,
)

func escape(s string) string { return meCardEscaper.Replace(s) }

// WiFiAuth is the Wi-Fi authentication type.
type WiFiAuth string

const (
	WPA    WiFiAuth = "WPA"
	WEP    WiFiAuth = "WEP"
	NoPass WiFiAuth = "nopass"
)

// WiFi is a Wi-Fi network auto-join payload.
//
// Format reference: https://en.wikipedia.org/wiki/QR_code#Joining_a_Wi%E2%80%90Fi_network
type WiFi struct {
	SSID     string
	Password string
	Auth     WiFiAuth
	Hidden   bool
}

func (w WiFi) String() string {
	auth := w.Auth
	if auth == "" {
		if w.Password == "" {
			auth = NoPass
		} else {
			auth = WPA
		}
	}
	var sb strings.Builder
	sb.WriteString("WIFI:")
	fmt.Fprintf(&sb, "T:%s;", auth)
	fmt.Fprintf(&sb, "S:%s;", escape(w.SSID))
	if auth != NoPass {
		fmt.Fprintf(&sb, "P:%s;", escape(w.Password))
	}
	if w.Hidden {
		sb.WriteString("H:true;")
	}
	sb.WriteString(";")
	return sb.String()
}

// VCard is a minimal MECARD-style contact payload. MECARD is more compact than
// full vCard and has broader scanner support on mobile devices.
//
// Format reference: https://en.wikipedia.org/wiki/MeCard_(QR_code)
type VCard struct {
	Name    string // Surname,Given or free-form; the first comma separates the two
	Phone   string
	Email   string
	URL     string
	Address string
	Org     string
	Note    string
}

func (v VCard) String() string {
	var sb strings.Builder
	sb.WriteString("MECARD:")
	write := func(tag, val string) {
		if val != "" {
			fmt.Fprintf(&sb, "%s:%s;", tag, escape(val))
		}
	}
	// The first comma of a name separates the surname from the given name
	// (an escaped one would be part of the name), so it is written as is.
	if surname, given, ok := strings.Cut(v.Name, ","); ok {
		fmt.Fprintf(&sb, "N:%s,%s;", escape(surname), escape(given))
	} else {
		write("N", v.Name)
	}
	write("TEL", v.Phone)
	write("EMAIL", v.Email)
	write("URL", v.URL)
	write("ADR", v.Address)
	write("ORG", v.Org)
	write("NOTE", v.Note)
	sb.WriteString(";")
	return sb.String()
}

// Email is a mailto: payload. Body and Subject are percent-encoded.
type Email struct {
	To      string
	Subject string
	Body    string
	CC      []string
	BCC     []string
}

func (e Email) String() string {
	var sb strings.Builder
	sb.WriteString("mailto:")
	sb.WriteString(mailtoAddresses(e.To))
	params := url.Values{}
	if e.Subject != "" {
		params.Set("subject", e.Subject)
	}
	if e.Body != "" {
		params.Set("body", e.Body)
	}
	if len(e.CC) > 0 {
		params.Set("cc", strings.Join(e.CC, ","))
	}
	if len(e.BCC) > 0 {
		params.Set("bcc", strings.Join(e.BCC, ","))
	}
	if encoded := encodeQuery(params); encoded != "" {
		sb.WriteString("?")
		sb.WriteString(encoded)
	}
	return sb.String()
}

// SMS is an sms: payload.
type SMS struct {
	Number string
	Body   string
}

func (s SMS) String() string {
	if s.Body == "" {
		return "sms:" + s.Number
	}
	return "sms:" + s.Number + "?body=" + queryEscape(s.Body)
}

// Tel is a tel: payload.
type Tel struct {
	Number string
}

func (t Tel) String() string {
	return "tel:" + t.Number
}

// Geo is a geo:lat,lon payload per RFC 5870.
type Geo struct {
	Lat, Lon float64
	// Query, when non-empty, is appended as ?q=... (not standard but widely
	// supported by map apps for pins with labels).
	Query string
}

func (g Geo) String() string {
	// RFC 5870 numbers have no exponent, which %v writes for small and
	// large values.
	out := "geo:" + strconv.FormatFloat(g.Lat, 'f', -1, 64) + "," + strconv.FormatFloat(g.Lon, 'f', -1, 64)
	if g.Query != "" {
		out += "?q=" + queryEscape(g.Query)
	}
	return out
}

// URL wraps any URL string for clarity at the call site.
type URL struct {
	Href string
}

func (u URL) String() string { return u.Href }

// queryEscape escapes s for a URI query, spaces as %20. url.QueryEscape
// writes them as +, which form decoding reads as a space but RFC 3986, and
// with it mail and messaging apps reading mailto: and sms: URIs, as a
// literal plus sign.
func queryEscape(s string) string {
	return strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
}

// encodeQuery is url.Values.Encode with spaces as %20; see queryEscape.
func encodeQuery(v url.Values) string {
	return strings.ReplaceAll(v.Encode(), "+", "%20")
}

// mailtoAddresses percent-encodes the addresses of a mailto: URI (RFC 6068
// section 2): %, which starts an escape, ? and #, which end the addresses,
// spaces, control and non-ASCII bytes, which a URI cannot hold, and a
// leading /, which would make the addresses a path. The characters of
// addresses, commas between them included, stay as they are.
func mailtoAddresses(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b == '%' || b == '?' || b == '#' || b <= ' ' || b >= 0x7f || i == 0 && b == '/' {
			fmt.Fprintf(&sb, "%%%02X", b)
		} else {
			sb.WriteByte(b)
		}
	}
	return sb.String()
}
