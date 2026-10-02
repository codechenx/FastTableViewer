package main

import (
	"errors"
	"sort"
	"strings"
	"sync"
)

// Buffer holds the table and owns every rule for reading or changing it
// safely. Callers do not need to know which operations are concurrent with a
// load: every exported operation takes the lock it needs, so the loader's
// workers and the UI goroutine can both hold a *Buffer.
type Buffer struct {
	sep          rune         // Column separator character
	cont         [][]string   // Table content (rows x columns)
	colType      []ColumnType // Column data types
	rowLen       int          // Number of rows
	colLen       int          // Number of columns
	rowFreeze    int          // Number of frozen header rows (0 or 1)
	colFreeze    int          // Number of frozen columns (0 or 1)
	selectedCell [][]int      // Selected cell coordinates
	mu           sync.RWMutex // Guards every field above
}

const (
	// Pre-allocated capacity for rows (optimized for large files)
	defaultRowCapacity = 10000

	// detectSampleSize is how many cells autoDetectColumnType inspects before
	// deciding a column's type.
	detectSampleSize = 100

	// detectThreshold is the share of non-missing values that must satisfy a
	// type for the column to be given that type.
	detectThreshold = 0.90
)

// createNewBuffer initializes and returns a new empty Buffer
func createNewBuffer() *Buffer {
	return &Buffer{
		sep:          0,
		cont:         [][]string{},
		colType:      []ColumnType{},
		rowLen:       0,
		colLen:       0,
		rowFreeze:    1,
		colFreeze:    1,
		selectedCell: [][]int{},
	}
}

// createNewBufferWithData creates a Buffer from existing data.
func createNewBufferWithData(ss [][]string, strict bool) (*Buffer, error) {
	buf := createNewBuffer()
	for _, s := range ss {
		if err := buf.contAppendSli(s, strict); err != nil {
			return nil, err
		}
	}
	return buf, nil
}

// contAppendSli appends a row to the buffer
// strict: if true, enforces consistent column count
func (b *Buffer) contAppendSli(s []string, strict bool) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Initialize on first row
	if b.rowLen == 0 {
		b.colLen = len(s)
		b.colType = make([]ColumnType, b.colLen+1)
		// Pre-allocate capacity to reduce reallocations
		if cap(b.cont) == 0 {
			b.cont = make([][]string, 0, defaultRowCapacity)
		}
	}

	// Strict mode: enforce column count
	if strict && len(s) != b.colLen {
		return errors.New("Row " + I2S(b.rowLen+b.rowFreeze) + " lacks some columns")
	}

	b.cont = append(b.cont, s)

	// Adjust column count if needed
	if b.colLen != len(s) {
		b.resizeColUnsafe(len(s))
	}
	b.rowLen++

	return nil
}

// resizeColUnsafe adjusts the number of columns (must be called with lock held)
// Fills missing columns with "NaN"
func (b *Buffer) resizeColUnsafe(n int) {
	if n <= 0 {
		return
	}

	lackLen := b.colLen - n
	if lackLen < 0 {
		lackLen = n - b.colLen
		b.colLen = n
	}

	// Fill missing columns with NaN
	for ii := range b.cont {
		for m := 0; m < lackLen; m++ {
			b.cont[ii] = append(b.cont[ii], "NaN")
		}
	}

	// Keep the type slice as wide as the table, so a later, wider row can
	// never put a column index out of range.
	for len(b.colType) < b.colLen+1 {
		b.colType = append(b.colType, colTypeStr)
	}
}

// resizeCol adjusts the number of columns (thread-safe)
func (b *Buffer) resizeCol(n int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.resizeColUnsafe(n)
}

// SortBy orders the buffer's data rows by one column, using that column's own
// type to decide what order means. Callers pick a column and a direction; the
// buffer picks the comparison, so nothing outside has to switch on type.
func (b *Buffer) SortBy(colIndex int, desc bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if colIndex < 0 || colIndex >= b.colLen {
		return
	}

	rows := b.cont
	if b.rowFreeze > 0 && len(rows) >= b.rowFreeze {
		rows = rows[b.rowFreeze:]
	}
	if len(rows) < 2 {
		return
	}

	cellAt := func(row []string) string {
		if colIndex < len(row) {
			return row[colIndex]
		}
		return ""
	}

	colType := colTypeStr
	if colIndex < len(b.colType) {
		colType = b.colType[colIndex]
	}
	parse := columnTypes[colType].parse

	// A type with no parser orders lexically on the raw cell.
	if parse == nil {
		sort.SliceStable(rows, func(i, j int) bool {
			if desc {
				return cellAt(rows[i]) > cellAt(rows[j])
			}
			return cellAt(rows[i]) < cellAt(rows[j])
		})
		return
	}

	// Parse each cell once rather than on every comparison.
	type keyedRow struct {
		row []string
		key float64
	}
	pairs := make([]keyedRow, len(rows))
	for i, row := range rows {
		key, _ := parse(cellAt(row))
		pairs[i] = keyedRow{row: row, key: key}
	}

	sort.SliceStable(pairs, func(i, j int) bool {
		if desc {
			return pairs[i].key > pairs[j].key
		}
		return pairs[i].key < pairs[j].key
	})

	for i := range pairs {
		rows[i] = pairs[i].row
	}
}

// getCol returns the ith column data as a string slice
func (b *Buffer) getCol(i int) []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	result := make([]string, b.rowLen)
	for rowI := 0; rowI < b.rowLen; rowI++ {
		if i < len(b.cont[rowI]) {
			result[rowI] = b.cont[rowI][i]
		}
	}
	return result
}

// setColType sets the ith column's data type.
func (b *Buffer) setColType(i int, t ColumnType) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if i < 0 || i >= len(b.colType) {
		return
	}
	b.colType[i] = t
}

// getColType returns the ith column's data type.
func (b *Buffer) getColType(i int) ColumnType {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if i < 0 || i >= len(b.colType) {
		return colTypeStr
	}
	return b.colType[i]
}

// autoDetectColumnType decides a column's type by sampling its values. It asks
// each candidate type's own parser, so detection can never disagree with the
// ordering that parser produces.
func (b *Buffer) autoDetectColumnType(colIndex int) ColumnType {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if colIndex < 0 || colIndex >= b.colLen {
		return colTypeStr
	}

	startRow := b.rowFreeze
	endRow := b.rowLen
	sampleRows := []int{}

	if endRow-startRow > detectSampleSize {
		// Sample first 50 rows
		for i := startRow; i < startRow+50 && i < endRow; i++ {
			sampleRows = append(sampleRows, i)
		}
		// Sample middle 25 rows
		midPoint := (startRow + endRow) / 2
		for i := midPoint; i < midPoint+25 && i < endRow; i++ {
			sampleRows = append(sampleRows, i)
		}
		// Sample last 25 rows
		for i := endRow - 25; i < endRow; i++ {
			if i > startRow {
				sampleRows = append(sampleRows, i)
			}
		}
	} else {
		// For small datasets, check all rows
		for i := startRow; i < endRow; i++ {
			sampleRows = append(sampleRows, i)
		}
	}

	counts := make(map[ColumnType]int, len(detectionOrder))
	totalCount := 0

	for _, rowIdx := range sampleRows {
		if rowIdx >= b.rowLen || colIndex >= len(b.cont[rowIdx]) {
			continue
		}

		value := strings.TrimSpace(b.cont[rowIdx][colIndex])
		if isMissing(value) {
			continue
		}
		totalCount++

		// detectionOrder is most specific first, so the first type that
		// accepts the value is the one it counts for.
		for _, candidate := range detectionOrder {
			if _, ok := columnTypes[candidate].parse(value); ok {
				counts[candidate]++
				break
			}
		}
	}

	if totalCount == 0 {
		return colTypeStr
	}

	threshold := float64(totalCount) * detectThreshold
	for _, candidate := range detectionOrder {
		if float64(counts[candidate]) >= threshold {
			return candidate
		}
	}

	return colTypeStr
}

// detectAllColumnTypes automatically detects types for all columns.
func (b *Buffer) detectAllColumnTypes() {
	b.mu.RLock()
	colLen := b.colLen
	b.mu.RUnlock()

	for i := 0; i < colLen; i++ {
		b.setColType(i, b.autoDetectColumnType(i))
	}
}

// selectBySearch records every cell holding exactly s.
func (b *Buffer) selectBySearch(s string) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for ii, i := range b.cont {
		for ji, j := range i {
			if s == j {
				b.selectedCell = append(b.selectedCell, []int{ii, ji})
			}
		}
	}
}

// filterByColumn returns a new buffer holding the rows whose value in one
// column satisfies options. The matcher is compiled once for the whole scan.
func (b *Buffer) filterByColumn(colIndex int, options FilterOptions) *Buffer {
	b.mu.RLock()
	defer b.mu.RUnlock()

	filtered := createNewBuffer()
	filtered.sep = b.sep
	filtered.colLen = b.colLen
	filtered.rowFreeze = b.rowFreeze
	filtered.colFreeze = b.colFreeze
	filtered.colType = make([]ColumnType, len(b.colType))
	copy(filtered.colType, b.colType)

	// Add header row if present
	if b.rowFreeze > 0 && b.rowLen > 0 {
		filtered.cont = append(filtered.cont, b.cont[0])
		filtered.rowLen = 1
	}

	colType := colTypeStr
	if colIndex >= 0 && colIndex < len(b.colType) {
		colType = b.colType[colIndex]
	}

	matcher, err := CompileMatcher(options, colType)
	if err != nil {
		// An unusable pattern matches nothing, leaving just the header.
		return filtered
	}

	for i := b.rowFreeze; i < b.rowLen; i++ {
		if colIndex < 0 || colIndex >= len(b.cont[i]) {
			continue
		}
		if matcher.MatchCell(b.cont[i][colIndex]) {
			filtered.cont = append(filtered.cont, b.cont[i])
			filtered.rowLen++
		}
	}

	return filtered
}
