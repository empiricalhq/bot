package utils

// LevenshteinDistance calculates the Levenshtein distance between two strings.
// The distance is the number of single-character edits (insertions, deletions, or substitutions)
// required to change one string into the other.
func LevenshteinDistance(a, b string) int {
	// Convert strings to rune slices to handle multi-byte characters correctly.
	// This is necessary to handle accented characters and other multi-byte characters.
	runesA := []rune(a)
	runesB := []rune(b)

	lenA := len(runesA)
	lenB := len(runesB)

	// Create a matrix to store the distances
	distances := make([][]int, lenA+1)
	for row := range distances {
		distances[row] = make([]int, lenB+1)
	}

	// Initialize the first row and column of the matrix
	for row := 0; row <= lenA; row++ {
		distances[row][0] = row
	}

	for col := 0; col <= lenB; col++ {
		distances[0][col] = col
	}

	// Fill the rest of the matrix
	for col := 1; col <= lenB; col++ {
		for row := 1; row <= lenA; row++ {
			cost := 0
			if runesA[row-1] != runesB[col-1] {
				cost = 1
			}

			distances[row][col] = min(
				distances[row-1][col]+1,      // deletion
				distances[row][col-1]+1,      // insertion
				distances[row-1][col-1]+cost, // substitution
			)
		}
	}

	return distances[lenA][lenB]
}

// min returns the minimum of a slice of integers.
func min(values ...int) int {
	result := values[0]
	for _, v := range values {
		if v < result {
			result = v
		}
	}

	return result
}
