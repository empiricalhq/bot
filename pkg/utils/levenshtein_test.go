package utils_test

import (
	"testing"

	"whatsbot/pkg/utils"
)

func TestLevenshteinDistance(t *testing.T) {
	// The FSM uses a Levenshtein distance of <= 2 to match keywords
	testCases := []struct {
		name     string
		a, b     string
		expected int
	}{
		{"Identical keywords", "precio", "precio", 0},
		{"Identical with accents", "básico", "básico", 0},
		{"Empty strings", "", "", 0},

		{"Single character substitution", "online", "onlime", 1},
		{"Vowel substitution", "matricula", "matricola", 1},
		{"Consonant substitution", "ayuda", "ayusa", 1},
		{"Accent vs no accent", "básico", "basico", 1},
		{"Accent vs no accent 2", "catálogo", "catalogo", 1},
		{"Accent vs no accent 3", "corazón", "corazon", 1},

		{"Single character insertion", "costo", "costos", 1},
		{"Repeated character", "menu", "mennu", 1},
		{"End of word insertion", "pago", "pagos", 1},

		{"Single character deletion", "horario", "horrio", 1},
		{"Vowel deletion", "principiante", "princpiante", 1},
		{"Consonant deletion", "gracias", "grcias", 1},

		{"Two deletions", "horarios", "horaro", 2},
		{"One deletion, one substitution", "experiencia", "exerincia", 2},
		{"Adjacent character swap (transpose)", "costo", "csoto", 2},

		{"Too many errors", "presencial", "precidensial", 4},
		{"Completely different keyword", "precio", "gratis", 4},
		{"One empty string", "ayuda", "", 5},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := utils.LevenshteinDistance(tc.a, tc.b)
			if got != tc.expected {
				t.Errorf("LevenshteinDistance(%q, %q) = %d; want %d", tc.a, tc.b, got, tc.expected)
			}
		})
	}
}
