package main

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// The table's palette. Kept together so the whole surface can be retuned in
// one place rather than by hunting literals through the painting code.
var (
	cellTextColor   = tcell.NewRGBColor(220, 226, 235)
	headerTextColor = tcell.NewRGBColor(226, 238, 255)
	headerBackColor = tcell.NewRGBColor(28, 52, 94)
	stripeBackColor = tcell.NewRGBColor(24, 28, 36)
	rowLabelColor   = tcell.NewRGBColor(236, 206, 140)
	filterMarkColor = tcell.NewRGBColor(236, 148, 56)
	matchBackColor  = tcell.NewRGBColor(0, 170, 205)
	otherMatchColor = tcell.NewRGBColor(86, 92, 132)
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

	layout := view.Layout(column)

	color := cellTextColor
	backgroundColor := tcell.ColorDefault
	attributes := tcell.AttrNone
	alignment := tview.AlignLeft
	if layout.rightAlign {
		alignment = tview.AlignRight
	}

	// Check if this is a header row/column (frozen area)
	isHeaderRow := row < rowFreeze && args.Header != -1 && args.Header != 2
	isHeaderCol := column < colFreeze

	// Modern header styling with rich visual design
	switch {
	case isHeaderRow:
		// The header takes the column's own alignment, so a numeric heading
		// sits over its digits instead of floating in the middle of the field.
		color = headerTextColor
		backgroundColor = headerBackColor
		attributes = tcell.AttrBold

		if _, hasFilter := view.FilterAt(column); hasFilter {
			cellText = "▼ " + cellText
			backgroundColor = filterMarkColor
			color = tcell.ColorBlack
		}

	default:
		// Zebra striping: alternate data rows carry a slightly lifted
		// background, which is what makes a wide row easy to follow across.
		if (row-rowFreeze)%2 == 1 {
			backgroundColor = stripeBackColor
		}
		if isHeaderCol {
			// The frozen column reads as a row label: lifted, not shouting.
			color = rowLabelColor
		}
	}

	// Modern search match highlighting (overrides header styling)
	if isSearchMatch, isCurrentMatch := view.MatchAt(row, column); isSearchMatch {
		if isCurrentMatch {
			backgroundColor = matchBackColor
			color = tcell.ColorBlack
			attributes = tcell.AttrBold
		} else {
			backgroundColor = otherMatchColor
			color = tcell.ColorWhite
			attributes = tcell.AttrNone
		}
	}

	// The measured width governs, unless the user pinned one with 'W'.
	width := layout.width
	if pinned, isPinned := view.ColumnWidth(column); isPinned && pinned < width {
		width = pinned
	}

	// A space either side keeps values off the column separators. Without it
	// the tightened columns read as one run of characters.
	cellText = " " + truncateText(cellText, width) + " "

	return tview.NewTableCell(cellText).
		SetTextColor(color).
		SetBackgroundColor(backgroundColor).
		SetAttributes(attributes).
		SetAlign(alignment).
		SetExpansion(layout.expand).
		SetMaxWidth(width + 2)
}
