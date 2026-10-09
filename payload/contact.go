package payload

import "strings"

// Contact is a vCard 3.0 contact. It supports more fields than the compact
// MECARD written by VCard, including several phone numbers and emails, at
// the cost of a larger symbol.
//
// Format reference: RFC 2426.
type Contact struct {
	Name       string // formatted name; built from GivenName and FamilyName if empty
	FamilyName string
	GivenName  string
	Org        string
	Title      string
	Phones     []string
	Emails     []string
	URL        string
	Address    string // single line, written as the street of the address
	Note       string
}

// vCardEscaper escapes text values per RFC 2426 §4.
var vCardEscaper = strings.NewReplacer(`\`, `\\`, `;`, `\;`, `,`, `\,`, "\r\n", `\n`, "\n", `\n`)

// vCardURIEscaper makes a URI safe as a vCard uri value, which is not
// escaped like text: it percent-encodes line breaks, which would end the
// content line, and backslashes, which readers may unescape.
var vCardURIEscaper = strings.NewReplacer("\r", "%0D", "\n", "%0A", `\`, "%5C")

func (c Contact) String() string {
	name := c.Name
	if name == "" {
		name = strings.TrimSpace(c.GivenName + " " + c.FamilyName)
	}
	family, given := c.FamilyName, c.GivenName
	if family == "" && given == "" {
		family = name
	}

	var sb strings.Builder
	line := func(key, value string) {
		sb.WriteString(key)
		sb.WriteByte(':')
		sb.WriteString(value)
		sb.WriteString("\r\n")
	}
	text := func(key, value string) {
		if value != "" {
			line(key, vCardEscaper.Replace(value))
		}
	}

	line("BEGIN", "VCARD")
	line("VERSION", "3.0")
	line("N", vCardEscaper.Replace(family)+";"+vCardEscaper.Replace(given)+";;;")
	line("FN", vCardEscaper.Replace(name))
	text("ORG", c.Org)
	text("TITLE", c.Title)
	for _, p := range c.Phones {
		text("TEL", p)
	}
	for _, e := range c.Emails {
		text("EMAIL", e)
	}
	if c.URL != "" {
		// URL is of type uri (RFC 2426 §3.6.8), whose commas and
		// semicolons are part of the address.
		line("URL", vCardURIEscaper.Replace(c.URL))
	}
	if c.Address != "" {
		line("ADR", ";;"+vCardEscaper.Replace(c.Address)+";;;;")
	}
	text("NOTE", c.Note)
	sb.WriteString("END:VCARD")
	return sb.String()
}
