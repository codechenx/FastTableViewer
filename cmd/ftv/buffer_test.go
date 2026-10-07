package main

import (
	"reflect"
	"testing"
)

// ========================================
// Buffer Creation Tests
// ========================================

func Test_createNewBuffer(t *testing.T) {
	tests := []struct {
		name string
	}{
		{"Create empty buffer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := createNewBuffer()
			if buf == nil {
				t.Error("createNewBuffer() returned nil")
				return
			}
			if buf.rowLen != 0 || buf.colLen != 0 {
				t.Error("New buffer should be empty")
			}
		})
	}
}

func Test_createNewBufferWithData(t *testing.T) {
	type args struct {
		ss     [][]string
		strict bool
	}
	wantBuffer := createNewBuffer()
	wantBuffer.colLen = 3
	wantBuffer.rowLen = 4
	wantBuffer.cont = [][]string{{"a", "b", "c"}, {"1", "2", "3"}, {"4", "5", "6"}, {"7", "8", "9"}}
	wantBuffer.colType = []ColumnType{0, 0, 0, 0}
	tests := []struct {
		name    string
		args    args
		want    *Buffer
		wantErr bool
	}{
		{"Valid data strict mode", args{ss: [][]string{{"a", "b", "c"}, {"1", "2", "3"}, {"4", "5", "6"}, {"7", "8", "9"}}, strict: true}, wantBuffer, false},
		{"Inconsistent columns strict mode", args{ss: [][]string{{"a", "b", "c"}, {"1", "2", "3"}, {"4", "5"}, {"7", "8", "9"}}, strict: true}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := createNewBufferWithData(tt.args.ss, tt.args.strict)
			if (err != nil) != tt.wantErr {
				t.Errorf("createNewBufferWithData() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("createNewBufferWithData() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ========================================
// Buffer Data Manipulation Tests
// ========================================

func TestBuffer_contAppendSli(t *testing.T) {
	type args struct {
		s      []string
		strict bool
	}
	b := createNewBuffer()
	b.colLen = 2
	b.rowLen = 1
	b.cont = [][]string{{"a", "b"}}
	tests := []struct {
		name    string
		b       *Buffer
		args    args
		wantErr bool
	}{
		{"Matching columns strict", b, args{s: []string{"a", "1"}, strict: true}, false},
		{"Extra columns strict", b, args{s: []string{"a", "1", "3"}, strict: true}, true},
		{"Extra columns non-strict", b, args{s: []string{"a", "1", "2"}, strict: false}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.b.contAppendSli(tt.args.s, tt.args.strict); (err != nil) != tt.wantErr {
				t.Errorf("Buffer.contAppendSli() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuffer_ResizeCol(t *testing.T) {
	b := createNewBuffer()
	_ = b.contAppendSli([]string{"A", "B"}, false)
	_ = b.contAppendSli([]string{"1", "2"}, false)

	b.resizeCol(4)

	if b.colLen != 4 {
		t.Errorf("resizeCol(4) set colLen = %d, want 4", b.colLen)
	}

	for _, row := range b.cont {
		if len(row) != 4 {
			t.Errorf("Row length = %d, want 4", len(row))
		}
	}
}

// ========================================
// Buffer Sorting Tests
// ========================================

func TestBuffer_sortByStr(t *testing.T) {
	type args struct {
		colIndex int
		rev      bool
	}
	testBuffer, _ := createNewBufferWithData([][]string{{"a", "b", "c"}, {"2", "2", "3"}, {"4", "5", "6"}, {"10", "8", "9"}}, true)
	wantBuffer, _ := createNewBufferWithData([][]string{{"a", "b", "c"}, {"10", "8", "9"}, {"2", "2", "3"}, {"4", "5", "6"}}, true)
	tests := []struct {
		name string
		b    *Buffer
		args args
		want *Buffer
	}{
		{"String sort ascending", testBuffer, args{colIndex: 0, rev: false}, wantBuffer},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.b.SortBy(tt.args.colIndex, tt.args.rev)
			if !reflect.DeepEqual(tt.b, tt.want) {
				t.Errorf("Buffer_sortByStr() = %v, want %v", tt.b, tt.want)
			}
		})
	}
}

func TestBuffer_SortByStr(t *testing.T) {
	b := createNewBuffer()
	b.rowFreeze = 1
	_ = b.contAppendSli([]string{"Name"}, false)
	_ = b.contAppendSli([]string{"Charlie"}, false)
	_ = b.contAppendSli([]string{"Alice"}, false)
	_ = b.contAppendSli([]string{"Bob"}, false)

	b.SortBy(0, false)
	if b.cont[1][0] != "Alice" {
		t.Errorf("After ascending sort, first data row = %s, want Alice", b.cont[1][0])
	}

	b.SortBy(0, true)
	if b.cont[1][0] != "Charlie" {
		t.Errorf("After descending sort, first data row = %s, want Charlie", b.cont[1][0])
	}
}

func TestBuffer_sortByNum(t *testing.T) {
	type args struct {
		colIndex int
		rev      bool
	}
	testBuffer, _ := createNewBufferWithData([][]string{{"a", "b", "c"}, {"5", "2", "3"}, {"4", "5", "6"}, {"10", "8", "9"}}, true)
	wantBuffer, _ := createNewBufferWithData([][]string{{"a", "b", "c"}, {"4", "5", "6"}, {"5", "2", "3"}, {"10", "8", "9"}}, true)
	testBuffer.setColType(0, colTypeFloat)
	wantBuffer.setColType(0, colTypeFloat)
	tests := []struct {
		name string
		b    *Buffer
		args args
		want *Buffer
	}{
		{"Number sort ascending", testBuffer, args{colIndex: 0, rev: false}, wantBuffer},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.b.SortBy(tt.args.colIndex, tt.args.rev)
			if !reflect.DeepEqual(tt.b, tt.want) {
				t.Errorf("Buffer_sortByNum() = %v, want %v", tt.b, tt.want)
			}
		})
	}
}

func TestBuffer_SortByNum(t *testing.T) {
	b := createNewBuffer()
	b.rowFreeze = 1
	_ = b.contAppendSli([]string{"Score"}, false)
	_ = b.contAppendSli([]string{"85.5"}, false)
	_ = b.contAppendSli([]string{"92.3"}, false)
	_ = b.contAppendSli([]string{"78.9"}, false)
	b.setColType(0, colTypeFloat)

	b.SortBy(0, false)
	if b.cont[1][0] != "78.9" {
		t.Errorf("After ascending sort, first data row = %s, want 78.9", b.cont[1][0])
	}

	b.SortBy(0, true)
	if b.cont[1][0] != "92.3" {
		t.Errorf("After descending sort, first data row = %s, want 92.3", b.cont[1][0])
	}
}

// ========================================
// Buffer Transformation Tests
// ========================================

// ========================================
// Buffer Query Tests
// ========================================

func TestBuffer_getCol(t *testing.T) {
	type args struct {
		i int
	}
	testBuffer, _ := createNewBufferWithData([][]string{{"a", "b", "c"}, {"1", "2", "3"}, {"4", "5", "6"}}, true)
	tests := []struct {
		name string
		b    *Buffer
		args args
		want []string
	}{
		{"Get first column", testBuffer, args{i: 0}, []string{"a", "1", "4"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.b.getCol(tt.args.i); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Buffer.getCol() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuffer_GetCol(t *testing.T) {
	b := createNewBuffer()
	_ = b.contAppendSli([]string{"A", "B", "C"}, false)
	_ = b.contAppendSli([]string{"1", "2", "3"}, false)
	_ = b.contAppendSli([]string{"X", "Y", "Z"}, false)

	col := b.getCol(1)
	expected := []string{"B", "2", "Y"}

	if len(col) != len(expected) {
		t.Errorf("getCol() returned %d elements, want %d", len(col), len(expected))
	}

	for i, val := range col {
		if val != expected[i] {
			t.Errorf("getCol()[%d] = %s, want %s", i, val, expected[i])
		}
	}
}

func TestBuffer_GetColType(t *testing.T) {
	b := createNewBuffer()
	b.colType = []ColumnType{colTypeStr, colTypeFloat, colTypeStr}

	if b.getColType(0) != colTypeStr {
		t.Error("Expected colTypeStr for column 0")
	}
	if b.getColType(1) != colTypeFloat {
		t.Error("Expected colTypeFloat for column 1")
	}
}

func TestBuffer_selectBySearch(t *testing.T) {
	type args struct {
		s string
	}
	testBuffer, _ := createNewBufferWithData([][]string{{"a", "b", "c"}, {"1", "2", "3"}, {"4", "1", "6"}}, true)
	tests := []struct {
		name string
		b    *Buffer
		args args
		want [][]int
	}{
		{"Search for '1'", testBuffer, args{s: "1"}, [][]int{{1, 0}, {2, 1}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.b.selectBySearch(tt.args.s)
			if got := tt.b.selectedCell; !reflect.DeepEqual(got, tt.want) {
				t.Errorf("selectBySearch() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ========================================
// Buffer Concurrency Tests
// ========================================

func TestBuffer_ConcurrentAccess(t *testing.T) {
	b := createNewBuffer()
	_ = b.contAppendSli([]string{"A", "B", "C"}, false)

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			_ = b.getCol(0)
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

// ========================================
// Buffer Edge Case Tests
// ========================================

func TestBuffer_EdgeCases(t *testing.T) {
	t.Run("Empty buffer check", func(t *testing.T) {
		b := createNewBuffer()
		if b.rowLen != 0 || b.colLen != 0 {
			t.Error("New buffer should be empty")
		}
	})

	t.Run("Single cell", func(t *testing.T) {
		b := createNewBuffer()
		_ = b.contAppendSli([]string{"X"}, false)
		if b.cont[0][0] != "X" {
			t.Error("Single cell failed")
		}
	})

	t.Run("Strict mode violation", func(t *testing.T) {
		b := createNewBuffer()
		_ = b.contAppendSli([]string{"A", "B"}, false)
		err := b.contAppendSli([]string{"1", "2", "3"}, true)
		if err == nil {
			t.Error("Expected error for mismatched columns in strict mode")
		}
	})
}

// ========================================
// Additional Buffer Tests for Coverage
// ========================================

func TestBuffer_LargeDataset(t *testing.T) {
	b := createNewBuffer()

	// Add header
	_ = b.contAppendSli([]string{"ID", "Name", "Value"}, false)

	// Add many rows
	for i := 0; i < 1000; i++ {
		_ = b.contAppendSli([]string{I2S(i), "Name" + I2S(i), I2S(i * 10)}, false)
	}

	if b.rowLen != 1001 {
		t.Errorf("Expected 1001 rows, got %d", b.rowLen)
	}
}

func TestBuffer_SetAndGetColType(t *testing.T) {
	b := createNewBuffer()
	_ = b.contAppendSli([]string{"A", "B", "C"}, false)

	b.setColType(0, colTypeFloat)
	if b.getColType(0) != colTypeFloat {
		t.Error("setColType/getColType failed")
	}

	b.setColType(0, colTypeStr)
	if b.getColType(0) != colTypeStr {
		t.Error("setColType/getColType failed for string")
	}
}

func TestBuffer_ResizeColMultipleTimes(t *testing.T) {
	b := createNewBuffer()
	_ = b.contAppendSli([]string{"A", "B"}, false)

	b.resizeCol(5)
	if b.colLen != 5 {
		t.Errorf("Expected colLen=5, got %d", b.colLen)
	}

	// resizeCol doesn't shrink columns, only extends them
	// So after resize to 3, it should stay at 5
	b.resizeCol(3)
	if b.colLen < 3 {
		t.Errorf("Expected colLen >= 3, got %d", b.colLen)
	}
}
