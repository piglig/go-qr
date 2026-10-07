package payload_test

import (
	"fmt"
	"time"

	"github.com/piglig/go-qr/v2/payload"
)

func ExampleOTP() {
	otp := payload.OTP{Issuer: "Example", Account: "alice@example.com", Secret: "JBSWY3DPEHPK3PXP"}
	fmt.Println(otp)
	// Output: otpauth://totp/Example:alice@example.com?issuer=Example&secret=JBSWY3DPEHPK3PXP
}

func ExampleEvent() {
	e := payload.Event{
		Summary: "Release party",
		Start:   time.Date(2026, 10, 7, 18, 0, 0, 0, time.UTC),
		End:     time.Date(2026, 10, 7, 21, 0, 0, 0, time.UTC),
	}
	fmt.Printf("%q\n", e.String())
	// Output: "BEGIN:VEVENT\r\nSUMMARY:Release party\r\nDTSTART:20261007T180000Z\r\nDTEND:20261007T210000Z\r\nEND:VEVENT"
}

func ExampleEPC() {
	pay := payload.EPC{Name: "Red Cross", IBAN: "BE72 0000 0000 1616", Amount: 2500, Text: "Donation"}
	if err := pay.Validate(); err != nil {
		panic(err)
	}
	fmt.Printf("%q\n", pay.String())
	// Output: "BCD\n002\n1\nSCT\n\nRed Cross\nBE72000000001616\nEUR25.00\n\n\nDonation"
}

func ExampleParse() {
	p, err := payload.Parse("WIFI:T:WPA;S:home;P:s3cret;;")
	if err != nil {
		panic(err)
	}
	switch v := p.(type) {
	case payload.WiFi:
		fmt.Println("join", v.SSID, "with", v.Auth)
	case payload.URL:
		fmt.Println("open", v.Href)
	}
	// Output: join home with WPA
}
