package main

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func rowViewerText(text string) string {
	var out strings.Builder
	for _, r := range text {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			quoted := strconv.QuoteRune(r)
			out.WriteString(quoted[1 : len(quoted)-1])
		} else {
			out.WriteRune(r)
		}
	}
	return escapeMarkup(out.String())
}

func showRowViewer(row, column int) {
	first, _ := view.Freeze()
	if bufferTable.GetRowCount() <= first {
		return
	}
	if row < first {
		row = first
	}
	userMovedCursor = true
	reader := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(true).
		SetWordWrap(true).
		SetScrollable(true)
	reader.SetBorder(true)

	show := func() {
		var text strings.Builder
		for col := 0; col < bufferTable.GetColumnCount(); col++ {
			field := (bufferContent{}).GetCell(row, col).GetReference().(cellField)
			fmt.Fprintf(&text, "[\"field-%d\"][::b]%d %s[::-][\"\"]\n%s\n\n",
				col, field.Column+1, rowViewerText(field.Name), rowViewerText(field.Value))
		}
		reader.SetText(text.String())
		reader.SetTitle(fmt.Sprintf(" Row %d/%d · n next · p previous · Esc close ", row-first+1, bufferTable.GetRowCount()-first))
		reader.Highlight("field-" + I2S(column)).ScrollToHighlight()
	}
	reader.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Key() == tcell.KeyRune && event.Rune() == 'q' {
			UI.RemovePage("rowViewer")
			bufferTable.Select(row, column)
			app.SetFocus(bufferTable)
			return nil
		}
		if event.Key() == tcell.KeyRune {
			switch event.Rune() {
			case 'n':
				if row+1 < bufferTable.GetRowCount() {
					row++
					show()
				}
				return nil
			case 'p':
				if row > first {
					row--
					show()
				}
				return nil
			}
		}
		return event
	})
	show()
	UI.AddPage("rowViewer", reader, true, true)
	app.SetFocus(reader)
}
