package main

import "sort"

// ViewState owns everything the viewer knows about what is on screen: which
// buffer is visible, which filters and search are active, which columns are
// width-limited, and where the cursor is.
//
// Every change goes through one of its methods and every method leaves the
// derived state consistent, so the rendered table, the match highlighting and
// the footer cannot disagree about which buffer is current. That disagreement
// was previously possible because drawUI shadowed the package-level buffer:
// reassigning it inside the closure left the footer helpers reading the old one.
//
// It is reached only from the tview event loop (key handlers and
// QueueUpdateDraw callbacks), never from the loading goroutine, which touches
// the Buffer directly and is safe because the Buffer guards itself.
type ViewState struct {
	original *Buffer // as loaded, never filtered
	visible  *Buffer // what the table renders

	filters map[int]FilterOptions

	search    MatchSpec
	results   []SearchResult
	resultSet map[[2]int]bool
	resultAt  int // index into results, -1 when there is no current match

	wrapped map[int]int // column -> max width

	cursorCol int
}

// defaultWrapWidth is the width a limited column is held to.
const defaultWrapWidth = 50

// NewViewState returns a view over b with nothing filtered and nothing found.
func NewViewState(b *Buffer) *ViewState {
	return &ViewState{
		original:  b,
		visible:   b,
		filters:   map[int]FilterOptions{},
		results:   []SearchResult{},
		resultSet: map[[2]int]bool{},
		resultAt:  -1,
		wrapped:   map[int]int{},
	}
}

// Visible returns the buffer the table should render.
func (v *ViewState) Visible() *Buffer { return v.visible }

// Original returns the buffer as loaded, before any filtering.
func (v *ViewState) Original() *Buffer { return v.original }

// Filtered reports whether any filter is active.
func (v *ViewState) Filtered() bool { return len(v.filters) > 0 }

// FilterCount returns how many columns are filtered.
func (v *ViewState) FilterCount() int { return len(v.filters) }

// FilterAt returns the filter on one column, if there is one.
func (v *ViewState) FilterAt(col int) (FilterOptions, bool) {
	opts, ok := v.filters[col]
	return opts, ok
}

// DataRows returns the number of rows excluding any frozen header.
func (v *ViewState) DataRows() int {
	v.visible.mu.RLock()
	defer v.visible.mu.RUnlock()
	return v.visible.rowLen - v.visible.rowFreeze
}

// ColumnName returns the header text for a column, falling back to its index.
func (v *ViewState) ColumnName(col int) string {
	v.visible.mu.RLock()
	defer v.visible.mu.RUnlock()

	if v.visible.rowFreeze > 0 && len(v.visible.cont) > 0 && col < len(v.visible.cont[0]) {
		return v.visible.cont[0][col]
	}
	return "Column " + I2S(col)
}

// ApplyFilter adds or replaces the filter on one column. It reports the data
// rows that survive and whether the filter was kept: a filter matching nothing
// is discarded, leaving the previous view in place.
func (v *ViewState) ApplyFilter(col int, opts FilterOptions) (rows int, ok bool) {
	previous, had := v.filters[col]
	v.filters[col] = opts

	candidate := v.applyAllFilters()
	candidate.mu.RLock()
	surviving := candidate.rowLen - candidate.rowFreeze
	candidate.mu.RUnlock()

	if surviving <= 0 {
		// Put the filter set back the way it was.
		if had {
			v.filters[col] = previous
		} else {
			delete(v.filters, col)
		}
		return 0, false
	}

	v.setVisible(candidate)
	return surviving, true
}

// ClearFilter removes one column's filter. It reports whether there was one.
func (v *ViewState) ClearFilter(col int) bool {
	if _, ok := v.filters[col]; !ok {
		return false
	}
	delete(v.filters, col)
	v.setVisible(v.applyAllFilters())
	return true
}

// ClearAllFilters removes every filter.
func (v *ViewState) ClearAllFilters() {
	if len(v.filters) == 0 {
		return
	}
	v.filters = map[int]FilterOptions{}
	v.setVisible(v.original)
}

// applyAllFilters rebuilds the filtered buffer from the original. Columns are
// applied in index order so the result does not depend on map iteration.
func (v *ViewState) applyAllFilters() *Buffer {
	if len(v.filters) == 0 {
		return v.original
	}

	cols := make([]int, 0, len(v.filters))
	for col := range v.filters {
		cols = append(cols, col)
	}
	sort.Ints(cols)

	out := v.original
	for _, col := range cols {
		out = out.filterByColumn(col, v.filters[col])
	}
	return out
}

// setVisible swaps the rendered buffer and brings derived state with it. A
// search is re-run against the new buffer, because match coordinates taken
// from the old one would highlight the wrong cells.
func (v *ViewState) setVisible(b *Buffer) {
	v.visible = b
	if v.search.Query != "" {
		v.runSearch()
	}
}

// Search records a new search and returns the number of matches.
func (v *ViewState) Search(spec MatchSpec) int {
	v.search = spec
	v.runSearch()
	return len(v.results)
}

// runSearch recomputes the match set against the visible buffer.
func (v *ViewState) runSearch() {
	v.results = performSearch(v.visible, v.search)
	v.resultSet = make(map[[2]int]bool, len(v.results))
	for _, r := range v.results {
		v.resultSet[[2]int{r.Row, r.Col}] = true
	}
	if len(v.results) > 0 {
		v.resultAt = 0
	} else {
		v.resultAt = -1
	}
}

// ClearSearch forgets the current search and its matches.
func (v *ViewState) ClearSearch() {
	v.search = MatchSpec{}
	v.results = []SearchResult{}
	v.resultSet = map[[2]int]bool{}
	v.resultAt = -1
}

// Searching reports whether a search is active.
func (v *ViewState) Searching() bool { return v.search.Query != "" }

// SearchSpec returns the active search.
func (v *ViewState) SearchSpec() MatchSpec { return v.search }

// MatchCount returns how many cells match.
func (v *ViewState) MatchCount() int { return len(v.results) }

// MatchIndex returns the 1-based position of the current match, or 0 if none.
func (v *ViewState) MatchIndex() int {
	if v.resultAt < 0 {
		return 0
	}
	return v.resultAt + 1
}

// CurrentMatch returns the current match, if there is one.
func (v *ViewState) CurrentMatch() (SearchResult, bool) {
	if v.resultAt < 0 || v.resultAt >= len(v.results) {
		return SearchResult{}, false
	}
	return v.results[v.resultAt], true
}

// NextMatch advances to the following match, wrapping around.
func (v *ViewState) NextMatch() (SearchResult, bool) {
	if len(v.results) == 0 || v.resultAt < 0 {
		return SearchResult{}, false
	}
	v.resultAt = (v.resultAt + 1) % len(v.results)
	return v.results[v.resultAt], true
}

// PrevMatch steps back to the preceding match, wrapping around.
func (v *ViewState) PrevMatch() (SearchResult, bool) {
	if len(v.results) == 0 || v.resultAt < 0 {
		return SearchResult{}, false
	}
	v.resultAt--
	if v.resultAt < 0 {
		v.resultAt = len(v.results) - 1
	}
	return v.results[v.resultAt], true
}

// MatchAt reports whether a cell matches the search, and whether it is the
// current match. One map lookup replaces a scan of every result per cell.
func (v *ViewState) MatchAt(row, col int) (match, current bool) {
	if !v.resultSet[[2]int{row, col}] {
		return false, false
	}
	if cur, ok := v.CurrentMatch(); ok && cur.Row == row && cur.Col == col {
		return true, true
	}
	return true, false
}

// ColType returns a column's type.
func (v *ViewState) ColType(col int) ColumnType { return v.visible.getColType(col) }

// CycleColType advances a column's type and returns the new one. The type is
// set on both buffers so it survives a change of filter.
func (v *ViewState) CycleColType(col int) ColumnType {
	next := nextColumnType(v.visible.getColType(col))
	v.visible.setColType(col, next)
	if v.original != v.visible {
		v.original.setColType(col, next)
	}
	return next
}

// SortBy orders the visible buffer by one column.
func (v *ViewState) SortBy(col int, desc bool) { v.visible.SortBy(col, desc) }

// ColumnWidth returns a column's width limit, and whether it has one.
func (v *ViewState) ColumnWidth(col int) (int, bool) {
	width, ok := v.wrapped[col]
	return width, ok
}

// ToggleWrap turns a column's width limit on or off. It returns the width in
// force and whether the column is now limited.
func (v *ViewState) ToggleWrap(col int) (int, bool) {
	if _, ok := v.wrapped[col]; ok {
		delete(v.wrapped, col)
		return 0, false
	}
	v.wrapped[col] = defaultWrapWidth
	return defaultWrapWidth, true
}

// DetectWideColumns limits any column whose sampled content exceeds threshold,
// leaving columns already set by hand alone.
func (v *ViewState) DetectWideColumns(sampleSize, threshold int) {
	b := v.visible

	b.mu.RLock()
	maxSample := sampleSize
	if b.rowLen < maxSample {
		maxSample = b.rowLen
	}
	startRow := b.rowFreeze

	maxLengths := make([]int, b.colLen)
	for r := startRow; r < maxSample; r++ {
		for c := 0; c < b.colLen && c < len(b.cont[r]); c++ {
			if cellLen := len(b.cont[r][c]); cellLen > maxLengths[c] {
				maxLengths[c] = cellLen
			}
		}
	}
	b.mu.RUnlock()

	for c, length := range maxLengths {
		if length <= threshold {
			continue
		}
		if _, set := v.wrapped[c]; !set {
			v.wrapped[c] = defaultWrapWidth
		}
	}
}

// SetCursorColumn records where the cursor is.
func (v *ViewState) SetCursorColumn(col int) { v.cursorCol = col }

// CursorColumn returns the cursor's column.
func (v *ViewState) CursorColumn() int { return v.cursorCol }

// Dims returns the visible buffer's row and column counts.
func (v *ViewState) Dims() (rows, cols int) {
	v.visible.mu.RLock()
	defer v.visible.mu.RUnlock()
	return v.visible.rowLen, v.visible.colLen
}

// Freeze returns the frozen row and column counts of the visible buffer.
func (v *ViewState) Freeze() (rows, cols int) {
	v.visible.mu.RLock()
	defer v.visible.mu.RUnlock()
	return v.visible.rowFreeze, v.visible.colFreeze
}
