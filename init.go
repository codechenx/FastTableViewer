package main

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Application-level state. What the viewer *shows* lives behind view; what
// remains here is the terminal UI itself, the strings currently painted in the
// footer, and the input mode needed to recognise a two-key sequence.
var (
	app         *tview.Application
	UI          *tview.Pages
	mainPage    *tview.Frame
	bufferTable *tview.Table

	view  *ViewState // the data on screen, and every rule for changing it
	args  Args
	debug bool

	loadProgress LoadProgress

	statusMessage string // footer centre
	fileNameStr   string // footer left
	cursorPosStr  string // footer right

	userMovedCursor bool      // whether to keep the cursor pinned while loading
	lastGPress      time.Time // for recognising "gg"
)

// gRepeatWindow is how long the second 'g' of a "gg" may arrive.
const gRepeatWindow = 500 * time.Millisecond

// SearchResult represents a cell that matches a search query.
type SearchResult struct {
	Row int
	Col int
}

// initialize tview and the view
func initView() {
	app = tview.NewApplication()
	app.EnableMouse(true) // Enable mouse support
	view = NewViewState(createNewBuffer())
	userMovedCursor = false
	lastGPress = time.Time{}
	statusMessage = ""
	fileNameStr = ""
	cursorPosStr = ""
}

// stop UI
func stopView() {
	if app != nil {
		app.Stop()
	}
}

// pressedGTwice reports whether this 'g' completes a "gg" within the repeat
// window. Using a timestamp keeps the check on the event loop; the flag it
// replaced was cleared from a timer goroutine, racing with the key handler.
func pressedGTwice() bool {
	if !lastGPress.IsZero() && time.Since(lastGPress) < gRepeatWindow {
		lastGPress = time.Time{}
		return true
	}
	lastGPress = time.Now()
	return false
}

// updateFooterWithStatus updates the footer with a status message
func updateFooterWithStatus(status string) {
	statusMessage = status
	if mainPage != nil {
		// Update the footer by rebuilding it
		mainPage.Clear()
		mainPage.AddText(fileNameStr, false, tview.AlignLeft, tcell.ColorDarkOrange).
			AddText(status, false, tview.AlignCenter, tcell.ColorDarkOrange).
			AddText(cursorPosStr, false, tview.AlignRight, tcell.ColorDarkOrange)
	}
}
