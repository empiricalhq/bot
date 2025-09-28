package nameparser

import (
	"regexp"
	"strings"
	"unicode"
)

const (
	minNameLength = 3
	maxWords      = 4
	vowels        = "aeiouáéíóúàèìòùâêîôûãõäëïöü"
)

var (
	// nonLetterRegex matches any character that is not a Unicode letter or hyphen.
	nonLetterRegex = regexp.MustCompile(`[^\p{L}-]+`)
	// vowelRegex matches vowels, case-insensitive.
	vowelRegex = regexp.MustCompile(`(?i)[` + vowels + `]`)
)

// Parse extracts a plausible first name from a full name string.
// It applies several heuristics to clean and validate the name.
// If a valid name is found, it's returned in Title Case.
// If no suitable name is found, it returns an empty string.
func Parse(fullName string) string {
	if strings.TrimSpace(fullName) == "" {
		return ""
	}

	words := strings.Fields(fullName)

	if len(words) > maxWords {
		return ""
	}

	for _, word := range words {
		cleanedWord := nonLetterRegex.ReplaceAllString(word, "")

		if strings.Contains(cleanedWord, "-") {
			cleanedWord = strings.Split(cleanedWord, "-")[0]
		}

		// we use rune length for Unicode characters.
		if len([]rune(cleanedWord)) < minNameLength {
			continue
		}

		// names shouldn't be just consonants.
		if !vowelRegex.MatchString(cleanedWord) {
			continue
		}

		return capitalize(cleanedWord)
	}

	return ""
}

// capitalize converts a string to Title Case (e.g., "jOhN" -> "John").
// It is Unicode-aware.
func capitalize(s string) string {
	if s == "" {
		return ""
	}

	// convert the whole string to lower case first to handle cases like "JOSÉ".
	lower := strings.ToLower(s)
	runes := []rune(lower)

	// capitalize the first letter.
	runes[0] = unicode.ToTitle(runes[0])

	return string(runes)
}
