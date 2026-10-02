package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ============================================================
// Candidate 01: one intake module
// ============================================================

// Skip rules must apply the same way whatever the source. The file loaders
// guarded skip-lines with an extra "only when -n is set" clause that the pipe
// loaders did not, so --skip-lines was silently ignored for files.
func TestIntake_SkipLinesAppliesToEverySourceWithoutLineLimit(t *testing.T) {
	const data = "junk1\njunk2\nName,Age\nJohn,30\nJane,25\n"
	cfg := IntakeConfig{Sep: ',', SkipLines: 2}

	dir := t.TempDir()
	path := filepath.Join(dir, "sample.txt")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}

	src, err := openFileSource(path)
	if err != nil {
		t.Fatal(err)
	}
	fromFile := createNewBuffer()
	if err := (Intake{Source: src, Config: cfg}).Into(fromFile); err != nil {
		t.Fatal(err)
	}

	fromPipe := createNewBuffer()
	if err := (Intake{Source: pipeSource(strings.NewReader(data)), Config: cfg}).Into(fromPipe); err != nil {
		t.Fatal(err)
	}

	if fromFile.rowLen != 3 {
		t.Errorf("file load kept %d rows, want 3 (skip-lines ignored?)", fromFile.rowLen)
	}
	if fromPipe.rowLen != 3 {
		t.Errorf("pipe load kept %d rows, want 3", fromPipe.rowLen)
	}
	if fromFile.rowLen != fromPipe.rowLen {
		t.Errorf("file kept %d rows but pipe kept %d: skip rules differ by source",
			fromFile.rowLen, fromPipe.rowLen)
	}
	if fromFile.cont[0][0] != "Name" {
		t.Errorf("first row is %v, want the header", fromFile.cont[0])
	}
}

// The loaders decremented SkipNum on the shared Args as they read, so a second
// load in the same process behaved differently from the first.
func TestIntakeConfig_IsNotConsumedByLoading(t *testing.T) {
	const data = "junk1\njunk2\nName,Age\nJohn,30\n"
	cfg := IntakeConfig{Sep: ',', SkipLines: 2}

	first := createNewBuffer()
	if err := (Intake{Source: pipeSource(strings.NewReader(data)), Config: cfg}).Into(first); err != nil {
		t.Fatal(err)
	}

	second := createNewBuffer()
	if err := (Intake{Source: pipeSource(strings.NewReader(data)), Config: cfg}).Into(second); err != nil {
		t.Fatal(err)
	}

	if cfg.SkipLines != 2 {
		t.Errorf("cfg.SkipLines = %d after loading, want 2", cfg.SkipLines)
	}
	if first.rowLen != second.rowLen {
		t.Errorf("first load read %d rows, second read %d: the config was consumed",
			first.rowLen, second.rowLen)
	}
}

// Nothing closed the file handle when a bare scanner was handed out.
func TestIntake_ClosesItsSource(t *testing.T) {
	closed := false
	src := Source{
		Reader: strings.NewReader("a,b\n1,2\n"),
		Close:  func() error { closed = true; return nil },
	}

	if err := (Intake{Source: src}).Into(createNewBuffer()); err != nil {
		t.Fatal(err)
	}
	if !closed {
		t.Error("Intake should close the source it was given")
	}
}

// An unidentifiable separator is an error from every source. The sync loaders
// called fatalError, exiting the process, while the async pair reported it.
func TestIntake_UnidentifiableSeparatorIsAnError(t *testing.T) {
	b := createNewBuffer()
	// A single alphanumeric run offers no separator candidate at all. (A line
	// with spaces would detect space, which is deliberate.)
	err := (Intake{Source: pipeSource(strings.NewReader("justoneword\n"))}).Into(b)
	if err == nil {
		t.Error("expected an error when no separator can be identified")
	}
}

func TestIntake_MaxLinesStopsEarly(t *testing.T) {
	var in strings.Builder
	in.WriteString("idx\n")
	for i := 1; i <= 100; i++ {
		in.WriteString(I2S(i) + "\n")
	}

	b := createNewBuffer()
	cfg := IntakeConfig{Sep: ',', MaxLines: 5}
	if err := (Intake{Source: pipeSource(strings.NewReader(in.String())), Config: cfg}).Into(b); err != nil {
		t.Fatal(err)
	}
	if b.rowLen != 5 {
		t.Errorf("rows = %d, want 5", b.rowLen)
	}
}

// ============================================================
// Candidate 02: the view owns derived state
// ============================================================

// drawUI took a *Buffer parameter that shadowed the package-level one, so
// reassigning it on filter left the footer helpers reading the old buffer: the
// column type used for sorting and the type shown in the footer diverged.
// Going through the view, a type change is visible everywhere and survives the
// buffer swap that filtering performs.
func TestViewState_ColumnTypeSurvivesFiltering(t *testing.T) {
	b, err := createNewBufferWithData([][]string{
		{"Name", "Age"},
		{"Alice", "30"},
		{"Bob", "25"},
		{"Carol", "35"},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	b.rowFreeze = 1
	b.detectAllColumnTypes()

	v := NewViewState(b)

	rows, kept := v.ApplyFilter(0, FilterOptions{Query: "a", Operator: opContains})
	if !kept || rows == 0 {
		t.Fatalf("filter kept %d rows (kept=%v), expected some", rows, kept)
	}

	want := v.CycleColType(1)
	if got := v.ColType(1); got != want {
		t.Errorf("ColType(1) = %v straight after cycling, want %v", got, want)
	}

	v.ClearAllFilters()
	if got := v.ColType(1); got != want {
		t.Errorf("ColType(1) = %v after clearing filters, want %v: "+
			"the type was set on the filtered copy only", got, want)
	}
}

// A filter matching nothing must not blank the view.
func TestViewState_FilterMatchingNothingIsDiscarded(t *testing.T) {
	b, err := createNewBufferWithData([][]string{
		{"Name"}, {"Alice"}, {"Bob"},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	b.rowFreeze = 1

	v := NewViewState(b)
	rows, kept := v.ApplyFilter(0, FilterOptions{Query: "zzz", Operator: opContains})
	if kept {
		t.Error("a filter matching nothing should not be kept")
	}
	if rows != 0 {
		t.Errorf("rows = %d, want 0", rows)
	}
	if v.Filtered() {
		t.Error("the view should report no active filter")
	}
	if v.DataRows() != 2 {
		t.Errorf("DataRows() = %d, want 2: the previous view should stand", v.DataRows())
	}
}

// Search coordinates index the visible buffer, so they have to be recomputed
// when filtering swaps it; otherwise the highlight lands on unrelated cells.
func TestViewState_SearchIsRecomputedWhenFiltersChange(t *testing.T) {
	b, err := createNewBufferWithData([][]string{
		{"Name"}, {"Alice"}, {"Bob"}, {"Anna"},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	b.rowFreeze = 1

	v := NewViewState(b)
	if found := v.Search(MatchSpec{Query: "a"}); found == 0 {
		t.Fatal("expected matches for \"a\"")
	}

	if _, kept := v.ApplyFilter(0, FilterOptions{Query: "Anna", Operator: opEquals}); !kept {
		t.Fatal("expected the Anna filter to be kept")
	}

	// Every surviving match must be inside the filtered buffer.
	rows, cols := v.Dims()
	for _, r := range v.results {
		if r.Row >= rows || r.Col >= cols {
			t.Errorf("match at (%d,%d) is outside the %dx%d visible buffer", r.Row, r.Col, rows, cols)
		}
	}
}

// createNewBufferWithData assigned the package-level buffer instead of a local,
// so every test fixture clobbered shared state.
func TestCreateNewBufferWithData_ReturnsIndependentBuffers(t *testing.T) {
	first, err := createNewBufferWithData([][]string{{"a"}, {"1"}}, false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := createNewBufferWithData([][]string{{"b"}, {"2"}}, false)
	if err != nil {
		t.Fatal(err)
	}

	if first == second {
		t.Fatal("both fixtures are the same buffer")
	}
	if first.cont[0][0] != "a" {
		t.Errorf("first fixture holds %q, want \"a\": it was overwritten", first.cont[0][0])
	}
}

// ============================================================
// Candidate 03: one matcher behind search and filter
// ============================================================

// Search folded case with a hand-rolled lowercaser that mapped only A-Z, while
// filter used strings.ToLower, so the two disagreed on non-ASCII text.
func TestSearchAndFilterFoldCaseIdentically(t *testing.T) {
	b, err := createNewBufferWithData([][]string{{"City"}, {"ÉCOLE"}}, false)
	if err != nil {
		t.Fatal(err)
	}
	b.rowFreeze = 1

	found := performSearch(b, MatchSpec{Query: "école"})
	if len(found) != 1 {
		t.Errorf("case-insensitive search matched %d cells, want 1", len(found))
	}

	filtered := b.filterByColumn(0, FilterOptions{Query: "école", Operator: opContains})
	if got := filtered.rowLen - filtered.rowFreeze; got != 1 {
		t.Errorf("case-insensitive filter kept %d rows, want 1", got)
	}
}

// The filter had no case-insensitive regex path at all: the pattern was always
// matched against the raw cell, whatever the checkbox said.
func TestFilterRegexHonoursCaseSensitivity(t *testing.T) {
	b, err := createNewBufferWithData([][]string{{"Status"}, {"Active"}, {"inactive"}}, false)
	if err != nil {
		t.Fatal(err)
	}
	b.rowFreeze = 1

	insensitive := b.filterByColumn(0, FilterOptions{Query: "^active$", Operator: opRegex})
	if got := insensitive.rowLen - insensitive.rowFreeze; got != 1 {
		t.Errorf("case-insensitive regex kept %d rows, want 1", got)
	}

	sensitive := b.filterByColumn(0, FilterOptions{
		Query: "^active$", Operator: opRegex, CaseSensitive: true,
	})
	if got := sensitive.rowLen - sensitive.rowFreeze; got != 0 {
		t.Errorf("case-sensitive regex kept %d rows, want 0", got)
	}
}

// An unusable pattern is reported, not silently treated as "matches nothing".
func TestCompileMatcher_ReportsBadRegex(t *testing.T) {
	if _, err := CompileMatcher(MatchSpec{Query: "[invalid(", Operator: opRegex}, colTypeStr); err == nil {
		t.Error("expected an error for an invalid pattern")
	}
}

// A comparison on a date column used the numeric parser, which reads every
// date as 0, so every row compared equal. The matcher now uses the column
// type's own parser.
func TestMatcher_ComparesDatesOnDateColumns(t *testing.T) {
	b, err := createNewBufferWithData([][]string{
		{"Event", "When"},
		{"Early", "2024-01-15"},
		{"Late", "2024-12-31"},
	}, false)
	if err != nil {
		t.Fatal(err)
	}
	b.rowFreeze = 1
	b.detectAllColumnTypes()

	if got := b.getColType(1); got != colTypeDate {
		t.Fatalf("column 1 detected as %v, want Date", type2name(got))
	}

	filtered := b.filterByColumn(1, FilterOptions{Query: "2024-06-01", Operator: opGT})
	if got := filtered.rowLen - filtered.rowFreeze; got != 1 {
		t.Fatalf("date comparison kept %d rows, want 1", got)
	}
	if filtered.cont[1][0] != "Late" {
		t.Errorf("surviving row is %q, want \"Late\"", filtered.cont[1][0])
	}
}

// A cell carrying no value must not satisfy a comparison. The old code parsed
// it to 0, so "NA" counted as less than any positive threshold.
func TestMatcher_MissingValuesFailComparisons(t *testing.T) {
	m, err := CompileMatcher(MatchSpec{Query: "10", Operator: opLT}, colTypeFloat)
	if err != nil {
		t.Fatal(err)
	}
	for _, cell := range []string{"", "NA", "N/A", "NaN", "null", "banana"} {
		if m.MatchCell(cell) {
			t.Errorf("MatchCell(%q) = true for a < comparison, want false", cell)
		}
	}
	if !m.MatchCell("5") {
		t.Error(`MatchCell("5") = false for "< 10", want true`)
	}
}

// ============================================================
// Candidate 04: detection and ordering share one parser
// ============================================================

// "1,2,3" is not a valid thousands grouping. Detection used a byte scanner
// that accepted stray commas while ordering stripped them, so the value was
// detected as numeric and then ordered as 123.
func TestNumericDetectionAgreesWithOrdering(t *testing.T) {
	if isNumericValue("1,2,3") {
		t.Error(`isNumericValue("1,2,3") = true, want false`)
	}
	if v := parseNumericValueFast("1,2,3"); v != 0 {
		t.Errorf(`parseNumericValueFast("1,2,3") = %v, want 0`, v)
	}

	// Valid groupings and digit separators still parse.
	for _, tc := range []struct {
		in   string
		want float64
	}{
		{"1,234.56", 1234.56},
		{"1_234_567", 1234567},
		{"12,345", 12345},
		{"-1,234", -1234},
	} {
		if !isNumericValue(tc.in) {
			t.Errorf("isNumericValue(%q) = false, want true", tc.in)
		}
		if got := parseNumericValueFast(tc.in); got != tc.want {
			t.Errorf("parseNumericValueFast(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// Every layout in the table must be both detectable and orderable. The two
// halves each carried their own copy of the list, and the detection guard
// rejected a layout the list contained.
func TestEveryDateLayoutIsBothDetectedAndOrdered(t *testing.T) {
	stamp := time.Date(2026, 3, 4, 15, 30, 0, 0, time.UTC)

	for _, format := range dateFormats {
		sample := stamp.Format(format)
		if !isDateValue(sample) {
			t.Errorf("isDateValue(%q) = false for layout %q", sample, format)
		}
		if parseDateValueFast(sample) == 0 {
			t.Errorf("parseDateValueFast(%q) = 0 for layout %q", sample, format)
		}
	}
}

// Sorting asks the column's type for the comparison, so callers no longer
// switch on type themselves.
func TestSortBy_UsesTheColumnsOwnOrdering(t *testing.T) {
	data := [][]string{{"n"}, {"10"}, {"9"}, {"100"}}

	lexical, err := createNewBufferWithData(data, false)
	if err != nil {
		t.Fatal(err)
	}
	lexical.rowFreeze = 1
	lexical.SortBy(0, false)
	if got := lexical.cont[1][0]; got != "10" {
		t.Errorf("string column sorted to %q first, want \"10\"", got)
	}

	numeric, err := createNewBufferWithData(data, false)
	if err != nil {
		t.Fatal(err)
	}
	numeric.rowFreeze = 1
	numeric.setColType(0, colTypeFloat)
	numeric.SortBy(0, false)
	if got := numeric.cont[1][0]; got != "9" {
		t.Errorf("numeric column sorted to %q first, want \"9\"", got)
	}
}

// ============================================================
// Candidate 05: the buffer owns its concurrency invariant
// ============================================================

// setColType and getColType took no lock, and detectAllColumnTypes ran in a
// bare goroutine while the UI read types on every cursor move. Run under
// -race, this fails if any of those paths is unguarded.
func TestBuffer_ColumnTypesAreSafeDuringLoad(t *testing.T) {
	b := createNewBuffer()
	b.rowFreeze = 1

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 400; i++ {
			_ = b.contAppendSli([]string{I2S(i), "2024-01-15", "text"}, false)
		}
	}()

	for i := 0; i < 400; i++ {
		b.detectAllColumnTypes()
		_ = b.getColType(i % 3)
		b.setColType(i%3, colTypeStr)
		b.SortBy(0, i%2 == 0)
	}
	<-done
}

// A later, wider row grew colLen without growing the type slice, so a column
// index valid for the table was out of range for its types.
func TestBuffer_ColumnTypeSliceTracksWidth(t *testing.T) {
	b := createNewBuffer()
	if err := b.contAppendSli([]string{"a", "b"}, false); err != nil {
		t.Fatal(err)
	}
	if err := b.contAppendSli([]string{"a", "b", "c", "d"}, false); err != nil {
		t.Fatal(err)
	}

	if b.colLen != 4 {
		t.Fatalf("colLen = %d, want 4", b.colLen)
	}
	if len(b.colType) < b.colLen {
		t.Errorf("len(colType) = %d, want at least colLen %d", len(b.colType), b.colLen)
	}

	// These must not panic for any column the table now has.
	for i := 0; i < b.colLen; i++ {
		b.setColType(i, colTypeFloat)
		if got := b.getColType(i); got != colTypeFloat {
			t.Errorf("getColType(%d) = %v, want Num", i, type2name(got))
		}
	}
}

// Reading a ragged file must not panic: rows short of the full width are padded
// and every accessor bounds-checks.
func TestBuffer_HandlesRaggedRows(t *testing.T) {
	b := createNewBuffer()
	if err := loadFileInto("./data/test/inconsistent_columns.csv", b); err != nil {
		t.Skipf("fixture unavailable: %v", err)
	}
	b.detectAllColumnTypes()

	for c := 0; c < b.colLen; c++ {
		_ = b.getCol(c)
		_ = b.getColType(c)
		b.SortBy(c, false)
	}
}
