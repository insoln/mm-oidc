package main

import "testing"

func TestSanitizeUsername(t *testing.T) {
	cases := map[string]string{
		"Simple":             "john",
		"MixedCase":          "john-doe",
		"Spaces and Symbols": "johndoe",
		"Empty":              "",
		"Unicode":            "usr",
	}

	inputs := map[string]string{
		"Simple":             "John",
		"MixedCase":          "John Doe",
		"Spaces and Symbols": " John@Doe! ",
		"Empty":              "   ",
		"Unicode":            "Usér",
	}

	for name, input := range inputs {
		got := sanitizeUsername(input)
		if got != cases[name] {
			t.Fatalf("%s: expected %s, got %s", name, cases[name], got)
		}
	}
}
