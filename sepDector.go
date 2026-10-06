package main

import (
	"strings"
	"unicode"
)

type sepDetecor struct {
}

// Fast separator detection algorithm with improved heuristics
// Key improvements:
// 1. Priority-based candidate selection
// 2. Early exit for common separators
// 3. Optimized character counting
// 4. Better validation logic

func (sd *sepDetecor) sepDetect(s []string) rune {
	// A blank line holds none of every candidate, so counting it rejected
	// every separator outright.
	lines := make([]string, 0, len(s))
	for _, line := range s {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) < 1 {
		return 0
	}

	// Fast path: Check common separators first (99% of cases)
	commonSeps := []rune{',', '\t', '|', ';'}
	for _, sep := range commonSeps {
		if sd.isValidSeparator(lines, sep) {
			return sep
		}
	}

	// Fallback: Analyze all potential separators
	return sd.detectBestSeparator(lines)
}

// Fast validation: Check if a separator is valid for all lines
func (sd *sepDetecor) isValidSeparator(lines []string, sep rune) bool {
	if len(lines) == 0 {
		return false
	}

	// Count separator occurrences in first line
	firstCount := countRuneOutsideQuotes(lines[0], sep)
	if firstCount == 0 {
		return false // Separator not found
	}

	// Verify all lines have same count
	for i := 1; i < len(lines); i++ {
		if countRuneOutsideQuotes(lines[i], sep) != firstCount {
			return false
		}
	}

	return true
}

// countRuneOutsideQuotes counts r in s, ignoring anything inside double
// quotes. A separator within a quoted field is data rather than structure;
// counting it made lines disagree on their column count, which rejected the
// separator and left the file unopenable.
func countRuneOutsideQuotes(s string, r rune) int {
	count, inQuotes := 0, false
	for _, c := range s {
		switch {
		case c == '"':
			inQuotes = !inQuotes
		case c == r && !inQuotes:
			count++
		}
	}
	return count
}

// modalCount returns the most common non-zero count of sep across lines, and
// how many lines carry that count. Ties favour the larger count.
func modalCount(lines []string, sep rune) (count, agreeing int) {
	freq := make(map[int]int, len(lines))
	for _, line := range lines {
		if n := countRuneOutsideQuotes(line, sep); n > 0 {
			freq[n]++
		}
	}
	for n, c := range freq {
		if c > agreeing || (c == agreeing && n > count) {
			count, agreeing = n, c
		}
	}
	return count, agreeing
}

// Optimized rune counter - much faster than strings.Count for single runes
func countRuneFast(s string, r rune) int {
	count := 0
	for _, c := range s {
		if c == r {
			count++
		}
	}
	return count
}

// Analyze all potential separators when common ones don't work
func (sd *sepDetecor) detectBestSeparator(lines []string) rune {
	if len(lines) == 0 {
		return 0
	}

	// Consider characters from every sampled line, not just the first: a
	// leading comment or title row would otherwise hide the real separator.
	seen := make(map[rune]bool)
	var candidates []rune
	for _, line := range lines {
		for _, r := range sd.getCandidates(line) {
			if !seen[r] {
				seen[r] = true
				candidates = append(candidates, r)
			}
		}
	}
	if len(candidates) == 0 {
		return 0
	}

	// A separator has to recur: one stray character in a single line is not
	// structure. With only a line or two, one occurrence is all there is.
	minAgreeing := 2
	if len(lines) < 3 {
		minAgreeing = 1
	}

	// Requiring every line to agree exactly meant one ragged row — a column
	// pasted in by hand — rejected the separator and the file would not open.
	// Score on how much of the sample agrees instead.
	best, bestScore := rune(0), 0
	for _, sep := range candidates {
		count, agreeing := modalCount(lines, sep)
		if count == 0 || agreeing < minAgreeing {
			continue
		}

		// Weight the separator's own priority by the share of the sample that
		// agrees on its column count, so a consistent ';' beats a sporadic ','.
		score := sd.scoreSeparator(sep, count) * agreeing / len(lines)
		if score > bestScore {
			best, bestScore = sep, score
		}
	}

	return best
}

// Get candidate separators from first line
func (sd *sepDetecor) getCandidates(line string) []rune {
	// Use map for deduplication
	seen := make(map[rune]bool)
	var candidates []rune

	// Priority characters to check first
	priority := []rune{',', '\t', '|', ';', ':', ' '}
	for _, r := range priority {
		if strings.ContainsRune(line, r) && !seen[r] {
			seen[r] = true
			candidates = append(candidates, r)
		}
	}

	// Check other non-alphanumeric characters
	for _, r := range line {
		if seen[r] || unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		// Skip quotes and other problematic chars
		if r == '"' || r == '\'' || r == '\\' {
			continue
		}
		seen[r] = true
		candidates = append(candidates, r)
	}

	return candidates
}

// Score separator quality (higher is better)
func (sd *sepDetecor) scoreSeparator(sep rune, count int) int {
	score := 0

	// Prefer common separators
	switch sep {
	case ',':
		score += 1000 // Highest priority
	case '\t':
		score += 900
	case '|':
		score += 800
	case ';':
		score += 700
	case ':':
		score += 600
	case ' ':
		score += 100 // Lowest priority (can be ambiguous)
	default:
		score += 500 // Moderate priority for other chars
	}

	// Prefer separators with reasonable column counts (2-100)
	if count >= 2 && count <= 100 {
		score += count * 10
	} else if count > 100 {
		score -= 100 // Penalize too many columns
	}

	return score
}

// remove duplication item in []rune
func uniqueChar(intSlice []rune) []rune {
	keys := make(map[rune]bool)
	var list []rune
	for _, entry := range intSlice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

// check if all item in []int is equal, false for empty array
func allIntItemEqual(r []int) bool {
	if len(r) == 0 {
		return false
	}
	for i := 1; i < len(r); i++ {
		if r[i] != r[0] {
			return false
		}
	}
	return true
}
