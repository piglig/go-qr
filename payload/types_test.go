package payload

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestOTP(t *testing.T) {
	o := OTP{Issuer: "Example Co", Account: "alice@example.com", Secret: "jbsw y3dp ehpk 3pxp=="}
	assertEqual(t, "otpauth://totp/Example%20Co:alice@example.com?issuer=Example%20Co&secret=JBSWY3DPEHPK3PXP", o.String())

	h := OTP{Type: HOTP, Account: "bob", Secret: "ABC", Algorithm: SHA256, Digits: 8, Counter: 7, Period: 60}
	assertEqual(t, "otpauth://hotp/bob?algorithm=SHA256&counter=7&digits=8&secret=ABC", h.String())
}

func TestContact(t *testing.T) {
	c := Contact{
		GivenName: "Jane", FamilyName: "Doe", Org: "ACME; Inc.", Title: "CTO",
		Phones: []string{"+1 555 0100", "+1 555 0101"}, Emails: []string{"jane@acme.test"},
		URL: "https://acme.test", Address: "1 Main St, Springfield", Note: "line1\nline2",
	}
	want := strings.Join([]string{
		"BEGIN:VCARD",
		"VERSION:3.0",
		"N:Doe;Jane;;;",
		"FN:Jane Doe",
		`ORG:ACME\; Inc.`,
		"TITLE:CTO",
		"TEL:+1 555 0100",
		"TEL:+1 555 0101",
		"EMAIL:jane@acme.test",
		"URL:https://acme.test",
		`ADR:;;1 Main St\, Springfield;;;;`,
		`NOTE:line1\nline2`,
		"END:VCARD",
	}, "\r\n")
	assertEqual(t, want, c.String())

	assertContains(t, Contact{Name: "Solo"}.String(), "N:Solo;;;;\r\nFN:Solo\r\n")
}

func TestEvent(t *testing.T) {
	loc := time.FixedZone("UTC+8", 8*3600)
	e := Event{
		Summary:  "Launch, v2",
		Location: "Room 1",
		Start:    time.Date(2026, 10, 7, 18, 0, 0, 0, loc),
		End:      time.Date(2026, 10, 7, 19, 30, 0, 0, loc),
	}
	assertEqual(t, "BEGIN:VEVENT\r\nSUMMARY:Launch\\, v2\r\nDTSTART:20261007T100000Z\r\nDTEND:20261007T113000Z\r\nLOCATION:Room 1\r\nEND:VEVENT", e.String())

	day := Event{Summary: "Holiday", Start: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), AllDay: true}
	assertContains(t, day.String(), "DTSTART;VALUE=DATE:20261001\r\nDTEND;VALUE=DATE:20261002\r\n")
}

func TestEPC(t *testing.T) {
	e := EPC{
		Name: "Red Cross", IBAN: "BE72 0000 0000 1616", BIC: "bpotbeb1",
		Amount: 1050, Text: "Donation",
	}
	assertEqual(t, "BCD\n002\n1\nSCT\nBPOTBEB1\nRed Cross\nBE72000000001616\nEUR10.50\n\n\nDonation", e.String())
	if err := e.Validate(); err != nil {
		t.Fatal(err)
	}

	minimal := EPC{Name: "Shop", IBAN: "DE89370400440532013000"}
	assertEqual(t, "BCD\n002\n1\nSCT\n\nShop\nDE89370400440532013000", minimal.String())

	for name, bad := range map[string]EPC{
		"no name":         {IBAN: "DE89370400440532013000"},
		"short IBAN":      {Name: "x", IBAN: "DE89"},
		"bad BIC":         {Name: "x", IBAN: "DE89370400440532013000", BIC: "ABC"},
		"negative amount": {Name: "x", IBAN: "DE89370400440532013000", Amount: -1},
		"huge amount":     {Name: "x", IBAN: "DE89370400440532013000", Amount: maxEPCAmount + 1},
		"ref and text":    {Name: "x", IBAN: "DE89370400440532013000", Reference: "RF18", Text: "t"},
		"long text":       {Name: "x", IBAN: "DE89370400440532013000", Text: strings.Repeat("t", 141)},
	} {
		if err := bad.Validate(); !errors.Is(err, ErrInvalidEPC) {
			t.Errorf("%s: Validate = %v, want ErrInvalidEPC", name, err)
		}
	}
}

func TestParseRoundTrip(t *testing.T) {
	start := time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)
	for _, p := range []Payload{
		WiFi{SSID: `my;wifi,"net"`, Password: `a:b\c`, Auth: WPA, Hidden: true},
		WiFi{SSID: "guest", Auth: NoPass},
		VCard{Name: "Smith,John", Phone: "+1234", Email: "a@b.c", URL: "https://x", Address: "1; Main", Org: "Org", Note: "n"},
		Contact{Name: "Jane Doe", GivenName: "Jane", FamilyName: "Doe", Org: "ACME; Inc.", Phones: []string{"1", "2"}, Emails: []string{"j@a.t"}, URL: "https://a.t", Address: "1 Main St", Note: "a\nb"},
		Contact{Name: "Solo"},
		Email{To: "a@b.c", Subject: "Hi there", Body: "line1\nline2 & more", CC: []string{"c@d.e", "f@g.h"}, BCC: []string{"x@y.z"}},
		SMS{Number: "+1555", Body: "hello world?"},
		SMS{Number: "+1555"},
		Tel{Number: "+1-555-0100"},
		Geo{Lat: 52.5163, Lon: -13.3777, Query: "Brandenburg Gate"},
		OTP{Issuer: "Example Co", Account: "alice@example.com", Secret: "JBSWY3DPEHPK3PXP", Algorithm: SHA512, Digits: 8, Period: 60, Type: TOTP},
		OTP{Type: HOTP, Account: "bob", Secret: "ABC", Counter: 42},
		Event{Summary: "Launch, v2", Location: "Room; 1", Description: "a\nb", Start: start, End: start.Add(90 * time.Minute)},
		Event{Summary: "Holiday", Start: start.Truncate(24 * time.Hour), End: start.Truncate(24*time.Hour).AddDate(0, 0, 2), AllDay: true},
		EPC{Name: "Red Cross", IBAN: "BE72000000001616", BIC: "BPOTBEB1", Amount: 1050, Purpose: "CHAR", Reference: "RF18539007547034", Info: "Thanks"},
		URL{Href: "https://example.com/a?b=c"},
	} {
		got, err := Parse(p.String())
		if err != nil {
			t.Errorf("Parse(%q): %v", p.String(), err)
			continue
		}
		if !reflect.DeepEqual(got, p) {
			t.Errorf("Parse(%q)\n got %#v\nwant %#v", p.String(), got, p)
		}
	}
}

func TestParseVariants(t *testing.T) {
	tests := []struct {
		in   string
		want Payload
	}{
		{"wifi:S:home;T:WPA;P:pw;;", WiFi{SSID: "home", Auth: WPA, Password: "pw"}},
		{"SMSTO:+1555:hello", SMS{Number: "+1555", Body: "hello"}},
		{"smsto:+1555", SMS{Number: "+1555"}},
		{"geo:1.5,2.5;u=35", Geo{Lat: 1.5, Lon: 2.5}},
		{"geo:1.5,2.5,100", Geo{Lat: 1.5, Lon: 2.5}},
		{"HTTPS://EXAMPLE.COM", URL{Href: "HTTPS://EXAMPLE.COM"}},
		{"otpauth://totp/ACME:%20alice?secret=ABC&issuer=ACME", OTP{Type: TOTP, Issuer: "ACME", Account: "alice", Secret: "ABC"}},
		{"BEGIN:VCARD\nVERSION:3.0\nN:Doe;Jane\nFN:Jane Doe\nTEL;TYPE=CELL:+1\nADR;TYPE=HOME:;;Street;City;;123;Country\nEND:VCARD",
			Contact{Name: "Jane Doe", FamilyName: "Doe", GivenName: "Jane", Phones: []string{"+1"}, Address: "Street, City, 123, Country"}},
		{"BEGIN:VCALENDAR\r\nBEGIN:VEVENT\r\nSUMMARY:Long\r\n  folded\r\nDTSTART:20261007T100000\r\nEND:VEVENT\r\nEND:VCALENDAR",
			Event{Summary: "Long folded", Start: time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)}},
		{"BCD\r\n001\r\n1\r\nSCT\r\nBIC12345\r\nName\r\nIBAN12345678901\r\nEUR5.5", EPC{BIC: "BIC12345", Name: "Name", IBAN: "IBAN12345678901", Amount: 550}},
	}
	for _, tt := range tests {
		got, err := Parse(tt.in)
		if err != nil {
			t.Errorf("Parse(%q): %v", tt.in, err)
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Parse(%q)\n got %#v\nwant %#v", tt.in, got, tt.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	for in, want := range map[string]error{
		"":                                      ErrUnrecognized,
		"hello world":                           ErrUnrecognized,
		"ftp://example.com":                     ErrUnrecognized,
		"WIFI:T:WPA;;":                          ErrMalformed,
		"geo:abc":                               ErrMalformed,
		"geo:1,x":                               ErrMalformed,
		"otpauth://totp/x":                      ErrMalformed,
		"otpauth://push/x?secret=A":             ErrMalformed,
		"otpauth://totp/x?secret=A&digits=x":    ErrMalformed,
		"BEGIN:VEVENT\nSUMMARY:x\nEND:VEVENT":   ErrMalformed,
		"BEGIN:VEVENT\nDTSTART:bad\nEND:VEVENT": ErrMalformed,
		"BCD\n003\n1\nSCT":                      ErrMalformed,
		"BCD\n002\n1\nINST":                     ErrMalformed,
		"BCD\n002\n1\nSCT\n\nN\nI\nUSD1":        ErrMalformed,
	} {
		if _, err := Parse(in); !errors.Is(err, want) {
			t.Errorf("Parse(%q) error = %v, want %v", in, err, want)
		}
	}
}

func FuzzParse(f *testing.F) {
	for _, s := range []string{"WIFI:S:a;;", "MECARD:N:x;;", "BEGIN:VCARD\nFN:x\nEND:VCARD", "BCD\n002\n1\nSCT\n\nn\ni\nEUR1.2",
		"otpauth://totp/a:b?secret=X", "geo:1,2?q=x", "mailto:a@b?cc=c", "BEGIN:VEVENT\nDTSTART:20260101\nEND:VEVENT"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		p, err := Parse(s)
		if err == nil && p == nil {
			t.Fatal("nil payload without error")
		}
	})
}
