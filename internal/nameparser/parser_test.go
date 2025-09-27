package nameparser

import "testing"

func TestParse(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		// Basic cases
		{"Simple Name", "John", "John"},
		{"Lowercase", "john", "John"},
		{"Mixed Case", "jOhN", "John"},
		{"Two words", "John Doe", "John"},
		{"Four words", "John Michael F. Doe", "John"},
		{"Five words", "John Michael Fitzgerald Doe Smith", ""},

		// Handpicked examples
		{"Cambridge Uni", "Maybe 🇬🇧CAMBRIDGE UNIVERSITY🇬🇧", "Maybe"},
		{"El Rafa", "🔥El Rafa🔥", "Rafa"},
		{"AG Cotrina", "AG’ Cotrina", "Cotrina"},
		{"Ariana with emojis", "Ariana 🌸🥑🪼", "Ariana"},
		{"Name with numbers", "christiangallece2019", "Christiangallece"},
		{"De la cruz", "De la cruz", "Cruz"},
		{"Name with numbers and no vowels", "Fr4nck01zz", ""},
		{"Short name with emoji", "Ro😊", ""},
		{"Sandra Carvo", "sandracarvo", "Sandracarvo"},
		{"Just semicolon", ";", ""},
		{"Just dot", ".", ""},
		{"Leading dot", ".lenin Vargas", "Lenin"},
		{"Unicode symbols George", "꧁༺George༻꧂", "George"},
		{"Unicode symbols Nataly", "꧁Nataly꧂", "Nataly"},
		{"Quoted Noe", `"Noé"`, ""},
		{"Brackets and emoji", "<Victor >✌️", "Victor"},
		{"Tilde", "~JaiR", "Jair"},
		{"Rita and symbols", "𝄞 rita ♡", "Rita"},
		{"A and symbols", " A~~~🧿", ""},
		{"Ali short", "Ali★", ""},
		{"Aliii long", "Aliii🌟", "Aliii"},
		{"DMA", " ~DMA~", ""},

		{"Accented name", "José", "José"},
		{"Short accented name", "Ana", ""},
		{"Name with hyphen", "Jean-Claude", "Jean"},
		{"Empty string", "", ""},
		{"Whitespace", "   ", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := Parse(tc.input)
			if got != tc.expected {
				t.Errorf("Parse(%q) = %q; want %q", tc.input, got, tc.expected)
			}
		})
	}
}
