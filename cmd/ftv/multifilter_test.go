package main

import (
	"testing"
)

func TestMultiColumnFilter(t *testing.T) {
	// Create test data
	data := [][]string{
		{"Name", "Age", "City", "Department"},
		{"Alice", "30", "New York", "Engineering"},
		{"Bob", "25", "San Francisco", "Sales"},
		{"Charlie", "35", "New York", "Engineering"},
		{"David", "28", "Chicago", "Marketing"},
		{"Eve", "32", "New York", "Sales"},
		{"Frank", "45", "San Francisco", "Engineering"},
	}

	b, err := createNewBufferWithData(data, true)
	if err != nil {
		t.Fatalf("Failed to create buffer: %v", err)
	}
	b.rowFreeze = 1

	t.Run("Apply single filter", func(t *testing.T) {
		// Filter by City = "New York"
		filtered := b.filterByColumn(2, FilterOptions{Query: "New York", Operator: "equals"})

		// Should have header + 3 data rows
		expected := 4 // 1 header + 3 matching rows
		if filtered.rowLen != expected {
			t.Errorf("Expected %d rows, got %d", expected, filtered.rowLen)
		}
	})

	t.Run("Apply multiple filters sequentially", func(t *testing.T) {
		// First filter by City = "New York"
		filtered1 := b.filterByColumn(2, FilterOptions{Query: "New York", Operator: "equals"})

		// Then filter by Department = "Engineering"
		filtered2 := filtered1.filterByColumn(3, FilterOptions{Query: "Engineering", Operator: "equals"})

		// Should have header + 2 data rows (Alice and Charlie)
		expected := 3 // 1 header + 2 matching rows
		if filtered2.rowLen != expected {
			t.Errorf("Expected %d rows, got %d", expected, filtered2.rowLen)
		}

		// Verify the correct rows remain
		if filtered2.cont[1][0] != "Alice" {
			t.Errorf("Expected Alice in row 1, got %s", filtered2.cont[1][0])
		}
		if filtered2.cont[2][0] != "Charlie" {
			t.Errorf("Expected Charlie in row 2, got %s", filtered2.cont[2][0])
		}
	})

	t.Run("Apply three filters", func(t *testing.T) {
		// Filter by City = "New York"
		filtered1 := b.filterByColumn(2, FilterOptions{Query: "New York", Operator: "equals"})

		// Filter by Department containing "Eng"
		filtered2 := filtered1.filterByColumn(3, FilterOptions{Query: "Eng", Operator: "contains"})

		// Filter by Age > 32
		filtered2.detectAllColumnTypes()
		filtered3 := filtered2.filterByColumn(1, FilterOptions{Query: "32", Operator: ">"})

		// Should have header + 1 data row (Charlie:35)
		expected := 2
		if filtered3.rowLen != expected {
			t.Errorf("Expected %d rows after 3 filters, got %d", expected, filtered3.rowLen)
		}
	})

	t.Run("Filter with no results", func(t *testing.T) {
		// Filter by City = "New York"
		filtered1 := b.filterByColumn(2, FilterOptions{Query: "New York", Operator: "equals"})

		// Filter by Department = "Marketing" (no New York Marketing employees)
		filtered2 := filtered1.filterByColumn(3, FilterOptions{Query: "Marketing", Operator: "equals"})

		// Should have header only
		expected := 1
		if filtered2.rowLen != expected {
			t.Errorf("Expected %d row (header only), got %d", expected, filtered2.rowLen)
		}
	})

	t.Run("Filters preserve column types", func(t *testing.T) {
		b.colType[1] = colTypeFloat // Set Age as numeric
		b.colType[2] = colTypeStr   // Set City as string

		filtered := b.filterByColumn(2, FilterOptions{Query: "New York", Operator: "equals"})

		// Check that column types are preserved
		if filtered.colType[1] != colTypeFloat {
			t.Errorf("Column type for Age not preserved after filter")
		}
		if filtered.colType[2] != colTypeStr {
			t.Errorf("Column type for City not preserved after filter")
		}
	})
}
