package main

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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

// ============================================================
// Loading progress readout
// ============================================================

func TestFormatCount(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"0", "0"}, {"7", "7"}, {"999", "999"},
		{"1000", "1,000"}, {"12345", "12,345"},
		{"100000", "100,000"}, {"1234567", "1,234,567"},
	} {
		n, _ := strconv.Atoi(tc.in)
		if got := formatCount(n); got != tc.want {
			t.Errorf("formatCount(%s) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestProgressBar(t *testing.T) {
	const w = 10

	for _, tc := range []struct {
		percent float64
		filled  int
	}{
		{0, 0}, {5, 1}, {10, 1}, {50, 5}, {99, 9}, {100, 10},
		{-20, 0},  // clamped low
		{150, 10}, // clamped high
	} {
		got := progressBar(tc.percent, w)
		if n := len([]rune(got)); n != w {
			t.Errorf("progressBar(%v, %d) has %d cells, want %d", tc.percent, w, n, w)
		}
		filled := strings.Count(got, "█")
		if filled != tc.filled {
			t.Errorf("progressBar(%v, %d) filled %d cells, want %d (%q)",
				tc.percent, w, filled, tc.filled, got)
		}
	}

	if progressBar(50, 0) != "" {
		t.Error("a zero-width bar should render as empty")
	}
}

// Any progress at all must show a sliver, so the bar never looks stalled while
// rows are arriving.
func TestProgressBar_ShowsSliverForTinyProgress(t *testing.T) {
	if got := progressBar(0.01, 20); strings.Count(got, "█") != 1 {
		t.Errorf("progressBar(0.01, 20) = %q, want exactly one filled cell", got)
	}
	if got := progressBar(0, 20); strings.Count(got, "█") != 0 {
		t.Errorf("progressBar(0, 20) = %q, want no filled cells", got)
	}
}

// A source with a known size gets a determinate bar; one without gets a
// spinner, because no honest percentage exists for a stream or a .gz.
func TestBuildLoadingStatus(t *testing.T) {
	determinate := buildLoadingStatus(1500, 50, 200, 0)
	if !strings.Contains(determinate, "█") {
		t.Errorf("determinate status %q should contain a bar", determinate)
	}
	if !strings.Contains(determinate, "25.0%") {
		t.Errorf("determinate status %q should report 25.0%%", determinate)
	}
	if !strings.Contains(determinate, "1,500 rows") {
		t.Errorf("determinate status %q should report a separated row count", determinate)
	}

	indeterminate := buildLoadingStatus(42, 0, 0, 0)
	if strings.Contains(indeterminate, "█") || strings.Contains(indeterminate, "%") {
		t.Errorf("indeterminate status %q should show neither bar nor percentage", indeterminate)
	}
	if !strings.ContainsRune(indeterminate, spinnerFrames[0]) {
		t.Errorf("indeterminate status %q should show a spinner frame", indeterminate)
	}
	if !strings.Contains(indeterminate, "42 rows") {
		t.Errorf("indeterminate status %q should report the row tally", indeterminate)
	}
}

// Bytes beyond the reported total must not produce a bar wider than its width
// or a percentage above 100.
func TestBuildLoadingStatus_ClampsOvershoot(t *testing.T) {
	got := buildLoadingStatus(10, 500, 100, 0)
	if !strings.Contains(got, "100.0%") {
		t.Errorf("status %q should clamp to 100.0%%", got)
	}
	if strings.Count(got, "█") != progressBarWidth {
		t.Errorf("status %q should fill exactly %d cells", got, progressBarWidth)
	}
}

func TestBuildLoadingStatus_SpinnerAdvances(t *testing.T) {
	seen := map[rune]bool{}
	for tick := 0; tick < spinnerTicksPerFrame*len(spinnerFrames); tick++ {
		for _, r := range buildLoadingStatus(1, 0, 0, tick) {
			if r >= '⠀' && r <= '⣿' {
				seen[r] = true
			}
		}
	}
	if len(seen) != len(spinnerFrames) {
		t.Errorf("saw %d distinct spinner frames over one cycle, want %d", len(seen), len(spinnerFrames))
	}
}

// trickleReader delivers its chunks one Read at a time, pausing between them,
// standing in for a slow producer on the other end of a pipe.
type trickleReader struct {
	chunks []string
	i      int
	pause  time.Duration
}

func (r *trickleReader) Read(p []byte) (int, error) {
	if r.i >= len(r.chunks) {
		return 0, io.EOF
	}
	if r.i > 0 {
		time.Sleep(r.pause)
	}
	n := copy(p, r.chunks[r.i])
	r.i++
	return n, nil
}

// A batch used to be flushed only once full or at EOF, so a slow stream showed
// nothing beyond the separator-detection head until its producer closed. Rows
// must reach the buffer, and progress must be reported, while data is still
// arriving.
func TestIntake_FlushesPartialBatchesWhileStreaming(t *testing.T) {
	chunks := []string{"a,b,c\n"}
	for i := 0; i < intakeDetectLines; i++ {
		chunks[0] += I2S(i) + ",x,y\n"
	}
	for i := 0; i < 40; i++ {
		chunks = append(chunks, I2S(100+i)+",x,y\n")
	}

	b := createNewBuffer()
	b.rowFreeze = 1

	var mu sync.Mutex
	reports := 0
	in := Intake{
		Source:     Source{Reader: &trickleReader{chunks: chunks, pause: 15 * time.Millisecond}},
		Config:     IntakeConfig{Sep: ','},
		OnProgress: func(int, int64, int64) { mu.Lock(); reports++; mu.Unlock() },
	}

	done := make(chan error, 1)
	go func() { done <- in.Into(b) }()

	// Well before the stream ends (40 chunks x 15ms = 600ms), rows past the
	// head batch must already be visible.
	time.Sleep(300 * time.Millisecond)

	b.mu.RLock()
	midLoad := b.rowLen
	b.mu.RUnlock()
	mu.Lock()
	midReports := reports
	mu.Unlock()

	if midLoad <= intakeDetectLines+1 {
		t.Errorf("only %d rows landed mid-stream; the batch is not being flushed until EOF", midLoad)
	}
	if midReports < 2 {
		t.Errorf("only %d progress reports mid-stream, want several", midReports)
	}

	if err := <-done; err != nil {
		t.Fatal(err)
	}
	b.mu.RLock()
	total := b.rowLen
	b.mu.RUnlock()
	if total != 1+intakeDetectLines+40 {
		t.Errorf("loaded %d rows in total, want %d", total, 1+intakeDetectLines+40)
	}
}

// ============================================================
// Issue #25: separator detection and -s both failed
// ============================================================

// Supplying -s made the loader skip the block that reads the first lines, so
// no rows had been appended when it signalled the UI, and the emptiness check
// fired immediately: every forced separator reported "File is empty".
func TestIssue25_ForcedSeparatorLoadsRows(t *testing.T) {
	for _, tc := range []struct {
		name string
		sep  rune
		data string
	}{
		{"semicolon", ';', "name;age;city\nAlice;30;Berlin\nBob;25;Paris\n"},
		{"tab", '\t', "name\tage\tcity\nAlice\t30\tBerlin\nBob\t25\tParis\n"},
		{"pipe", '|', "name|age|city\nAlice|30|Berlin\nBob|25|Paris\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := createNewBuffer()
			b.rowFreeze = 1

			reportedRows := -1
			in := Intake{
				Source:     pipeSource(strings.NewReader(tc.data)),
				Config:     IntakeConfig{Sep: tc.sep},
				OnProgress: func(rows int, _, _ int64) { reportedRows = rows },
			}
			if err := in.Into(b); err != nil {
				t.Fatal(err)
			}

			if b.rowLen != 3 {
				t.Errorf("loaded %d rows, want 3", b.rowLen)
			}
			if b.colLen != 3 {
				t.Errorf("found %d columns, want 3", b.colLen)
			}
			// The first report must already carry rows, or the caller's
			// emptiness check sees an empty buffer and gives up.
			if reportedRows <= 0 {
				t.Errorf("first progress report carried %d rows, want more than 0", reportedRows)
			}
		})
	}
}

// Detection required every sampled line to hold an identical number of
// separators, so one irregular line rejected the separator and the file could
// not be opened at all.
func TestIssue25_DetectionToleratesIrregularLines(t *testing.T) {
	sd := sepDetecor{}

	for _, tc := range []struct {
		name  string
		lines []string
		want  rune
	}{
		{
			// A separator inside a quoted field is data, not structure.
			"separator inside a quoted field",
			[]string{`name;note;age`, `Alice;"has;semi";30`, `Bob;plain;25`, `Carol;"a;b;c";35`},
			';',
		},
		{
			// A column pasted in by hand leaves one row wider than the rest.
			"a ragged row",
			[]string{"a\tb\tc", "1\t2\t3", "4\t5\t6\t7", "8\t9"},
			'\t',
		},
		{
			"a blank line in the sample",
			[]string{"id;name", "1;Alice", "", "2;Bob", "3;Carol"},
			';',
		},
		{
			// The real separator is absent from the first line entirely.
			"a leading comment line",
			[]string{"# exported from a tool", "name;age;city", "Alice;30;Berlin", "Bob;25;Paris"},
			';',
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := sd.sepDetect(tc.lines); got != tc.want {
				t.Errorf("sepDetect() = %q, want %q", got, tc.want)
			}
		})
	}
}

// Tolerance must not turn into guessing: a single column still has no
// separator, and one stray character in one line is not structure.
func TestIssue25_DetectionStillRefusesNonTabular(t *testing.T) {
	sd := sepDetecor{}

	for _, tc := range []struct {
		name  string
		lines []string
	}{
		{"a single alphanumeric run", []string{"justoneword", "anotherword", "thirdword"}},
		{"no lines at all", nil},
		{"only blank lines", []string{"", "   ", ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := sd.sepDetect(tc.lines); got != 0 {
				t.Errorf("sepDetect() = %q, want no separator", got)
			}
		})
	}
}

// A consistent separator should win over one that only appears sporadically,
// even when the sporadic one ranks higher by preference.
func TestIssue25_ConsistencyBeatsPreference(t *testing.T) {
	sd := sepDetecor{}

	lines := []string{
		"name;note;age",
		"Alice;hello, world;30",
		"Bob;plain;25",
		"Carol;a, b, c;35",
	}
	if got := sd.sepDetect(lines); got != ';' {
		t.Errorf("sepDetect() = %q, want ';' — the comma appears only in some lines", got)
	}
}
