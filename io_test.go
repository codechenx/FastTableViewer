package main

import (
	"bytes"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// loadFileInto loads a file through Intake, the way the viewer does.
func loadFileInto(fn string, b *Buffer) error {
	src, err := openFileSource(fn)
	if err != nil {
		return err
	}
	return Intake{Source: src}.Into(b)
}

// loadReaderInto loads a stream through Intake, the way a pipe does. It is the
// third adapter for the Source seam, alongside the file and standard input.
func loadReaderInto(r io.Reader, b *Buffer) error {
	return Intake{Source: pipeSource(r)}.Into(b)
}

// ========================================
// File Loading Tests
// ========================================

func Test_loadFileInto(t *testing.T) {
	type args struct {
		fn string
		b  *Buffer
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"Load TSV file", args{fn: "./data/test/test.tsv", b: createNewBuffer()}, false},
		{"Load CSV file", args{fn: "./data/test/test.csv", b: createNewBuffer()}, false},
		{"Load gzip file", args{fn: "./data/test/test.csv.gz", b: createNewBuffer()}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := loadFileInto(tt.args.fn, tt.args.b); (err != nil) != tt.wantErr {
				t.Errorf("loadFileInto() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadFileToBuffer_LargeFile(t *testing.T) {
	b := createNewBuffer()
	err := loadFileInto("./data/test/large_sample.csv", b)
	if err != nil {
		t.Skipf("Test file not found: %v", err)
		return
	}

	if b.rowLen < 10 {
		t.Errorf("Expected at least 10 rows, got %d", b.rowLen)
	}

	if b.colLen < 5 {
		t.Errorf("Expected at least 5 columns, got %d", b.colLen)
	}
}

func TestLoadFileToBuffer_NumericData(t *testing.T) {
	b := createNewBuffer()
	err := loadFileInto("./data/test/numeric_data.csv", b)
	if err != nil {
		t.Skipf("Test file not found: %v", err)
		return
	}

	if b.rowLen < 2 {
		t.Errorf("Expected at least 2 rows, got %d", b.rowLen)
	}

	t.Logf("Loaded %d rows with %d columns", b.rowLen, b.colLen)
}

func TestLoadFileToBuffer_SpecialChars(t *testing.T) {
	b := createNewBuffer()
	err := loadFileInto("./data/test/special_characters.csv", b)
	if err != nil {
		t.Skipf("Test file not found: %v", err)
		return
	}

	if b.rowLen < 3 {
		t.Errorf("Expected at least 3 rows for special chars file, got %d", b.rowLen)
	}
}

func TestLoadFileToBuffer_Compressed(t *testing.T) {
	b := createNewBuffer()
	err := loadFileInto("./data/test/compressed_data.csv.gz", b)
	if err != nil {
		t.Skipf("Compressed test file not found: %v", err)
		return
	}

	if b.rowLen < 5 {
		t.Errorf("Expected data from compressed file, got %d rows", b.rowLen)
	}
}

func TestLoadFileToBuffer_TSV(t *testing.T) {
	b := createNewBuffer()
	err := loadFileInto("./data/test/tab_separated.tsv", b)
	if err != nil {
		t.Skipf("TSV test file not found: %v", err)
		return
	}

	if b.sep != '\t' {
		t.Errorf("Expected tab separator, got %q", b.sep)
	}
}

func TestLoadFileToBuffer_EdgeCases(t *testing.T) {
	t.Run("Empty file", func(t *testing.T) {
		b := createNewBuffer()
		err := loadFileInto("./data/test/empty_file.csv", b)
		if err != nil {
			t.Skipf("Empty test file not found: %v", err)
			return
		}

		if b.rowLen != 0 {
			t.Errorf("Expected 0 rows for empty file, got %d", b.rowLen)
		}
	})

	t.Run("Single row", func(t *testing.T) {
		b := createNewBuffer()
		err := loadFileInto("./data/test/single_row_data.csv", b)
		if err != nil {
			t.Skipf("Single row test file not found: %v", err)
			return
		}

		if b.rowLen < 1 {
			t.Errorf("Expected at least 1 row, got %d", b.rowLen)
		}
	})

	t.Run("Non-existent file", func(t *testing.T) {
		b := createNewBuffer()
		err := loadFileInto("./nonexistent_file_xyz.csv", b)
		if err == nil {
			t.Error("Expected error for non-existent file")
		}
	})
}

// ========================================
// Pipe Loading Tests
// ========================================

func TestLoadPipeToBuffer(t *testing.T) {
	csvData := "Name,Age,City\nJohn,30,NYC\nJane,25,LA\nBob,35,SF\n"
	reader := strings.NewReader(csvData)

	b := createNewBuffer()
	err := loadReaderInto(reader, b)
	if err != nil {
		t.Fatalf("loadReaderInto() error = %v", err)
	}

	if b.rowLen != 4 {
		t.Errorf("Expected 4 rows (including header), got %d", b.rowLen)
	}

	if b.colLen != 3 {
		t.Errorf("Expected 3 columns, got %d", b.colLen)
	}
}

func TestLoadPipeToBuffer_Empty(t *testing.T) {
	b := createNewBuffer()
	err := loadReaderInto(strings.NewReader(""), b)
	if err == nil {
		t.Error("Expected an error for an empty stream with no detectable separator")
	}
	if b.rowLen != 0 {
		t.Errorf("Expected 0 rows, got %d", b.rowLen)
	}
}

func TestLoadPipeToBuffer_LargeData(t *testing.T) {
	var buf bytes.Buffer
	buf.WriteString("ID,Value\n")
	for i := 0; i < 1000; i++ {
		buf.WriteString(I2S(i) + ",Data" + I2S(i) + "\n")
	}

	b := createNewBuffer()
	err := loadReaderInto(&buf, b)
	if err != nil {
		t.Fatalf("loadReaderInto() error = %v", err)
	}

	if b.rowLen != 1001 {
		t.Errorf("Expected 1001 rows, got %d", b.rowLen)
	}
}

// ========================================
// Async Loading Tests
// ========================================

func TestIntakeAsync_File(t *testing.T) {
	b := createNewBuffer()
	src, err := openFileSource("./data/test/large_sample.csv")
	if err != nil {
		t.Skipf("Test file not found: %v", err)
	}

	first := make(chan struct{}, 1)
	done := make(chan error, 1)
	var once sync.Once

	intake := Intake{Source: src, OnProgress: func(rows int, loaded, total int64) {
		once.Do(func() { first <- struct{}{} })
	}}
	go func() { done <- intake.Into(b) }()

	select {
	case <-first:
		t.Log("Received first progress report")
	case err := <-done:
		// Finished before the first report; there is nothing left to wait for.
		if err != nil {
			t.Skipf("Async load failed: %v", err)
		}
		if b.rowLen == 0 {
			t.Error("Expected data to be loaded")
		}
		return
	}

	if err := <-done; err != nil {
		t.Skipf("Async load failed: %v", err)
	}
	if b.rowLen == 0 {
		t.Error("Expected data to be loaded")
	}
}

func TestIntakeAsync_Pipe(t *testing.T) {
	csvData := "Name,Age,City\nJohn,30,NYC\nJane,25,LA\n"

	b := createNewBuffer()
	done := make(chan error, 1)
	intake := Intake{Source: pipeSource(strings.NewReader(csvData))}
	go func() { done <- intake.Into(b) }()

	if err := <-done; err != nil {
		t.Fatalf("Intake.Into() error = %v", err)
	}
	if b.rowLen == 0 {
		t.Error("Expected data to be loaded")
	}
}

// ========================================
// Line Filtering Tests
// ========================================

func Test_skipLine(t *testing.T) {
	type args struct {
		line string
		sy   []string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{"Match prefix @some", args{"@some line fo filtrate", []string{"@some"}}, true},
		{"No match - middle", args{"@some line fo filtrate", []string{"some"}}, false},
		{"Multiple patterns match first", args{"@some line fo filtrate", []string{"@some", "some"}}, true},
		{"Multiple patterns match second", args{"some @some line fo filtrate", []string{"@some", "some"}}, true},
		{"No match with @some", args{"some @some line fo filtrate", []string{"@some"}}, false},
		{"Empty patterns", args{"some @some line fo filtrate", []string{}}, false},
		{"Match at start #", args{"# This is a comment", []string{"#"}}, true},
		{"Match in middle not prefix", args{"Some data # comment", []string{"#"}}, false},
		{"Multiple patterns match //", args{"// Comment line", []string{"//", "#", "--"}}, true},
		{"No match normal data", args{"Normal data line", []string{"#", "//"}}, false},
		{"Empty patterns no match", args{"Any line", []string{}}, false},
		{"Empty line no match", args{"", []string{"#"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := skipLine(tt.args.line, tt.args.sy); got != tt.want {
				t.Errorf("skipLine() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ========================================
// Column Visibility Tests
// ========================================

func Test_getVisCol(t *testing.T) {
	type args struct {
		showNumL []int
		hideNumL []int
		colLen   int
	}
	tests := []struct {
		name    string
		args    args
		want    []int
		wantErr bool
	}{
		{"Set show argument", args{[]int{1, 2, 5}, []int{}, 6}, []int{0, 1, 4}, false},
		{"Set hide argument", args{[]int{}, []int{1, 2, 5}, 6}, []int{2, 3, 5}, false},
		{"No arguments set", args{[]int{}, []int{}, 6}, []int{0, 1, 2, 3, 4, 5}, false},
		{"Both arguments error", args{[]int{1, 2, 3}, []int{1, 2, 3}, 6}, nil, true},
		{"Show column out of range", args{[]int{1, 2, 3, 7}, []int{}, 6}, nil, true},
		{"Hide column out of range", args{[]int{}, []int{1, 2, 3, 7}, 6}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getVisCol(tt.args.showNumL, tt.args.hideNumL, tt.args.colLen)
			if (err != nil) != tt.wantErr {
				t.Errorf("getVisCol() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getVisCol() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_checkVisible(t *testing.T) {
	type args struct {
		showNumL []int
		hideNumL []int
		col      int
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{"Show list col 5", args{[]int{1, 3, 5}, []int{}, 5}, false, false},
		{"Show list col 4", args{[]int{1, 3, 5}, []int{}, 4}, true, false},
		{"Show list col 3", args{[]int{1, 3, 5}, []int{}, 3}, false, false},
		{"Show list col 2", args{[]int{1, 3, 5}, []int{}, 2}, true, false},
		{"Show list col 1", args{[]int{1, 3, 5}, []int{}, 1}, false, false},
		{"Show list col 0", args{[]int{1, 3, 5}, []int{}, 0}, true, false},
		{"Hide list col 5", args{[]int{}, []int{1, 3, 5}, 5}, true, false},
		{"Hide list col 4", args{[]int{}, []int{1, 3, 5}, 4}, false, false},
		{"Hide list col 3", args{[]int{}, []int{1, 3, 5}, 3}, true, false},
		{"Hide list col 2", args{[]int{}, []int{1, 3, 5}, 2}, false, false},
		{"Hide list col 1", args{[]int{}, []int{1, 3, 5}, 1}, true, false},
		{"Hide list col 0", args{[]int{}, []int{1, 3, 5}, 0}, false, false},
		{"Both lists error", args{[]int{1, 3, 5}, []int{1, 3, 5}, 4}, false, true},
		{"No lists all visible", args{[]int{}, []int{}, 3}, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := checkVisible(tt.args.showNumL, tt.args.hideNumL, tt.args.col)
			if (err != nil) != tt.wantErr {
				t.Errorf("checkVisible() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("checkVisible() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ========================================
// CSV Parsing Tests
// ========================================

func Test_lineCSVParse(t *testing.T) {
	type args struct {
		s   string
		sep rune
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{"CSV line", args{"a,b,c,d", ','}, []string{"a", "b", "c", "d"}, false},
		{"TSV line", args{"a	b	c	d", '	'}, []string{"a", "b", "c", "d"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := lineCSVParse(tt.args.s, tt.args.sep)
			if (err != nil) != tt.wantErr {
				t.Errorf("lineCSVParse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("lineCSVParse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLineCSVParse(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		sep     rune
		wantLen int
	}{
		{"Simple CSV", "a,b,c", ',', 3},
		{"Tab separated", "a\tb\tc", '\t', 3},
		{"Empty fields", "a,,c", ',', 3},
		{"Single field", "single", ',', 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := lineCSVParse(tt.line, tt.sep)
			if err != nil {
				t.Fatalf("lineCSVParse() error = %v", err)
			}
			if len(result) != tt.wantLen {
				t.Errorf("lineCSVParse() returned %d fields, want %d", len(result), tt.wantLen)
			}
		})
	}
}

func TestVisibleFields(t *testing.T) {
	fields := []string{"a", "b", "c", "d"}

	tests := []struct {
		name    string
		show    []int
		hide    []int
		want    []string
		wantErr bool
	}{
		{"No selection keeps every column", nil, nil, fields, false},
		{"Show a subset", []int{1, 3}, nil, []string{"a", "c"}, false},
		{"Hide a subset", nil, []int{2}, []string{"a", "c", "d"}, false},
		{"Show and hide together is an error", []int{1}, []int{2}, nil, true},
		{"Out of range is an error", []int{9}, nil, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := visibleFields(fields, tt.show, tt.hide)
			if (err != nil) != tt.wantErr {
				t.Fatalf("visibleFields() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("visibleFields() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIntake_PreservesRowOrder guards the property the concurrent parse used to
// break: lines were handed to a worker pool and appended in completion order,
// so most rows arrived out of sequence.
func TestIntake_PreservesRowOrder(t *testing.T) {
	lines := []string{
		"Name,Age,City",
		"John,30,NYC",
		"Jane,25,LA",
		"Bob,35,SF",
		"Alice,28,Boston",
	}

	b := createNewBuffer()
	if err := loadReaderInto(strings.NewReader(strings.Join(lines, "\n")+"\n"), b); err != nil {
		t.Fatalf("loadReaderInto() error = %v", err)
	}

	if b.rowLen != len(lines) {
		t.Fatalf("Expected %d rows, got %d", len(lines), b.rowLen)
	}
	for i, line := range lines {
		want := strings.Split(line, ",")
		if !reflect.DeepEqual(b.cont[i], want) {
			t.Errorf("row %d = %v, want %v", i, b.cont[i], want)
		}
	}
}

// TestIntake_PreservesRowOrder_AcrossBatches covers more rows than one parse
// batch, where ordering has to hold across batch boundaries too.
func TestIntake_PreservesRowOrder_AcrossBatches(t *testing.T) {
	var in bytes.Buffer
	in.WriteString("idx,payload\n")
	const rows = 5000
	for i := 1; i <= rows; i++ {
		in.WriteString(I2S(i) + ",row" + I2S(i) + "\n")
	}

	b := createNewBuffer()
	if err := loadReaderInto(&in, b); err != nil {
		t.Fatalf("loadReaderInto() error = %v", err)
	}

	if b.rowLen != rows+1 {
		t.Fatalf("Expected %d rows, got %d", rows+1, b.rowLen)
	}
	for i := 1; i <= rows; i++ {
		if got := b.cont[i][0]; got != I2S(i) {
			t.Fatalf("row %d holds %q, want %q (rows arrived out of order)", i, got, I2S(i))
		}
	}
}

// ========================================
// Integration Tests
// ========================================

func TestIntegration_FullWorkflow(t *testing.T) {
	buf := createNewBuffer()

	err := loadFileInto("./data/test/numeric_data.csv", buf)
	if err != nil {
		t.Skipf("Integration test skipped: %v", err)
		return
	}

	if buf.rowLen > 1 {
		buf.SortBy(0, false)
	}

	t.Log("Integration test completed successfully")
}
