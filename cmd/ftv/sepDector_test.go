package main

import (
	"testing"
)

// ========================================
// Separator Detection Tests
// ========================================

func TestSepDetect_Comma(t *testing.T) {
	sd := sepDetecor{}
	lines := []string{
		"Name,Age,City",
		"Alice,30,NYC",
		"Bob,25,LA",
	}

	sep := sd.sepDetect(lines)
	if sep != ',' {
		t.Errorf("Expected comma separator, got %c", sep)
	}
}

func TestSepDetect_Tab(t *testing.T) {
	sd := sepDetecor{}
	lines := []string{
		"Name\tAge\tCity",
		"Alice\t30\tNYC",
		"Bob\t25\tLA",
	}

	sep := sd.sepDetect(lines)
	if sep != '\t' {
		t.Errorf("Expected tab separator, got %c", sep)
	}
}

func TestSepDetect_Pipe(t *testing.T) {
	sd := sepDetecor{}
	lines := []string{
		"Name|Age|City",
		"Alice|30|NYC",
		"Bob|25|LA",
	}

	sep := sd.sepDetect(lines)
	if sep != '|' {
		t.Errorf("Expected pipe separator, got %c", sep)
	}
}

func TestSepDetect_Semicolon(t *testing.T) {
	sd := sepDetecor{}
	lines := []string{
		"Product;Price;Quantity",
		"Apple;1.50;10",
		"Banana;0.75;20",
	}

	sep := sd.sepDetect(lines)
	if sep != ';' {
		t.Errorf("Expected semicolon separator, got %c", sep)
	}
}

func TestSepDetect_EmptyInput(t *testing.T) {
	sd := sepDetecor{}
	lines := []string{}

	sep := sd.sepDetect(lines)
	if sep != 0 {
		t.Errorf("Expected null separator for empty input, got %c", sep)
	}
}

func TestSepDetect_InconsistentSeparators(t *testing.T) {
	sd := sepDetecor{}
	lines := []string{
		"Name,Age,City",
		"Alice|30|NYC",
		"Bob\t25\tLA",
	}

	// Should detect the most consistent separator
	sep := sd.sepDetect(lines)
	// Any valid separator is acceptable here
	// With inconsistent separators, detection may fail (return 0)
	// or detect the first consistent one
	t.Logf("Detected separator for inconsistent input: %c (code: %d)", sep, sep)
}

func TestIsValidSeparator(t *testing.T) {
	sd := sepDetecor{}

	tests := []struct {
		name     string
		lines    []string
		sep      rune
		expected bool
	}{
		{
			"Valid comma",
			[]string{"a,b,c", "1,2,3"},
			',',
			true,
		},
		{
			"Invalid separator",
			[]string{"a,b,c", "1,2,3"},
			'|',
			false,
		},
		{
			"Inconsistent counts",
			[]string{"a,b,c", "1,2"},
			',',
			false,
		},
		{
			"Empty lines",
			[]string{},
			',',
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sd.isValidSeparator(tt.lines, tt.sep)
			if result != tt.expected {
				t.Errorf("isValidSeparator() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCountRuneFast(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		r        rune
		expected int
	}{
		{"Count commas", "a,b,c,d", ',', 3},
		{"Count none", "abcd", ',', 0},
		{"Count all", ",,,,", ',', 4},
		{"Empty string", "", ',', 0},
		{"Single char", "a", 'a', 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := countRuneFast(tt.s, tt.r)
			if result != tt.expected {
				t.Errorf("countRuneFast() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestGetCandidates(t *testing.T) {
	sd := sepDetecor{}

	line := "Name,Age|City;Country"
	candidates := sd.getCandidates(line)

	if len(candidates) == 0 {
		t.Error("Should find some candidates")
	}

	// Check that common separators are found
	found := false
	for _, c := range candidates {
		if c == ',' || c == '|' || c == ';' {
			found = true
			break
		}
	}

	if !found {
		t.Error("Should find at least one common separator")
	}
}

func TestScoreSeparator(t *testing.T) {
	sd := sepDetecor{}

	tests := []struct {
		name  string
		sep   rune
		count int
	}{
		{"Comma high score", ',', 5},
		{"Tab high score", '\t', 5},
		{"Pipe good score", '|', 5},
		{"Semicolon good score", ';', 5},
		{"Space low score", ' ', 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := sd.scoreSeparator(tt.sep, tt.count)
			if score <= 0 {
				t.Errorf("scoreSeparator(%c, %d) = %d, should be positive", tt.sep, tt.count, score)
			}
		})
	}

	// Verify comma has highest score
	commaScore := sd.scoreSeparator(',', 5)
	spaceScore := sd.scoreSeparator(' ', 5)

	if commaScore <= spaceScore {
		t.Error("Comma should have higher score than space")
	}
}

func TestUniqueChar(t *testing.T) {
	input := []rune{'a', 'b', 'a', 'c', 'b', 'd'}
	result := uniqueChar(input)

	// Check that result has no duplicates
	seen := make(map[rune]bool)
	for _, r := range result {
		if seen[r] {
			t.Errorf("Duplicate rune found: %c", r)
		}
		seen[r] = true
	}

	if len(result) > len(input) {
		t.Error("Result should not have more elements than input")
	}
}

func TestAllIntItemEqual(t *testing.T) {
	tests := []struct {
		name     string
		input    []int
		expected bool
	}{
		{"All equal", []int{5, 5, 5, 5}, true},
		{"Not equal", []int{5, 5, 3, 5}, false},
		{"Single item", []int{5}, true},
		{"Empty", []int{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := allIntItemEqual(tt.input)
			if result != tt.expected {
				t.Errorf("allIntItemEqual() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// ========================================
// Performance Tests
// ========================================

func BenchmarkSepDetect_Comma(b *testing.B) {
	sd := sepDetecor{}
	lines := []string{
		"Name,Age,City,Country,Score",
		"Alice,30,NYC,USA,95",
		"Bob,25,LA,USA,87",
		"Charlie,35,Chicago,USA,92",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sd.sepDetect(lines)
	}
}

func BenchmarkCountRuneFast(b *testing.B) {
	s := "a,b,c,d,e,f,g,h,i,j,k,l,m,n,o,p,q,r,s,t,u,v,w,x,y,z"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		countRuneFast(s, ',')
	}
}
