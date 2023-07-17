package go_qr

import "testing"

func TestMakeNumeric(t *testing.T) {
	cases := []struct {
		digits string
	}{}
	_ = cases
}

func TestIsAlphanumeric(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"HELLO WORLD", true},       // contains uppercase letters and a space
		{"12345", true},             // contains numbers
		{"$%*+-./:", true},          // contains valid special characters
		{"hello world", false},      // contains lowercase letters
		{"_NotValid", false},        // contains underscore which is not a valid character
		{"123abc", false},           // contains lowercase letters
		{"Special!@#", false},       // contains special characters that are not allowed
		{"Mixed123CASE$", false},    // mixture of digits, uppercase letters and a valid special character
		{"://www.apple.com", false}, // contains double slashes and lower case letters
	}
	for _, c := range cases {
		got := isisAlphanumeric(c.in)
		if got != c.want {
			t.Errorf("isisAlphanumeric(%q) == %v, want %v", c.in, got, c.want)
		}
	}
}

func TestIsNumeric(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"12345", true},        // contains only numbers
		{"ABCDE", false},       // contains no numbers
		{"abc123", true},       // contains numbers and lowercase letters
		{"ABC123", true},       // contains numbers and uppercase letters
		{"Special!@#1", true},  // contains special characters and a number
		{"Special!@#", false},  // contains special characters, but no number
		{" ", false},           // contains only a whitespace character
		{"Mixed123CASE", true}, // mixture of digits, uppercase and lower case letters
		{"1.23", true},         // contains numbers and a dot
	}
	for _, c := range cases {
		got := isNumeric(c.in)
		if got != c.want {
			t.Errorf("isNumeric(%q) == %v, want %v", c.in, got, c.want)
		}
	}
}
