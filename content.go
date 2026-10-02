package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// bufferContent presents the view's buffer to tview on demand: tview asks for
// the cells it is about to paint, and only those are built.
//
// The table used to be materialised in full, one tview.TableCell per cell of
// the file. That cost time and memory proportional to the whole file rather
// than to the window on screen — around 1.7KB per row, so a 48MB file needed
// roughly 2GB and a third of a second for every repaint — and it had to be
// rebuilt each time rows arrived during a load. Serving cells on demand makes
// both costs proportional to the visible area instead.
type bufferContent struct {
	tview.TableContentReadOnly
}

// GetRowCount returns the number of rows currently in the buffer, which grows
// while a load is in flight.
func (bufferContent) GetRowCount() int {
	rows, _ := view.Dims()
	return rows
}

// GetColumnCount returns the buffer's column count.
func (bufferContent) GetColumnCount() int {
	_, cols := view.Dims()
	return cols
}

// GetCell builds the cell at one position, styled for the header, the frozen
// column, any filter on the column and the current search match.
func (bufferContent) GetCell(row, column int) *tview.TableCell {
	b := view.Visible()

	b.mu.RLock()
	if row < 0 || row >= b.rowLen || column < 0 || column >= b.colLen {
		b.mu.RUnlock()
		return tview.NewTableCell("")
	}
	cellText := ""
	if column < len(b.cont[row]) {
		cellText = b.cont[row][column]
	}
	rowFreeze, colFreeze := b.rowFreeze, b.colFreeze
	b.mu.RUnlock()

	color := tcell.ColorWhite
	backgroundColor := tcell.ColorDefault
	attributes := tcell.AttrNone
	alignment := tview.AlignLeft

	// Check if this is a header row/column (frozen area)
	isHeaderRow := row < rowFreeze && args.Header != -1 && args.Header != 2
	isHeaderCol := column < colFreeze

	// Modern header styling with rich visual design
	if isHeaderRow {
		// Main header row: bold white text on gradient blue background
		color = tcell.ColorWhite
		backgroundColor = tcell.NewRGBColor(30, 60, 120) // Deep blue
		attributes = tcell.AttrBold | tcell.AttrUnderline
		alignment = tview.AlignCenter

		// Add filter indicator if this column has a filter applied
		if _, hasFilter := view.FilterAt(column); hasFilter {
			cellText = "🔎 " + cellText + " 🔎"
			backgroundColor = tcell.NewRGBColor(255, 100, 0) // Orange for a filtered column
		}
	} else if isHeaderCol {
		// Frozen column: gold color for row headers
		color = tcell.NewRGBColor(255, 215, 0) // Gold
		attributes = tcell.AttrBold
	}

	// Modern search match highlighting (overrides header styling)
	if isSearchMatch, isCurrentMatch := view.MatchAt(row, column); isSearchMatch {
		if isCurrentMatch {
			// Current match: vibrant cyan highlight
			backgroundColor = tcell.NewRGBColor(0, 180, 216)
			color = tcell.ColorBlack
			attributes = tcell.AttrBold
		} else {
			// Other matches: soft purple highlight
			backgroundColor = tcell.NewRGBColor(100, 100, 150)
			color = tcell.ColorWhite
			attributes = tcell.AttrNone
		}
	}

	// Determine max width for this column
	maxWidth := 0
	if width, isWrapped := view.ColumnWidth(column); isWrapped {
		maxWidth = width
		cellText = truncateText(cellText, maxWidth)
	}

	cell := tview.NewTableCell(cellText).
		SetTextColor(color).
		SetBackgroundColor(backgroundColor).
		SetAttributes(attributes).
		SetAlign(alignment).
		SetExpansion(1)

	if maxWidth > 0 {
		cell.SetMaxWidth(maxWidth)
	}
	return cell
}
