package main

import (
	"strings"
	"testing"
)

// ========================================
// Type Conversion Tests
// ========================================

func TestI2B(t *testing.T) {
	type args struct {
		i int
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"Positive number 1", args{i: 1}, true},
		{"Positive number 2", args{i: 2}, true},
		{"Zero value", args{i: 0}, false},
		{"Negative value", args{i: -1}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := I2B(tt.args.i); got != tt.want {
				t.Errorf("I2B() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestI2S_Basic(t *testing.T) {
	result := I2S(42)
	if result != "42" {
		t.Errorf("I2S(42) = %s, want 42", result)
	}
}

func TestF2S_Basic(t *testing.T) {
	result := F2S(42.0)
	if result == "" {
		t.Error("F2S should return a non-empty string")
	}
	t.Logf("F2S(42.0) = %s", result)
}

func TestS2F_Valid(t *testing.T) {
	result := S2F("3.14")
	if result != 3.14 {
		t.Errorf("S2F('3.14') = %f, want 3.14", result)
	}
}

func TestS2F_Invalid(t *testing.T) {
	t.Skip("Skipping S2F invalid test - function calls os.Exit on error")
}

// ========================================
// Text Wrapping Tests
// ========================================

func TestWrapText_Basic(t *testing.T) {
	result := wrapText("Hello World", 25)
	if len(result) < 1 {
		t.Error("wrapText should return at least one character")
	}
}

func TestWrapText_Long(t *testing.T) {
	longText := "This is a very long text that needs to be wrapped at twenty-five characters to test the wrapping functionality properly"
	result := wrapText(longText, 25)

	// Check that text was wrapped (contains newlines or is within limit)
	if len(longText) > 25 && len(result) == len(longText) {
		t.Error("Long text should be wrapped")
	}
}

func TestWrapText_Empty(t *testing.T) {
	result := wrapText("", 25)
	t.Logf("wrapText('', 25) returned: '%s'", result)
}

func TestWrapText_NoWrapNeeded(t *testing.T) {
	short := "Short"
	result := wrapText(short, 25)
	if result != short {
		t.Errorf("Short text should not be wrapped: got '%s', want '%s'", result, short)
	}
}

func TestWrapText_ExactLength(t *testing.T) {
	text := "Exactly25CharactersHere!!" // 25 characters
	result := wrapText(text, 25)
	if result != text {
		t.Errorf("Text at exact length should not be wrapped")
	}
}

// ========================================
// Text Truncation Tests
// ========================================

func TestTruncateText_Short(t *testing.T) {
	text := "Short"
	result := truncateText(text, 25)
	if result != text {
		t.Errorf("Short text should not be truncated: got '%s', want '%s'", result, text)
	}
}

func TestTruncateText_Long(t *testing.T) {
	text := "This is a very long text that needs to be truncated"
	result := truncateText(text, 20)
	expected := "This is a very lo..."
	if result != expected {
		t.Errorf("Long text should be truncated: got '%s', want '%s'", result, expected)
	}
	// Check that result doesn't exceed maxWidth
	if len([]rune(result)) > 20 {
		t.Errorf("Truncated text exceeds maxWidth: got length %d, want 20", len([]rune(result)))
	}
}

func TestTruncateText_ExactLength(t *testing.T) {
	text := "Exactly20Characters!"
	result := truncateText(text, 20)
	if result != text {
		t.Errorf("Text at exact length should not be truncated: got '%s', want '%s'", result, text)
	}
}

func TestTruncateText_ZeroWidth(t *testing.T) {
	text := "Some text"
	result := truncateText(text, 0)
	if result != text {
		t.Errorf("Zero maxWidth should return original text: got '%s', want '%s'", result, text)
	}
}

// ========================================
// Column Width Tests
// ========================================

func TestViewState_ColumnWidth_Unlimited(t *testing.T) {
	v := NewViewState(createNewBuffer())

	if width, limited := v.ColumnWidth(0); limited {
		t.Errorf("a fresh column should carry no width limit, got %d", width)
	}
}

func TestViewState_ToggleWrap(t *testing.T) {
	v := NewViewState(createNewBuffer())

	width, limited := v.ToggleWrap(0)
	if !limited {
		t.Fatal("ToggleWrap should limit an unlimited column")
	}
	if width != defaultWrapWidth {
		t.Errorf("ToggleWrap width = %d, want %d", width, defaultWrapWidth)
	}
	if got, ok := v.ColumnWidth(0); !ok || got != defaultWrapWidth {
		t.Errorf("ColumnWidth(0) = %d, %v; want %d, true", got, ok, defaultWrapWidth)
	}

	if _, limited := v.ToggleWrap(0); limited {
		t.Error("ToggleWrap should remove the limit on a limited column")
	}
	if _, ok := v.ColumnWidth(0); ok {
		t.Error("ColumnWidth should report no limit after the second toggle")
	}
}

func TestViewState_MeasureColumns(t *testing.T) {
	b := createNewBuffer()
	b.rowFreeze = 1
	_ = b.contAppendSli([]string{"name", "note", "age"}, false)
	_ = b.contAppendSli([]string{"Alice", strings.Repeat("x", 120), "30"}, false)
	_ = b.contAppendSli([]string{"Bob", strings.Repeat("y", 110), "25"}, false)
	b.detectAllColumnTypes()

	v := NewViewState(b)
	v.MeasureColumns()

	short := v.Layout(0)
	if short.width != len("Alice") {
		t.Errorf("short column width = %d, want %d", short.width, len("Alice"))
	}
	if short.expand != 0 {
		t.Errorf("a column that fits should not absorb leftover width, got expand %d", short.expand)
	}
	if short.rightAlign {
		t.Error("a string column should align left")
	}

	long := v.Layout(1)
	if long.width != maxColumnWidth {
		t.Errorf("long column width = %d, want the %d cap", long.width, maxColumnWidth)
	}
	if long.expand == 0 {
		t.Error("a column cut short by the cap should absorb leftover width")
	}

	num := v.Layout(2)
	if !num.rightAlign {
		t.Error("a numeric column should align right")
	}
	if num.width > len("age") {
		t.Errorf("numeric column width = %d, want no wider than its header", num.width)
	}
}

// An unmeasured column must still paint, rather than panicking on a bad index.
func TestViewState_LayoutFallsBack(t *testing.T) {
	v := NewViewState(createNewBuffer())
	for _, col := range []int{-1, 0, 99} {
		if got := v.Layout(col); got.width <= 0 {
			t.Errorf("Layout(%d) width = %d, want a usable width", col, got.width)
		}
	}
}

// ========================================
// Help Content Tests
// ========================================

func TestGetHelpContent_NotEmpty(t *testing.T) {
	help := getHelpContent()
	if len(help) == 0 {
		t.Error("getHelpContent() should return non-empty string")
	}
}

func TestGetHelpContent_ContainsBasics(t *testing.T) {
	help := getHelpContent()

	// Check for essential content
	if !contains(help, "Quit") {
		t.Error("Help should contain 'Quit' section")
	}
	if !contains(help, "Movement") {
		t.Error("Help should contain 'Movement' section")
	}
	if !contains(help, "Sort") {
		t.Error("Help should contain 'Sort' section")
	}
}

func TestUsefulInfo_NotEmpty(t *testing.T) {
	// Just verify it doesn't panic
	usefulInfo("test message")
	t.Log("usefulInfo executed successfully")
}

// ========================================
// Type Name Tests
// ========================================

func TestType2Name(t *testing.T) {
	tests := []struct {
		name     string
		colType  ColumnType
		expected string
	}{
		{"String type", colTypeStr, "Str"},
		{"Float type", colTypeFloat, "Num"},
		{"Date type", colTypeDate, "Date"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := type2name(tt.colType)
			if result != tt.expected {
				t.Errorf("type2name(%d) = %s, want %s", tt.colType, result, tt.expected)
			}
		})
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
