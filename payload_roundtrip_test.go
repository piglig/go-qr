package qr

import (
	"reflect"
	"testing"
	"time"

	"github.com/piglig/go-qr/v2/payload"
)

// TestPayloadRoundTrip renders each payload type, decodes the image and
// parses the text back.
func TestPayloadRoundTrip(t *testing.T) {
	for _, p := range []payload.Payload{
		payload.WiFi{SSID: "café;net", Password: "p:w", Auth: payload.WPA},
		payload.Contact{Name: "山田 太郎", GivenName: "太郎", FamilyName: "山田", Phones: []string{"+81 3 1234 5678"}, Emails: []string{"taro@example.jp"}},
		payload.OTP{Type: payload.TOTP, Issuer: "Example", Account: "alice@example.com", Secret: "JBSWY3DPEHPK3PXP", Digits: 8},
		payload.Event{Summary: "Launch", Start: time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC), End: time.Date(2026, 10, 7, 11, 0, 0, 0, time.UTC)},
		payload.EPC{Name: "Red Cross", IBAN: "BE72000000001616", Amount: 2500, Text: "Donation"},
	} {
		code := mustEncode(t, p.String(), WithECC(ECCMedium))
		res, err := Decode(mustImage(t, code, WithScale(4)))
		if err != nil {
			t.Fatalf("%T: %v", p, err)
		}
		got, err := payload.Parse(res.Text)
		if err != nil {
			t.Fatalf("%T: Parse: %v", p, err)
		}
		if !reflect.DeepEqual(got, p) {
			t.Errorf("%T round trip:\n got %#v\nwant %#v", p, got, p)
		}
	}
}
