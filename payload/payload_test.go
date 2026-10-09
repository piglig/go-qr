package payload

import (
	"strings"
	"testing"
)

func TestWiFi(t *testing.T) {
	t.Run("WPA with password", func(t *testing.T) {
		w := WiFi{SSID: "home", Password: "s3cret", Auth: WPA}
		assertEqual(t, "WIFI:T:WPA;S:home;P:s3cret;;", w.String())
	})

	t.Run("nopass omits password field", func(t *testing.T) {
		w := WiFi{SSID: "guest", Auth: NoPass}
		assertEqual(t, "WIFI:T:nopass;S:guest;;", w.String())
	})

	t.Run("hidden flag", func(t *testing.T) {
		w := WiFi{SSID: "stealth", Password: "x", Auth: WPA, Hidden: true}
		assertEqual(t, "WIFI:T:WPA;S:stealth;P:x;H:true;;", w.String())
	})

	t.Run("auto-detects auth from password", func(t *testing.T) {
		assertEqual(t, "WIFI:T:WPA;S:x;P:p;;", WiFi{SSID: "x", Password: "p"}.String())
		assertEqual(t, "WIFI:T:nopass;S:x;;", WiFi{SSID: "x"}.String())
	})

	t.Run("escapes reserved chars", func(t *testing.T) {
		w := WiFi{SSID: `my;wifi,"net"`, Password: `a:b\c`, Auth: WPA}
		assertEqual(t, `WIFI:T:WPA;S:my\;wifi\,\"net\";P:a\:b\\c;;`, w.String())
	})
}

func TestVCard(t *testing.T) {
	v := VCard{Name: "Smith,John", Phone: "+1234", Email: "a@b.c", URL: "https://x"}
	s := v.String()
	assertTrue(t, strings.HasPrefix(s, "MECARD:"))
	assertTrue(t, strings.HasSuffix(s, ";;"))
	assertContains(t, s, "N:Smith\\,John;")
	assertContains(t, s, "TEL:+1234;")
	assertContains(t, s, "EMAIL:a@b.c;")
	assertContains(t, s, `URL:https\://x;`)
}

func TestVCard_OmitsEmpty(t *testing.T) {
	v := VCard{Name: "Only"}
	assertEqual(t, "MECARD:N:Only;;", v.String())
}

func TestEmail(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		assertEqual(t, "mailto:a@b.c", Email{To: "a@b.c"}.String())
	})
	t.Run("with subject and body", func(t *testing.T) {
		s := Email{To: "a@b.c", Subject: "hi", Body: "hello world"}.String()
		assertTrue(t, strings.HasPrefix(s, "mailto:a@b.c?"))
		assertContains(t, s, "subject=hi")
		assertContains(t, s, "body=hello%20world")
	})
	t.Run("spaces and plus signs", func(t *testing.T) {
		s := Email{To: "a@b.c", Subject: "1 + 1"}.String()
		assertEqual(t, "mailto:a@b.c?subject=1%20%2B%201", s)
	})
	t.Run("cc/bcc", func(t *testing.T) {
		s := Email{To: "a@b.c", CC: []string{"c1@x", "c2@x"}, BCC: []string{"d@x"}}.String()
		assertContains(t, s, "cc=c1%40x%2Cc2%40x")
		assertContains(t, s, "bcc=d%40x")
	})
}

func TestSMS(t *testing.T) {
	assertEqual(t, "sms:+1234", SMS{Number: "+1234"}.String())
	assertEqual(t, "sms:+1234?body=hi%20there", SMS{Number: "+1234", Body: "hi there"}.String())
	assertEqual(t, "sms:+1234?body=1%2B1", SMS{Number: "+1234", Body: "1+1"}.String())
}

func TestTel(t *testing.T) {
	assertEqual(t, "tel:+1-555-0100", Tel{Number: "+1-555-0100"}.String())
}

func TestGeo(t *testing.T) {
	assertEqual(t, "geo:37.5,-122.3", Geo{Lat: 37.5, Lon: -122.3}.String())
	assertEqual(t, "geo:0.00001,-0.0000001", Geo{Lat: 0.00001, Lon: -0.0000001}.String())
	s := Geo{Lat: 0, Lon: 0, Query: "Null Island"}.String()
	assertContains(t, s, "geo:0,0?q=Null%20Island")
}

func TestURL(t *testing.T) {
	assertEqual(t, "https://example.com", URL{Href: "https://example.com"}.String())
}
