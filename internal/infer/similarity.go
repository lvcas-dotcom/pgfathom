package infer

import "strings"

// TrigramSimilarity is the Sørensen-Dice coefficient over sets of padded
// character trigrams, case-insensitive: twice the shared trigrams divided by
// the sum of both trigram counts.
//
// Padding follows pg_trgm's convention — two boundary characters before the
// string, one after — so a short name still yields trigrams instead of
// scoring artificially low for lack of them. Either side empty returns 0
// without extracting anything: there is nothing to compare.
//
// This is the only string-distance metric in the package on purpose: it
// generalizes to the reordering and abbreviation the corpus actually shows
// (idkey_operador vs operadorbasecalculo), which a prefix-weighted metric
// like Jaro-Winkler would not reach any better, and adding a second metric
// without a measured case that needs it is exactly the kind of unmeasured
// surface this project avoids.
func TrigramSimilarity(a, b string) float64 {
	if a == "" || b == "" {
		return 0
	}

	ta := trigramSet(a)
	tb := trigramSet(b)

	shared := 0
	for tri := range ta {
		if tb[tri] {
			shared++
		}
	}

	return 2 * float64(shared) / float64(len(ta)+len(tb))
}

func trigramSet(s string) map[string]bool {
	padded := "  " + strings.ToLower(s) + " "

	set := make(map[string]bool)
	for i := 0; i+3 <= len(padded); i++ {
		set[padded[i:i+3]] = true
	}
	return set
}
