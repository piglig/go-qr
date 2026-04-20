// Package payload provides constructors for structured QR code payloads.
//
// These types produce canonical strings consumable by standard QR code
// scanners (e.g. Wi-Fi auto-join, contact import, mail compose). Pass the
// result to go_qr.EncodeText.
//
//	wifi := payload.WiFi{SSID: "home", Password: "s3cret", Auth: payload.WPA}
//	qr, _ := go_qr.EncodeText(wifi.String(), go_qr.Medium)
package payload

import (
	"fmt"
	"net/url"
	"strings"
)

// escape escapes the reserved characters for MECARD/WiFi-style payloads:
// \ ; , " :
func escape(s string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`;`, `\;`,
		`,`, `\,`,
		`"`, `\"`,
		`:`, `\:`,
	)
	return r.Replace(s)
}

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
	Name     string // Surname,Given or free-form
	Phone    string
	Email    string
	URL      string
	Address  string
	Org      string
	Note     string
}

func (v VCard) String() string {
	var sb strings.Builder
	sb.WriteString("MECARD:")
	write := func(tag, val string) {
		if val != "" {
			fmt.Fprintf(&sb, "%s:%s;", tag, escape(val))
		}
	}
	write("N", v.Name)
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
	sb.WriteString(e.To)
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
	if encoded := params.Encode(); encoded != "" {
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
	return "sms:" + s.Number + "?body=" + url.QueryEscape(s.Body)
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
	out := fmt.Sprintf("geo:%v,%v", g.Lat, g.Lon)
	if g.Query != "" {
		out += "?q=" + url.QueryEscape(g.Query)
	}
	return out
}

// URL wraps any URL string for clarity at the call site.
type URL struct {
	Href string
}

func (u URL) String() string { return u.Href }
