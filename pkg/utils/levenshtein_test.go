package utils

import "testing"

func TestLevenshteinDistance(t *testing.T) {
	// The FSM uses a Levenshtein distance of <= 2 to match keywords
	testCases := []struct {
		name     string
		a, b     string
		expected int
	}{
		// distance = 0
		{"Identical keywords", "precio", "precio", 0},
		{"Identical with accents", "básico", "básico", 0},
		{"Empty strings", "", "", 0},

		// distance = 1 (substitution)
		{"Single character substitution", "online", "onlime", 1},
		{"Vowel substitution", "matricula", "matricola", 1},
		{"Consonant substitution", "ayuda", "ayusa", 1},
		{"Accent vs no accent", "básico", "basico", 1},
		{"Accent vs no accent 2", "catálogo", "catalogo", 1},
		{"Accent vs no accent 3", "corazón", "corazon", 1},

		// distance = 1 (insertion)
		{"Single character insertion", "costo", "costos", 1},
		{"Repeated character", "menu", "mennu", 1},
		{"End of word insertion", "pago", "pagos", 1},

		// distance = 1 (deletion)
		{"Single character deletion", "horario", "horrio", 1},
		{"Vowel deletion", "principiante", "princpiante", 1},
		{"Consonant deletion", "gracias", "grcias", 1},

		// distance = 2
		{"Two deletions", "horarios", "horaro", 2},
		{"One deletion, one substitution", "experiencia", "exerincia", 2}, // p deleted, e -> i
		{"Adjacent character swap (transpose)", "costo", "csoto", 2},      // Levenshtein counts a transpose as two operations

		// these should not match in FSM as the distance is greater than 2
		{"Too many errors", "presencial", "precidensial", 4},
		{"Completely different keyword", "precio", "gratis", 4},
		{"One empty string", "ayuda", "", 5},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := LevenshteinDistance(tc.a, tc.b)
			if got != tc.expected {
				t.Errorf("LevenshteinDistance(%q, %q) = %d; want %d", tc.a, tc.b, got, tc.expected)
			}
		})
	}
}
