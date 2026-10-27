package main

import (
	"errors"
	"fmt"
	"slices"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ConfigureColumns applies CLI choices without discarding input fields.
func (v *ViewState) ConfigureColumns(show, hide []int) error {
	if err := checkColumnSelection(show, hide); err != nil {
		return err
	}
	_, count := v.Dims()
	v.initialColumns = slices.Clone(show)
	v.hiddenColumns = make(map[int]bool, count)
	for column := 0; column < count; column++ {
		v.hiddenColumns[column] = len(show) > 0 && !slices.Contains(show, column+1)
	}
	for _, column := range hide {
		v.hiddenColumns[column-1] = true
	}
	v.knownColumns = count
	v.columns = nil
	return nil
}

func checkColumnSelection(show, hide []int) error {
	if len(show) > 0 && len(hide) > 0 {
		return errors.New("you can only set visible column or hidden column")
	}
	for _, columns := range [][]int{show, hide} {
		for _, column := range columns {
			if column <= 0 {
				return fmt.Errorf("column number %d does not exist", column)
			}
		}
	}
	return nil
}

// A requested column may arrive in a later streamed row, so check bounds at EOF.
func validateLoadedColumns(b *Buffer) error {
	b.mu.RLock()
	count := b.colLen
	b.mu.RUnlock()
	if count == 0 {
		return nil
	}
	columns, err := getVisCol(args.ShowNum, args.HideNum, count)
	if err != nil {
		return err
	}
	if len(columns) == 0 {
		return errors.New("at least one column must remain visible")
	}
	return nil
}

// VisibleColumns returns original column indices in display order.
func (v *ViewState) VisibleColumns() []int {
	_, count := v.Dims()
	if count == v.knownColumns && v.columns != nil {
		return v.columns
	}
	for column := v.knownColumns; column < count; column++ {
		if len(v.initialColumns) > 0 {
			v.hiddenColumns[column] = !slices.Contains(v.initialColumns, column+1)
		}
	}
	v.columns = make([]int, 0, count)
	for column := 0; column < count; column++ {
		if !v.hiddenColumns[column] {
			v.columns = append(v.columns, column)
		}
	}
	v.knownColumns = count
	return v.columns
}

func (v *ViewState) SourceColumn(display int) int {
	columns := v.VisibleColumns()
	if display < 0 || display >= len(columns) {
		return -1
	}
	return columns[display]
}

func (v *ViewState) DisplayColumn(source int) int {
	return slices.Index(v.VisibleColumns(), source)
}

// SetColumnVisibility applies a picker snapshot. Columns loaded later keep their policy.
func (v *ViewState) SetColumnVisibility(shown []bool) error {
	v.VisibleColumns()
	if len(shown) > v.knownColumns {
		return errors.New("column selection exceeds the input width")
	}
	remaining := 0
	for column := 0; column < v.knownColumns; column++ {
		visible := !v.hiddenColumns[column]
		if column < len(shown) {
			visible = shown[column]
		}
		if visible {
			remaining++
		}
	}
	if remaining == 0 {
		return errors.New("at least one column must remain visible")
	}
	for column, visible := range shown {
		v.hiddenColumns[column] = !visible
	}
	v.columns = nil
	if v.Searching() {
		v.runSearch()
	}
	return nil
}

func applyColumnVisibility(shown []bool) bool {
	row, display := bufferTable.GetSelection()
	source := view.SourceColumn(display)
	if err := view.SetColumnVisibility(shown); err != nil {
		drawFooter(fileNameStr, err.Error(), cursorPosStr)
		return false
	}
	next := view.DisplayColumn(source)
	if next < 0 {
		next = len(view.VisibleColumns()) - 1
		for index, column := range view.VisibleColumns() {
			if column > source {
				next = index
				break
			}
		}
	}
	bufferTable.Select(row, next)
	view.SetCursorColumn(view.SourceColumn(next))
	fileNameStr = buildFileInfoStr()
	cursorPosStr = buildCursorPosStr(row, next)
	drawFooter(fileNameStr, "Column visibility updated", cursorPosStr)
	return true
}

func hideSelectedColumn() {
	_, count := view.Dims()
	shown := make([]bool, count)
	for _, column := range view.VisibleColumns() {
		shown[column] = true
	}
	_, display := bufferTable.GetSelection()
	if source := view.SourceColumn(display); source >= 0 {
		shown[source] = false
		applyColumnVisibility(shown)
	}
}

func showColumnPicker() {
	_, count := view.Dims()
	list := tview.NewList().ShowSecondaryText(false)
	shown := make([]bool, count)
	label := func(column int) string {
		mark := "☐"
		if shown[column] {
			mark = "☑"
		}
		text := fmt.Sprintf("%s %d %s", mark, column+1, view.ColumnName(column))
		if _, filtered := view.FilterAt(column); filtered {
			text += " (filtered)"
		}
		return escapeMarkup(text)
	}
	view.VisibleColumns()
	for column := 0; column < count; column++ {
		shown[column] = !view.hiddenColumns[column]
		list.AddItem(label(column), "", 0, nil)
	}
	_, display := bufferTable.GetSelection()
	list.SetCurrentItem(max(view.SourceColumn(display), 0))
	closePicker := func() {
		UI.RemovePage("columnPicker")
		app.SetFocus(bufferTable)
	}
	toggle := func(column int) {
		shown[column] = !shown[column]
		list.SetItemText(column, label(column), "")
		list.SetTitle(" Columns ")
	}
	list.SetSelectedFunc(func(column int, _, _ string, _ rune) { toggle(column) })
	list.SetBorder(true).SetTitle(" Columns ")
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEscape:
			closePicker()
			return nil
		case tcell.KeyEnter:
			if applyColumnVisibility(shown) {
				closePicker()
			} else {
				list.SetTitle(" Keep at least one column visible ")
			}
			return nil
		case tcell.KeyRune:
			switch event.Rune() {
			case ' ':
				toggle(list.GetCurrentItem())
				return nil
			case 'j':
				list.SetCurrentItem(min(list.GetCurrentItem()+1, count-1))
				return nil
			case 'k':
				list.SetCurrentItem(max(list.GetCurrentItem()-1, 0))
				return nil
			}
		}
		return event
	})
	picker := tview.NewFrame(list).SetBorders(0, 0, 0, 0, 0, 0).
		AddText("Space toggle · Enter apply · Esc cancel", false, tview.AlignLeft, footerLeftColor)
	UI.AddPage("columnPicker", picker, true, true)
	app.SetFocus(list)
}
