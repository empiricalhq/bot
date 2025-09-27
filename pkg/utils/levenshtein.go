package utils

// LevenshteinDistance calculates the Levenshtein distance between two strings.
// The distance is the number of single-character edits (insertions, deletions, or substitutions)
// required to change one string into the other.
func LevenshteinDistance(a, b string) int {
	// Convert strings to rune slices to handle multi-byte characters correctly.
	// This is necessary to handle accented characters and other multi-byte characters.
	runesA := []rune(a)
	runesB := []rune(b)

	m := len(runesA)
	n := len(runesB)

	// Create a matrix to store the distances
	d := make([][]int, m+1)
	for i := range d {
		d[i] = make([]int, n+1)
	}

	// Initialize the first row and column of the matrix
	for i := 0; i <= m; i++ {
		d[i][0] = i
	}

	for j := 0; j <= n; j++ {
		d[0][j] = j
	}

	// Fill the rest of the matrix
	for j := 1; j <= n; j++ {
		for i := 1; i <= m; i++ {
			cost := 0
			if runesA[i-1] != runesB[j-1] {
				cost = 1
			}

			d[i][j] = min(d[i-1][j]+1, d[i][j-1]+1, d[i-1][j-1]+cost)
		}
	}

	return d[m][n]
}

// min returns the minimum of a slice of integers.
func min(a ...int) int {
	res := a[0]
	for _, v := range a {
		if v < res {
			res = v
		}
	}

	return res
}
