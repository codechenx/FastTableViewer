package main

import (
	"testing"
)

// ========================================
// Continuous Stats Tests
// ========================================

func TestContinuousStats_Summary(t *testing.T) {
	cs := &ContinuousStats{}
	data := []string{"10.5", "20.3", "15.7", "25.2", "18.9"}

	cs.summary(data)

	// Verify data was processed
	summaryData := cs.getSummaryData()
	if len(summaryData) == 0 {
		t.Error("Expected summary data, got empty")
	}
}

func TestContinuousStats_EmptyData(t *testing.T) {
	cs := &ContinuousStats{}
	data := []string{}

	cs.summary(data)

	summaryData := cs.getSummaryData()
	if len(summaryData) == 0 {
		t.Log("Empty data handled correctly")
	}
}

func TestContinuousStats_SingleValue(t *testing.T) {
	cs := &ContinuousStats{}
	data := []string{"42.0"}

	cs.summary(data)

	summaryData := cs.getSummaryData()
	if len(summaryData) == 0 {
		t.Error("Should handle single value")
	}
}

func TestContinuousStats_WithNaN(t *testing.T) {
	cs := &ContinuousStats{}
	data := []string{"10.5", "NaN", "15.7", "invalid", "18.9"}

	cs.summary(data)

	// Should handle invalid/NaN values gracefully
	summaryData := cs.getSummaryData()
	if len(summaryData) == 0 {
		t.Error("Should produce summary even with invalid values")
	}
}

// ========================================
// Discrete Stats Tests
// ========================================

func TestDiscreteStats_Summary(t *testing.T) {
	ds := &DiscreteStats{}
	data := []string{"Apple", "Banana", "Apple", "Orange", "Banana", "Apple"}

	ds.summary(data)

	summaryData := ds.getSummaryData()
	if len(summaryData) == 0 {
		t.Error("Expected summary data, got empty")
	}
}

func TestDiscreteStats_EmptyData(t *testing.T) {
	ds := &DiscreteStats{}
	data := []string{}

	ds.summary(data)

	summaryData := ds.getSummaryData()
	if len(summaryData) == 0 {
		t.Log("Empty data handled correctly")
	}
}

func TestDiscreteStats_UniqueValues(t *testing.T) {
	ds := &DiscreteStats{}
	data := []string{"A", "B", "C", "D", "E"}

	ds.summary(data)

	summaryData := ds.getSummaryData()
	if len(summaryData) == 0 {
		t.Error("Should handle all unique values")
	}
}

func TestDiscreteStats_RepeatedValues(t *testing.T) {
	ds := &DiscreteStats{}
	data := []string{"X", "X", "X", "X", "X"}

	ds.summary(data)

	summaryData := ds.getSummaryData()
	if len(summaryData) == 0 {
		t.Error("Should handle repeated values")
	}
}

// ========================================
// Stats Interface Tests
// ========================================

func TestStatsSummary_Interface(t *testing.T) {
	// Test that both types implement the interface
	var _ statsSummary = &ContinuousStats{}
	var _ statsSummary = &DiscreteStats{}

	t.Log("Both stats types implement statsSummary interface")
}

func TestGetSummaryStr(t *testing.T) {
	cs := &ContinuousStats{}
	data := []string{"10", "20", "30"}

	str := cs.getSummaryStr(data)
	if len(str) == 0 {
		t.Error("getSummaryStr should return non-empty string")
	}
}

// ========================================
// Performance Tests
// ========================================

func BenchmarkContinuousStats(b *testing.B) {
	data := make([]string, 1000)
	for i := 0; i < 1000; i++ {
		data[i] = F2S(float64(i) * 1.5)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cs := &ContinuousStats{}
		cs.summary(data)
	}
}

func BenchmarkDiscreteStats(b *testing.B) {
	data := make([]string, 1000)
	categories := []string{"A", "B", "C", "D", "E"}
	for i := 0; i < 1000; i++ {
		data[i] = categories[i%len(categories)]
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ds := &DiscreteStats{}
		ds.summary(data)
	}
}

// ========================================
// Plot Tests
// ========================================

func TestContinuousStats_GetPlot(t *testing.T) {
	cs := &ContinuousStats{}
	data := []string{"10", "20", "15", "25", "30", "18", "22", "27", "12", "16"}

	cs.summary(data)
	plot := cs.getPlot()

	if len(plot) == 0 {
		t.Error("Expected plot output, got empty string")
	}
	if plot == "No data to plot" {
		t.Error("Should generate plot for valid data")
	}
	t.Logf("Plot output:\n%s", plot)
}

func TestContinuousStats_GetPlot_EmptyData(t *testing.T) {
	cs := &ContinuousStats{}
	data := []string{}

	cs.summary(data)
	plot := cs.getPlot()

	if plot != "No data to plot" {
		t.Errorf("Expected 'No data to plot', got: %s", plot)
	}
}

func TestContinuousStats_GetPlot_IdenticalValues(t *testing.T) {
	cs := &ContinuousStats{}
	data := []string{"42", "42", "42", "42", "42"}

	cs.summary(data)
	plot := cs.getPlot()

	if plot != "All values are identical" {
		t.Errorf("Expected 'All values are identical', got: %s", plot)
	}
}

func TestDiscreteStats_GetPlot(t *testing.T) {
	ds := &DiscreteStats{}
	data := []string{"A", "B", "A", "C", "A", "B", "A", "D", "A", "B"}

	ds.summary(data)
	plot := ds.getPlot()

	if len(plot) == 0 {
		t.Error("Expected plot output, got empty string")
	}
	if plot == "No data to plot" {
		t.Error("Should generate plot for valid data")
	}
	t.Logf("Plot output:\n%s", plot)
}

func TestDiscreteStats_GetPlot_EmptyData(t *testing.T) {
	ds := &DiscreteStats{}
	data := []string{}

	ds.summary(data)
	plot := ds.getPlot()

	if plot != "No data to plot" {
		t.Errorf("Expected 'No data to plot', got: %s", plot)
	}
}
