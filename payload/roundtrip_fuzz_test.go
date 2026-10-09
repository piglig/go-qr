package payload

import (
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

// FuzzStringParse builds payloads of every free-text field from fuzzed
// strings and expects Parse to return each payload String writes, up to
// the normalizations the formats define.
func FuzzStringParse(f *testing.F) {
	f.Add("a@b.c", "Hi there", "1 + 1 = 2%")
	f.Add("Example:Co", "alice:x", `a;b,c\d"e`)
	f.Fuzz(func(t *testing.T, a, b, c string) {
		if !utf8.ValidString(a + b + c) {
			t.Skip()
		}
		// Spaces after the label's colon are optional in the Key URI
		// Format, so readers drop them.
		otp := OTP{Type: TOTP, Issuer: a, Account: b, Secret: "ABC"}
		if a != "" {
			otp.Account = strings.TrimLeft(b, " ")
		} else if strings.Contains(b, ":") {
			otp = OTP{} // documented as unsupported
		}
		// vCard text has a single escape for a line break, read as LF. A
		// URI holds no line breaks or backslashes; Contact percent-encodes
		// them.
		uri := strings.Map(func(r rune) rune {
			if strings.ContainsRune("\r\n\\", r) {
				return -1
			}
			return r
		}, a)
		nl := func(s string) string { return strings.ReplaceAll(s, "\r\n", "\n") }

		for _, tt := range []struct{ in, want Payload }{
			{Email{To: a, Subject: b, Body: c}, nil},
			{SMS{Number: "+1555", Body: a}, nil},
			{Geo{Lat: 1, Lon: 2, Query: a}, nil},
			{VCard{Name: a, Phone: b, Email: c, URL: a, Address: b, Org: c, Note: a}, nil},
			{OTP{Type: TOTP, Issuer: a, Account: b, Secret: "ABC"}, otp},
			{Contact{Name: a, Org: b, Title: c, URL: uri, Note: b}, Contact{Name: nl(a), Org: nl(b), Title: nl(c), URL: uri, Note: nl(b)}},
			{WiFi{SSID: a, Password: b, Auth: WPA}, nil},
		} {
			if w, ok := tt.in.(WiFi); ok && w.SSID == "" {
				continue // a network needs a name
			}
			if tt.want == (OTP{}) {
				continue
			}
			want := tt.want
			if want == nil {
				want = tt.in
			}
			got, err := Parse(tt.in.String())
			if err != nil {
				t.Fatalf("Parse(%q): %v", tt.in.String(), err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("Parse(%q)\n got %#v\nwant %#v", tt.in.String(), got, want)
			}
		}
	})
}
