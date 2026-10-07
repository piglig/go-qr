package server

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/piglig/go-qr/mcp/internal/inspect"
	"github.com/piglig/go-qr/v2"
	"github.com/piglig/go-qr/v2/payload"
)

// GenerateInput describes the code to create. Give exactly one of text or
// one payload field.
type GenerateInput struct {
	Text    string        `json:"text,omitempty" jsonschema:"plain text or a URL to encode"`
	WiFi    *WiFiInput    `json:"wifi,omitempty" jsonschema:"a Wi-Fi network to join"`
	Contact *ContactInput `json:"contact,omitempty" jsonschema:"a contact card (vCard 3.0)"`
	Event   *EventInput   `json:"event,omitempty" jsonschema:"a calendar event"`
	OTP     *OTPInput     `json:"otp,omitempty" jsonschema:"a two-factor authentication account for authenticator apps"`
	Payment *PaymentInput `json:"payment,omitempty" jsonschema:"a SEPA credit transfer (EPC QR / GiroCode)"`
	Email   *EmailInput   `json:"email,omitempty" jsonschema:"an email to compose"`
	SMS     *SMSInput     `json:"sms,omitempty" jsonschema:"a text message to compose"`
	Phone   string        `json:"phone,omitempty" jsonschema:"a phone number to call"`
	Geo     *GeoInput     `json:"geo,omitempty" jsonschema:"a map location"`

	ECC    string `json:"ecc,omitempty" jsonschema:"minimum error correction: L, M (default), Q or H; use H with a logo"`
	Format string `json:"format,omitempty" jsonschema:"png (default) or svg"`
	Style  *Style `json:"style,omitempty" jsonschema:"optional look of the code"`

	OutputPath string `json:"output_path,omitempty" jsonschema:"optional file to save the image to (.png or .svg)"`
	Overwrite  bool   `json:"overwrite,omitempty" jsonschema:"replace output_path if it exists"`
}

// Style controls the look of a generated code.
type Style struct {
	Module      string  `json:"module,omitempty" jsonschema:"module shape: square (default), dot or rounded"`
	Finder      string  `json:"finder,omitempty" jsonschema:"finder pattern shape: square (default), rounded or circle"`
	Foreground  string  `json:"foreground,omitempty" jsonschema:"dark color as #rrggbb (default #000000)"`
	Background  string  `json:"background,omitempty" jsonschema:"light color as #rrggbb, or transparent (default #ffffff)"`
	GradientTo  string  `json:"gradient_to,omitempty" jsonschema:"if set, fade the dark modules from foreground to this #rrggbb color"`
	GradientDeg float64 `json:"gradient_angle,omitempty" jsonschema:"gradient direction in degrees; 0 is left to right, 90 top to bottom"`
	FinderRing  string  `json:"finder_ring,omitempty" jsonschema:"color of the three finder pattern rings as #rrggbb"`
	FinderDot   string  `json:"finder_center,omitempty" jsonschema:"color of the finder pattern centers as #rrggbb"`
	Scale       int     `json:"scale,omitempty" jsonschema:"pixels per module, 1 to 40 (default 10)"`
	QuietZone   *int    `json:"quiet_zone,omitempty" jsonschema:"margin in modules, 0 to 16 (default 4)"`
}

// WiFiInput is a Wi-Fi network.
type WiFiInput struct {
	SSID     string `json:"ssid"`
	Password string `json:"password,omitempty"`
	Security string `json:"security,omitempty" jsonschema:"WPA (default with a password), WEP or nopass"`
	Hidden   bool   `json:"hidden,omitempty"`
}

// ContactInput is a contact card.
type ContactInput struct {
	Name    string   `json:"name"`
	Org     string   `json:"org,omitempty"`
	Title   string   `json:"title,omitempty"`
	Phones  []string `json:"phones,omitempty"`
	Emails  []string `json:"emails,omitempty"`
	URL     string   `json:"url,omitempty"`
	Address string   `json:"address,omitempty"`
	Note    string   `json:"note,omitempty"`
}

// EventInput is a calendar event.
type EventInput struct {
	Summary     string `json:"summary"`
	Start       string `json:"start" jsonschema:"RFC 3339 time such as 2026-10-07T18:00:00+02:00, or a date 2026-10-07 for an all-day event"`
	End         string `json:"end,omitempty" jsonschema:"same formats as start"`
	Location    string `json:"location,omitempty"`
	Description string `json:"description,omitempty"`
}

// OTPInput is a 2FA account.
type OTPInput struct {
	Issuer  string `json:"issuer" jsonschema:"service name shown in the app"`
	Account string `json:"account" jsonschema:"user name or email"`
	Secret  string `json:"secret" jsonschema:"shared secret in base32"`
	Digits  int    `json:"digits,omitempty" jsonschema:"code length, 6 (default) or 8"`
	Period  int    `json:"period,omitempty" jsonschema:"seconds per code (default 30)"`
}

// PaymentInput is a SEPA credit transfer.
type PaymentInput struct {
	Beneficiary string `json:"beneficiary"`
	IBAN        string `json:"iban"`
	BIC         string `json:"bic,omitempty"`
	AmountCents int64  `json:"amount_cents,omitempty" jsonschema:"amount in euro cents, for example 2550 for EUR 25.50; omit to let the payer enter it"`
	Reference   string `json:"reference,omitempty" jsonschema:"remittance information shown to the beneficiary"`
}

// EmailInput is an email to compose.
type EmailInput struct {
	To      string `json:"to"`
	Subject string `json:"subject,omitempty"`
	Body    string `json:"body,omitempty"`
}

// SMSInput is a text message to compose.
type SMSInput struct {
	Number string `json:"number"`
	Body   string `json:"body,omitempty"`
}

// GeoInput is a map location.
type GeoInput struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Query     string  `json:"query,omitempty" jsonschema:"optional label or search"`
}

// GenerateOutput reports what was created.
type GenerateOutput struct {
	Content    string         `json:"content" jsonschema:"the exact text encoded in the code"`
	Version    int            `json:"version"`
	ECC        string         `json:"ecc"`
	Modules    int            `json:"modules" jsonschema:"side of the symbol in modules"`
	Format     string         `json:"format"`
	Verified   bool           `json:"verified" jsonschema:"the rendering was decoded back and its colors have enough contrast"`
	SavedTo    string         `json:"saved_to,omitempty"`
	Inspection inspect.Report `json:"inspection" jsonschema:"what scanning the code does"`
}

func (f fileAccess) generate(_ context.Context, _ *mcp.CallToolRequest, in GenerateInput) (*mcp.CallToolResult, GenerateOutput, error) {
	content, err := in.content()
	if err != nil {
		return nil, GenerateOutput{}, err
	}
	ecc := qr.ECCMedium
	if in.ECC != "" {
		var ok bool
		if ecc, ok = map[string]qr.ECC{"L": qr.ECCLow, "M": qr.ECCMedium, "Q": qr.ECCQuartile, "H": qr.ECCHigh}[strings.ToUpper(in.ECC)]; !ok {
			return nil, GenerateOutput{}, fmt.Errorf("ecc %q: want L, M, Q or H", in.ECC)
		}
	}
	format := strings.ToLower(in.Format)
	if format == "" {
		format = "png"
	}
	if format != "png" && format != "svg" {
		return nil, GenerateOutput{}, fmt.Errorf("format %q: want png or svg", in.Format)
	}
	opts, err := in.Style.options()
	if err != nil {
		return nil, GenerateOutput{}, err
	}

	code, err := qr.Encode(content, qr.WithECC(ecc))
	if err != nil {
		return nil, GenerateOutput{}, err
	}
	if err := code.Verify(opts...); err != nil {
		return nil, GenerateOutput{}, fmt.Errorf("the code would not scan reliably with this style: %w; use darker colors, a plain style, or higher error correction", err)
	}

	var data []byte
	if format == "png" {
		data, err = code.PNG(opts...)
	} else {
		data, err = code.SVG(opts...)
	}
	if err != nil {
		return nil, GenerateOutput{}, err
	}

	out := GenerateOutput{
		Content: content, Version: code.Version(), ECC: code.ECC().String(), Modules: code.Size(),
		Format: format, Verified: true, Inspection: inspect.Text(content, false),
	}
	if in.OutputPath != "" {
		if out.SavedTo, err = f.save(in.OutputPath, format, data, in.Overwrite); err != nil {
			return nil, GenerateOutput{}, err
		}
	}

	summary := fmt.Sprintf("Created a version %d QR Code (%dx%d modules, error correction %s) that encodes %q. It was verified to scan.",
		out.Version, out.Modules, out.Modules, out.ECC, content)
	if out.SavedTo != "" {
		summary += " Saved to " + out.SavedTo + "."
	}
	result := &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: summary}}}
	if format == "png" {
		result.Content = append(result.Content, &mcp.ImageContent{Data: data, MIMEType: "image/png"})
	} else {
		result.Content = append(result.Content, &mcp.TextContent{Text: string(data)})
	}
	return result, out, nil
}

func (f fileAccess) save(p, format string, data []byte, overwrite bool) (string, error) {
	if ext := strings.ToLower(filepath.Ext(p)); ext != "."+format {
		return "", fmt.Errorf("output_path %q must end in .%s", p, format)
	}
	path, err := f.resolve(p)
	if err != nil {
		return "", err
	}
	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if !overwrite {
		flags = os.O_WRONLY | os.O_CREATE | os.O_EXCL
	}
	file, err := os.OpenFile(path, flags, 0o644)
	if errors.Is(err, os.ErrExist) {
		return "", fmt.Errorf("%s already exists; set overwrite to replace it", path)
	}
	if err != nil {
		return "", err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return "", err
	}
	return path, file.Close()
}

// content returns the text to encode.
func (in GenerateInput) content() (string, error) {
	var parts []string
	set := func(name string, ok bool) {
		if ok {
			parts = append(parts, name)
		}
	}
	set("text", in.Text != "")
	set("wifi", in.WiFi != nil)
	set("contact", in.Contact != nil)
	set("event", in.Event != nil)
	set("otp", in.OTP != nil)
	set("payment", in.Payment != nil)
	set("email", in.Email != nil)
	set("sms", in.SMS != nil)
	set("phone", in.Phone != "")
	set("geo", in.Geo != nil)
	if len(parts) != 1 {
		return "", fmt.Errorf("give exactly one of text, wifi, contact, event, otp, payment, email, sms, phone or geo (got %d: %s)", len(parts), strings.Join(parts, ", "))
	}

	switch {
	case in.Text != "":
		return in.Text, nil
	case in.WiFi != nil:
		w := in.WiFi
		if w.SSID == "" {
			return "", errors.New("wifi.ssid is required")
		}
		return payload.WiFi{SSID: w.SSID, Password: w.Password, Auth: payload.WiFiAuth(w.Security), Hidden: w.Hidden}.String(), nil
	case in.Contact != nil:
		c := in.Contact
		return payload.Contact{Name: c.Name, Org: c.Org, Title: c.Title, Phones: c.Phones, Emails: c.Emails, URL: c.URL, Address: c.Address, Note: c.Note}.String(), nil
	case in.Event != nil:
		e := in.Event
		start, allDay, err := parseTime(e.Start)
		if err != nil {
			return "", fmt.Errorf("event.start: %w", err)
		}
		ev := payload.Event{Summary: e.Summary, Location: e.Location, Description: e.Description, Start: start, AllDay: allDay}
		if e.End != "" {
			if ev.End, _, err = parseTime(e.End); err != nil {
				return "", fmt.Errorf("event.end: %w", err)
			}
		}
		return ev.String(), nil
	case in.OTP != nil:
		o := in.OTP
		if o.Secret == "" {
			return "", errors.New("otp.secret is required")
		}
		return payload.OTP{Issuer: o.Issuer, Account: o.Account, Secret: o.Secret, Digits: o.Digits, Period: o.Period}.String(), nil
	case in.Payment != nil:
		p := in.Payment
		epc := payload.EPC{Name: p.Beneficiary, IBAN: p.IBAN, BIC: p.BIC, Amount: p.AmountCents, Text: p.Reference}
		if err := epc.Validate(); err != nil {
			return "", err
		}
		return epc.String(), nil
	case in.Email != nil:
		return payload.Email{To: in.Email.To, Subject: in.Email.Subject, Body: in.Email.Body}.String(), nil
	case in.SMS != nil:
		return payload.SMS{Number: in.SMS.Number, Body: in.SMS.Body}.String(), nil
	case in.Phone != "":
		return payload.Tel{Number: in.Phone}.String(), nil
	default:
		return payload.Geo{Lat: in.Geo.Latitude, Lon: in.Geo.Longitude, Query: in.Geo.Query}.String(), nil
	}
}

func parseTime(s string) (time.Time, bool, error) {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, false, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, true, nil
	}
	return time.Time{}, false, fmt.Errorf("%q is neither RFC 3339 (2026-10-07T18:00:00Z) nor a date (2026-10-07)", s)
}

func (s *Style) options() ([]qr.RenderOption, error) {
	if s == nil {
		return nil, nil
	}
	var opts []qr.RenderOption
	if s.Scale != 0 {
		if s.Scale < 1 || s.Scale > 40 {
			return nil, fmt.Errorf("style.scale %d: want 1 to 40", s.Scale)
		}
		opts = append(opts, qr.WithScale(s.Scale))
	}
	if s.QuietZone != nil {
		if *s.QuietZone < 0 || *s.QuietZone > 16 {
			return nil, fmt.Errorf("style.quiet_zone %d: want 0 to 16", *s.QuietZone)
		}
		opts = append(opts, qr.WithQuietZone(*s.QuietZone))
	}
	if s.Module != "" {
		m, ok := map[string]qr.ModuleShape{"square": qr.ModuleSquare, "dot": qr.ModuleDot, "rounded": qr.ModuleRounded}[strings.ToLower(s.Module)]
		if !ok {
			return nil, fmt.Errorf("style.module %q: want square, dot or rounded", s.Module)
		}
		opts = append(opts, qr.WithModuleShape(m))
	}
	if s.Finder != "" {
		m, ok := map[string]qr.FinderShape{"square": qr.FinderSquare, "rounded": qr.FinderRounded, "circle": qr.FinderCircle}[strings.ToLower(s.Finder)]
		if !ok {
			return nil, fmt.Errorf("style.finder %q: want square, rounded or circle", s.Finder)
		}
		opts = append(opts, qr.WithFinderShape(m))
	}

	fg := color.Color(color.Black)
	if s.Foreground != "" {
		c, err := parseColor(s.Foreground)
		if err != nil {
			return nil, fmt.Errorf("style.foreground: %w", err)
		}
		fg = c
		opts = append(opts, qr.WithForeground(c))
	}
	if s.Background != "" {
		c, err := parseColor(s.Background)
		if err != nil {
			return nil, fmt.Errorf("style.background: %w", err)
		}
		opts = append(opts, qr.WithBackground(c))
	}
	if s.GradientTo != "" {
		to, err := parseColor(s.GradientTo)
		if err != nil {
			return nil, fmt.Errorf("style.gradient_to: %w", err)
		}
		opts = append(opts, qr.WithGradient(fg, to, s.GradientDeg))
	}
	if s.FinderRing != "" || s.FinderDot != "" {
		ring, center := fg, fg
		var err error
		if s.FinderRing != "" {
			if ring, err = parseColor(s.FinderRing); err != nil {
				return nil, fmt.Errorf("style.finder_ring: %w", err)
			}
		}
		if s.FinderDot != "" {
			if center, err = parseColor(s.FinderDot); err != nil {
				return nil, fmt.Errorf("style.finder_center: %w", err)
			}
		}
		opts = append(opts, qr.WithFinderColor(ring, center))
	}
	return opts, nil
}

// parseColor parses #rgb, #rrggbb, #rrggbbaa or "transparent".
func parseColor(s string) (color.Color, error) {
	s = strings.TrimSpace(s)
	if strings.EqualFold(s, "transparent") {
		return color.Transparent, nil
	}
	h := strings.TrimPrefix(s, "#")
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	if len(h) == 6 {
		h += "ff"
	}
	v, err := strconv.ParseUint(h, 16, 32)
	if len(h) != 8 || err != nil {
		return nil, fmt.Errorf("invalid color %q: want #rrggbb", s)
	}
	return color.NRGBA{R: uint8(v >> 24), G: uint8(v >> 16), B: uint8(v >> 8), A: uint8(v)}, nil
}
