package main

import (
	"testing"
)

func TestPerformSearch(t *testing.T) {
	// Create test buffer
	testData := [][]string{
		{"Name", "Age", "City"},
		{"John", "25", "New York"},
		{"Jane", "30", "Los Angeles"},
		{"Bob", "35", "Chicago"},
	}

	b, err := createNewBufferWithData(testData, false)
	if err != nil {
		t.Fatalf("Failed to create buffer: %v", err)
	}

	// Test case-insensitive search (non-regex)
	results := performSearch(b, MatchSpec{Query: "john"})
	if len(results) != 1 {
		t.Errorf("Expected 1 result for 'john', got %d", len(results))
	}
	if len(results) > 0 && (results[0].Row != 1 || results[0].Col != 0) {
		t.Errorf("Expected result at (1,0), got (%d,%d)", results[0].Row, results[0].Col)
	}

	// Test case-sensitive search (non-regex)
	results = performSearch(b, MatchSpec{Query: "John", CaseSensitive: true})
	if len(results) != 1 {
		t.Errorf("Expected 1 result for 'John', got %d", len(results))
	}

	// Test case-sensitive search with no results (non-regex)
	results = performSearch(b, MatchSpec{Query: "john", CaseSensitive: true})
	if len(results) != 0 {
		t.Errorf("Expected 0 results for 'john' case-sensitive, got %d", len(results))
	}

	// Test partial match (non-regex)
	results = performSearch(b, MatchSpec{Query: "an"})
	if len(results) != 2 { // "Jane" and "Los Angeles"
		t.Errorf("Expected 2 results for 'an', got %d", len(results))
	}

	// Test no match (non-regex)
	results = performSearch(b, MatchSpec{Query: "xyz"})
	if len(results) != 0 {
		t.Errorf("Expected 0 results for 'xyz', got %d", len(results))
	}
}

func TestPerformSearchRegex(t *testing.T) {
	// Create test buffer
	testData := [][]string{
		{"Name", "Email", "Age"},
		{"John", "john@test.com", "25"},
		{"Jane", "jane@example.org", "30"},
		{"Bob", "bob123@test.com", "35"},
	}

	b, err := createNewBufferWithData(testData, false)
	if err != nil {
		t.Fatalf("Failed to create buffer: %v", err)
	}

	// Test regex pattern for email domains
	results := performSearch(b, MatchSpec{Query: `@test\.com$`, Operator: opRegex})
	if len(results) != 2 { // john@test.com and bob123@test.com
		t.Errorf("Expected 2 results for email regex, got %d", len(results))
	}

	// Test regex pattern for numbers at end
	results = performSearch(b, MatchSpec{Query: `\d+$`, Operator: opRegex})
	if len(results) != 3 { // All age values (25, 30, 35)
		t.Errorf("Expected 3 results for number regex, got %d", len(results))
	}

	// Test regex pattern - case sensitive
	results = performSearch(b, MatchSpec{Query: `^J`, Operator: opRegex, CaseSensitive: true})
	if len(results) != 2 { // John and Jane
		t.Errorf("Expected 2 results for '^J' regex, got %d", len(results))
	}

	// Test regex pattern - case insensitive
	results = performSearch(b, MatchSpec{Query: `^j`, Operator: opRegex})
	if len(results) != 4 { // John, Jane, john@test.com, jane@example.org
		t.Errorf("Expected 4 results for '^j' regex, got %d", len(results))
	}

	// Test invalid regex
	results = performSearch(b, MatchSpec{Query: `[invalid(`, Operator: opRegex})
	if len(results) != 0 {
		t.Errorf("Expected 0 results for invalid regex, got %d", len(results))
	}

	// Test regex OR pattern
	results = performSearch(b, MatchSpec{Query: `John|Bob`, Operator: opRegex})
	if len(results) != 4 {
		t.Errorf("Expected 4 results for 'John|Bob' regex, got %d", len(results))
	}
}
