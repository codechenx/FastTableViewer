package main

import (
	"testing"
)

func TestBuffer_filterByColumn(t *testing.T) {
	tests := []struct {
		name         string
		data         [][]string
		colIndex     int
		options      FilterOptions
		expectedRows int // including header
	}{
		{
			name: "Filter with operator 'contains'",
			data: [][]string{
				{"Name", "Status", "Age"},
				{"Alice", "Active", "30"},
				{"Bob", "Inactive", "25"},
				{"Charlie", "Active", "35"},
			},
			colIndex:     1, // Status column
			options:      FilterOptions{Query: "act", Operator: "contains", CaseSensitive: false},
			expectedRows: 4, // header + 3 rows containing "act"
		},
		{
			name: "Filter with operator 'contains' case sensitive",
			data: [][]string{
				{"Name", "Status", "Age"},
				{"Alice", "Active", "30"},
				{"Bob", "Inactive", "25"},
				{"Charlie", "active", "35"},
			},
			colIndex:     1, // Status column
			options:      FilterOptions{Query: "Active", Operator: "contains", CaseSensitive: true},
			expectedRows: 2, // header + "Active"
		},
		{
			name: "Filter with operator 'equals'",
			data: [][]string{
				{"Name", "Status", "Age"},
				{"Alice", "Active", "30"},
				{"Bob", "Inactive", "25"},
				{"Charlie", "Active", "35"},
			},
			colIndex:     1, // Status column
			options:      FilterOptions{Query: "Active", Operator: "equals", CaseSensitive: false},
			expectedRows: 3, // header + 2 "Active" rows
		},
		{
			name: "Filter with operator 'starts with'",
			data: [][]string{
				{"Name", "Status", "Age"},
				{"Alice", "Active", "30"},
				{"Bob", "Inactive", "25"},
				{"Charlie", "Activation", "35"},
			},
			colIndex:     1, // Status column
			options:      FilterOptions{Query: "Activ", Operator: "starts with", CaseSensitive: false},
			expectedRows: 3, // header + "Active", "Activation"
		},
		{
			name: "Filter with operator 'ends with'",
			data: [][]string{
				{"Name", "Status", "Age"},
				{"Alice", "Active", "30"},
				{"Bob", "Inactive", "25"},
				{"Charlie", "Proactive", "35"},
			},
			colIndex:     1, // Status column
			options:      FilterOptions{Query: "active", Operator: "ends with", CaseSensitive: false},
			expectedRows: 4, // header + "Active", "Inactive", "Proactive"
		},
		{
			name: "Filter with operator 'regex'",
			data: [][]string{
				{"Name", "Status", "Age"},
				{"Alice", "Active", "30"},
				{"Bob", "Inactive", "25"},
				{"Charlie", "Pending", "35"},
			},
			colIndex:     1, // Status column
			options:      FilterOptions{Query: "^(Act|Inact)ive$", Operator: "regex", CaseSensitive: true},
			expectedRows: 3, // header + "Active", "Inactive"
		},
		{
			name: "Filter with numeric operator '>'",
			data: [][]string{
				{"Name", "Age"},
				{"Alice", "30"},
				{"Bob", "25"},
				{"Charlie", "35"},
			},
			colIndex:     1, // Age column
			options:      FilterOptions{Query: "28", Operator: ">"},
			expectedRows: 3, // header + "30", "35"
		},
		{
			name: "Filter with numeric operator '<='",
			data: [][]string{
				{"Name", "Age"},
				{"Alice", "30"},
				{"Bob", "25"},
				{"Charlie", "28"},
			},
			colIndex:     1, // Age column
			options:      FilterOptions{Query: "28", Operator: "<="},
			expectedRows: 3, // header + "25", "28"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create buffer with test data
			b, err := createNewBufferWithData(tt.data, false)
			if err != nil {
				t.Fatalf("Failed to create buffer: %v", err)
			}
			b.rowFreeze = 1 // Set header row
			b.detectAllColumnTypes()

			// Apply filter
			filtered := b.filterByColumn(tt.colIndex, tt.options)

			// Check result
			if filtered.rowLen != tt.expectedRows {
				t.Errorf("Expected %d rows (including header), got %d", tt.expectedRows, filtered.rowLen)
			}

			// Verify header is preserved
			if filtered.rowLen > 0 {
				for col := 0; col < filtered.colLen && col < len(tt.data[0]); col++ {
					if filtered.cont[0][col] != tt.data[0][col] {
						t.Errorf("Header not preserved: expected %s, got %s", tt.data[0][col], filtered.cont[0][col])
					}
				}
			}

			// Verify freeze settings are preserved
			if filtered.rowFreeze != b.rowFreeze {
				t.Errorf("rowFreeze not preserved: expected %d, got %d", b.rowFreeze, filtered.rowFreeze)
			}
			if filtered.colFreeze != b.colFreeze {
				t.Errorf("colFreeze not preserved: expected %d, got %d", b.colFreeze, filtered.colFreeze)
			}
		})
	}
}

func TestBuffer_filterByColumn_EdgeCases(t *testing.T) {
	t.Run("Filter column out of bounds", func(t *testing.T) {
		data := [][]string{
			{"Name", "Status"},
			{"Alice", "Active"},
		}
		b, _ := createNewBufferWithData(data, false)
		b.rowFreeze = 1

		// Filter by non-existent column
		filtered := b.filterByColumn(10, FilterOptions{Query: "test", Operator: "contains"})

		// Should return only header
		if filtered.rowLen != 1 {
			t.Errorf("Expected 1 row (header only), got %d", filtered.rowLen)
		}
	})

	t.Run("Filter empty buffer", func(t *testing.T) {
		b := createNewBuffer()
		b.rowFreeze = 0

		// Filter empty buffer
		filtered := b.filterByColumn(0, FilterOptions{Query: "test", Operator: "contains"})

		// Should return empty buffer
		if filtered.rowLen != 0 {
			t.Errorf("Expected 0 rows, got %d", filtered.rowLen)
		}
	})

	t.Run("Filter buffer with no header", func(t *testing.T) {
		data := [][]string{
			{"Alice", "Active"},
			{"Bob", "Inactive"},
		}
		b, _ := createNewBufferWithData(data, false)
		b.rowFreeze = 0 // No header

		// Filter by column
		filtered := b.filterByColumn(1, FilterOptions{Query: "Active", Operator: "equals", CaseSensitive: true})

		// Should return 1 row
		if filtered.rowLen != 1 {
			t.Errorf("Expected 1 row, got %d", filtered.rowLen)
		}
	})
}
