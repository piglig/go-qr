package payload

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWiFi(t *testing.T) {
	t.Run("WPA with password", func(t *testing.T) {
		w := WiFi{SSID: "home", Password: "s3cret", Auth: WPA}
		assert.Equal(t, "WIFI:T:WPA;S:home;P:s3cret;;", w.String())
	})

	t.Run("nopass omits password field", func(t *testing.T) {
		w := WiFi{SSID: "guest", Auth: NoPass}
		assert.Equal(t, "WIFI:T:nopass;S:guest;;", w.String())
	})

	t.Run("hidden flag", func(t *testing.T) {
		w := WiFi{SSID: "stealth", Password: "x", Auth: WPA, Hidden: true}
		assert.Equal(t, "WIFI:T:WPA;S:stealth;P:x;H:true;;", w.String())
	})

	t.Run("auto-detects auth from password", func(t *testing.T) {
		assert.Equal(t, "WIFI:T:WPA;S:x;P:p;;", WiFi{SSID: "x", Password: "p"}.String())
		assert.Equal(t, "WIFI:T:nopass;S:x;;", WiFi{SSID: "x"}.String())
	})

	t.Run("escapes reserved chars", func(t *testing.T) {
		w := WiFi{SSID: `my;wifi,"net"`, Password: `a:b\c`, Auth: WPA}
		assert.Equal(t, `WIFI:T:WPA;S:my\;wifi\,\"net\";P:a\:b\\c;;`, w.String())
	})
}

func TestVCard(t *testing.T) {
	v := VCard{Name: "Smith,John", Phone: "+1234", Email: "a@b.c", URL: "https://x"}
	s := v.String()
	assert.True(t, strings.HasPrefix(s, "MECARD:"))
	assert.True(t, strings.HasSuffix(s, ";;"))
	assert.Contains(t, s, "N:Smith\\,John;")
	assert.Contains(t, s, "TEL:+1234;")
	assert.Contains(t, s, "EMAIL:a@b.c;")
	assert.Contains(t, s, `URL:https\://x;`)
}

func TestVCard_OmitsEmpty(t *testing.T) {
	v := VCard{Name: "Only"}
	assert.Equal(t, "MECARD:N:Only;;", v.String())
}

func TestEmail(t *testing.T) {
	t.Run("plain", func(t *testing.T) {
		assert.Equal(t, "mailto:a@b.c", Email{To: "a@b.c"}.String())
	})
	t.Run("with subject and body", func(t *testing.T) {
		s := Email{To: "a@b.c", Subject: "hi", Body: "hello world"}.String()
		assert.True(t, strings.HasPrefix(s, "mailto:a@b.c?"))
		assert.Contains(t, s, "subject=hi")
		assert.Contains(t, s, "body=hello+world")
	})
	t.Run("cc/bcc", func(t *testing.T) {
		s := Email{To: "a@b.c", CC: []string{"c1@x", "c2@x"}, BCC: []string{"d@x"}}.String()
		assert.Contains(t, s, "cc=c1%40x%2Cc2%40x")
		assert.Contains(t, s, "bcc=d%40x")
	})
}

func TestSMS(t *testing.T) {
	assert.Equal(t, "sms:+1234", SMS{Number: "+1234"}.String())
	assert.Equal(t, "sms:+1234?body=hi+there", SMS{Number: "+1234", Body: "hi there"}.String())
}

func TestTel(t *testing.T) {
	assert.Equal(t, "tel:+1-555-0100", Tel{Number: "+1-555-0100"}.String())
}

func TestGeo(t *testing.T) {
	assert.Equal(t, "geo:37.5,-122.3", Geo{Lat: 37.5, Lon: -122.3}.String())
	s := Geo{Lat: 0, Lon: 0, Query: "Null Island"}.String()
	assert.Contains(t, s, "geo:0,0?q=Null+Island")
}

func TestURL(t *testing.T) {
	assert.Equal(t, "https://example.com", URL{Href: "https://example.com"}.String())
}
