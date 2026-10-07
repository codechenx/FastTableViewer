package main

import (
	"testing"
)

func TestIsNumericValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"Integer", "123", true},
		{"Negative integer", "-456", true},
		{"Float", "123.456", true},
		{"Negative float", "-123.456", true},
		{"Scientific notation", "1.23e10", true},
		{"Scientific negative", "1.23e-10", true},
		{"With comma separator", "1,234.56", true},
		{"With underscore", "1_234_567", true},
		{"Empty string", "", false},
		{"Text", "abc", false},
		{"Mixed text and numbers", "123abc", false},
		{"Just dot", ".", false},
		{"Just sign", "-", false},
		{"Multiple dots", "1.2.3", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNumericValue(tt.input)
			if result != tt.expected {
				t.Errorf("isNumericValue(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestIsDateValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"ISO date", "2024-10-17", true},
		{"ISO datetime", "2024-10-17 15:30:00", true},
		{"US date", "10/17/2024", true},
		{"EU date", "17/10/2024", true},
		{"Alt ISO", "2024/10/17", true},
		{"RFC3339", "2024-10-17T15:30:00Z", true},
		{"Month name", "Jan 02, 2024", true},
		{"Full month", "January 02, 2024", true},
		{"Dotted", "2024.10.17", true},
		{"Empty string", "", false},
		{"Just numbers", "20241017", false},
		{"Text", "not a date", false},
		{"Number", "12345", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isDateValue(tt.input)
			if result != tt.expected {
				t.Errorf("isDateValue(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestAutoDetectColumnType(t *testing.T) {
	tests := []struct {
		name         string
		data         [][]string
		colIndex     int
		expectedType ColumnType
	}{
		{
			name: "String column",
			data: [][]string{
				{"Name", "Age"},
				{"Alice", "30"},
				{"Bob", "25"},
				{"Charlie", "35"},
			},
			colIndex:     0,
			expectedType: colTypeStr,
		},
		{
			name: "Number column",
			data: [][]string{
				{"Name", "Age"},
				{"Alice", "30"},
				{"Bob", "25"},
				{"Charlie", "35"},
			},
			colIndex:     1,
			expectedType: colTypeFloat,
		},
		{
			name: "Date column",
			data: [][]string{
				{"Name", "Date"},
				{"Event1", "2024-01-15"},
				{"Event2", "2024-02-20"},
				{"Event3", "2024-03-10"},
			},
			colIndex:     1,
			expectedType: colTypeDate,
		},
		{
			name: "Mixed column defaults to string",
			data: [][]string{
				{"Name", "Value"},
				{"Item1", "100"},
				{"Item2", "abc"},
				{"Item3", "200"},
			},
			colIndex:     1,
			expectedType: colTypeStr,
		},
		{
			name: "Numeric with NA values",
			data: [][]string{
				{"Name", "Score"},
				{"Test1", "95.5"},
				{"Test2", "NA"},
				{"Test3", "87.3"},
				{"Test4", "92.1"},
			},
			colIndex:     1,
			expectedType: colTypeFloat,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := createNewBufferWithData(tt.data, false)
			if err != nil {
				t.Fatalf("Failed to create buffer: %v", err)
			}
			b.rowFreeze = 1 // Set header row

			detectedType := b.autoDetectColumnType(tt.colIndex)
			if detectedType != tt.expectedType {
				t.Errorf("Expected type %s (%d), got %s (%d)",
					type2name(tt.expectedType), tt.expectedType,
					type2name(detectedType), detectedType)
			}
		})
	}
}

func TestParseNumericValueFast(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected float64
	}{
		{"Integer", "123", 123.0},
		{"Negative", "-456", -456.0},
		{"Float", "123.456", 123.456},
		{"With commas", "1,234.56", 1234.56},
		{"With underscores", "1_234_567", 1234567.0},
		{"Scientific", "1.23e2", 123.0},
		{"Empty", "", 0.0},
		{"NA", "NA", 0.0},
		{"NaN", "NaN", 0.0},
		{"Invalid", "abc", 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseNumericValueFast(tt.input)
			if result != tt.expected {
				t.Errorf("parseNumericValueFast(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestParseDateValueFast(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		shouldBe string // Description of expected behavior
	}{
		{"ISO date", "2024-10-17", "non-zero"},
		{"ISO datetime", "2024-10-17 15:30:00", "non-zero"},
		{"US date", "10/17/2024", "non-zero"},
		{"Empty", "", "zero"},
		{"NA", "NA", "zero"},
		{"Invalid", "not a date", "zero"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDateValueFast(tt.input)
			if tt.shouldBe == "non-zero" && result == 0 {
				t.Errorf("parseDateValueFast(%q) = 0, expected non-zero timestamp", tt.input)
			} else if tt.shouldBe == "zero" && result != 0 {
				t.Errorf("parseDateValueFast(%q) = %d, expected 0", tt.input, result)
			}
		})
	}
}

func TestSortByNum_Performance(t *testing.T) {
	// Create a large dataset to test performance
	data := [][]string{{"Index", "Value"}}
	for i := 10000; i > 0; i-- {
		data = append(data, []string{I2S(i), I2S(i)})
	}

	b, err := createNewBufferWithData(data, false)
	if err != nil {
		t.Fatalf("Failed to create buffer: %v", err)
	}
	b.rowFreeze = 1
	b.setColType(1, colTypeFloat)

	// Sort ascending
	b.SortBy(1, false)

	// Check first and last values
	firstVal := parseNumericValueFast(b.cont[1][1])
	lastVal := parseNumericValueFast(b.cont[len(b.cont)-1][1])

	if firstVal >= lastVal {
		t.Errorf("Sort failed: first value (%v) should be less than last value (%v)", firstVal, lastVal)
	}
}

func TestSortByDate(t *testing.T) {
	data := [][]string{
		{"Event", "Date"},
		{"Last", "2024-12-31"},
		{"First", "2024-01-01"},
		{"Middle", "2024-06-15"},
	}

	b, err := createNewBufferWithData(data, false)
	if err != nil {
		t.Fatalf("Failed to create buffer: %v", err)
	}
	b.rowFreeze = 1
	b.setColType(1, colTypeDate)

	// Debug: check parsed dates
	for i := 1; i < len(b.cont); i++ {
		ts := parseDateValueFast(b.cont[i][1])
		t.Logf("Row %d: %s = %s (unix: %d)", i, b.cont[i][0], b.cont[i][1], ts)
	}

	// Sort ascending by date
	b.SortBy(1, false)

	// Debug: check after sort
	t.Log("After sort:")
	for i := 1; i < len(b.cont); i++ {
		ts := parseDateValueFast(b.cont[i][1])
		t.Logf("Row %d: %s = %s (unix: %d)", i, b.cont[i][0], b.cont[i][1], ts)
	}

	// Check order
	if b.cont[1][0] != "First" {
		t.Errorf("First event should be 'First', got '%s'", b.cont[1][0])
	}
	if b.cont[2][0] != "Middle" {
		t.Errorf("Second event should be 'Middle', got '%s'", b.cont[2][0])
	}
	if b.cont[3][0] != "Last" {
		t.Errorf("Third event should be 'Last', got '%s'", b.cont[3][0])
	}
}

func TestDetectAllColumnTypes(t *testing.T) {
	data := [][]string{
		{"Name", "Age", "Date", "Mixed"},
		{"Alice", "30", "2024-01-15", "Value1"},
		{"Bob", "25", "2024-02-20", "123"},
		{"Charlie", "35", "2024-03-10", "ABC"},
	}

	b, err := createNewBufferWithData(data, false)
	if err != nil {
		t.Fatalf("Failed to create buffer: %v", err)
	}
	b.rowFreeze = 1

	// Detect all column types
	b.detectAllColumnTypes()

	// Check detected types
	if b.getColType(0) != colTypeStr {
		t.Errorf("Column 0 (Name) should be String, got %s", type2name(b.getColType(0)))
	}
	if b.getColType(1) != colTypeFloat {
		t.Errorf("Column 1 (Age) should be Number, got %s", type2name(b.getColType(1)))
	}
	if b.getColType(2) != colTypeDate {
		t.Errorf("Column 2 (Date) should be Date, got %s", type2name(b.getColType(2)))
	}
	if b.getColType(3) != colTypeStr {
		t.Errorf("Column 3 (Mixed) should be String, got %s", type2name(b.getColType(3)))
	}
}
