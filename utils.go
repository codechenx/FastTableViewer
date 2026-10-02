package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/fatih/color"
)

// print fatal error and force quite app
func fatalError(err error) {
	if err != nil {
		color.Set(color.FgRed)
		fmt.Println(err)
		color.Unset()
		if app != nil {
			app.Stop()
		}
		if !debug {
			os.Exit(1)
		}
	}
}

// print useful info and force quite app
func usefulInfo(s string) {
	color.Set(color.FgHiYellow)
	fmt.Println(s)
	color.Unset()
}

// I2B  covert int to bool, if i >0:true, else false
func I2B(i int) bool {
	return i > 0
}

// F2S covert float64 to bool
func F2S(i float64) string {
	return strconv.FormatFloat(i, 'f', 4, 64)
}

// S2F covert string to float64
func S2F(i string) float64 {
	s, err := strconv.ParseFloat(i, 64)
	if err != nil {
		fatalError(err)
	}
	return s
}

// I2S covert int to string
func I2S(i int) string {
	return strconv.Itoa(i)
}

func getHelpContent() string {
	helpContent := `[::b][yellow]━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━[white]

[::b][cyan]🚀 TV - Modern Terminal Table Viewer[-][white]

[::b][yellow]━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━[white]

[::b][green]📖 Help Navigation[white]
  [yellow]j/k[-]                 Scroll help text
  [yellow]gg/G[-]                Jump to top/bottom
  [yellow]Ctrl-d/u[-]            Page down/up
  [yellow]? or q or Esc[-]       Close help dialog

[::b][red]🚪 Quit[white]
  [yellow]q[-]                   Quit application
  [yellow]Esc[-]                 Close dialog or clear search

[::b][blue]⬆️ Movement[white]
  [yellow]h[-]                   Move left ⬅️
  [yellow]l[-]                   Move right ➡️
  [yellow]j[-]                   Move down ⬇️
  [yellow]k[-]                   Move up ⬆️

  [yellow]w[-]                   Move to next column (word forward)
  [yellow]b[-]                   Move to previous column (word backward)

  [yellow]gg[-]                  Go to first row (press g twice)
  [yellow]G[-]                   Go to last row

  [yellow]0[-]                   Go to first column
  [yellow]$[-]                   Go to last column

  [yellow]Ctrl-d[-]              Page down (half page)
  [yellow]Ctrl-u[-]              Page up (half page)

[::b][cyan]🖱️  Mouse Support[white]
  [yellow]Left Click[-]          Select cell
  [yellow]Scroll Wheel[-]        Scroll up/down through rows
  [yellow]Click Buttons[-]       Interact with dialogs and forms

[::b][magenta]🔍 Search[white]
  [yellow]/[-]                   Search for text
                    • Case-insensitive by default
                    • Press [yellow]Tab[-] to navigate to checkbox
                    • Press [yellow]Space[-] to toggle [yellow]Use Regex[-] option
  [yellow]n[-]                   Next search result ⏭
  [yellow]N[-]                   Previous search result ⏮
  [yellow]Esc[-]                 Clear search highlighting

[::b][green]🎯 Regex Search Examples[white]
  [yellow]^start[-]              Match at beginning of cell
  [yellow]end$[-]                Match at end of cell
  [yellow]\d+[-]                 Match digits (numbers)
  [yellow]@.*\.com[-]            Match email pattern
  [yellow]word1|word2[-]         Match either word (OR)
  [yellow][A-Z]+[-]              Match uppercase letters

[::b][orange]🔎 Filter[white]
  [yellow]f[-]                   Filter rows by current column value
                    • Apply filters to multiple columns
                    • Edit filter: press f on filtered column
                    OR: same cell has either term
                    AND: same cell has both terms
                    ROR: different rows, any match (uppercase only)
  [yellow]r[-]                   Remove filter from current column

[::b][purple]🏷️  Data Type[white]
  [yellow]t[-]                   Toggle column data type
                    (String → Number → Date → String)

[::b][green]🔃 Sort[white]
  [yellow]s[-]                   Sort data by column (ascending ⬆️)
  [yellow]S[-]                   Sort data by column (descending ⬇️)

[::b][cyan]📏 Text Wrapping[white]
  [yellow]W[-]                   Toggle width limit for current column (50 chars)
                    Long columns (>50 chars) are limited automatically

[::b][blue]📊 Stats[white]
  [yellow]i[-]                   Show stats info for current column

[::b][yellow]❓ Help[white]
  [yellow]?[-]                   Show this help dialog

[::b][yellow]━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━[white]

[::b][green]💡 Pro Tips:[white]
  • Press [yellow]gg[-] to jump to the top of any table
  • Use [yellow]/[-] for quick searching across all cells
  • Enable [yellow]regex[-] mode for powerful pattern matching
  • Press [yellow]i[-] to see detailed statistics for any column
  • Use [yellow]f[-] on multiple columns to combine filters
  • Headers are frozen by default for easy navigation

[::b][yellow]━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━[white]
`
	return helpContent
}

// wrapText wraps text to fit within maxWidth characters
// Returns the wrapped text with newlines
func wrapText(text string, maxWidth int) string {
	if maxWidth <= 0 || len(text) <= maxWidth {
		return text
	}

	var result []rune
	runes := []rune(text)
	lineStart := 0

	for i := 0; i < len(runes); i++ {
		// Check if we've reached the wrap point
		if i-lineStart >= maxWidth {
			// Find last space before maxWidth for word wrap
			wrapPoint := i
			for j := i; j > lineStart; j-- {
				if runes[j] == ' ' || runes[j] == '\t' || runes[j] == '-' {
					wrapPoint = j + 1
					break
				}
			}

			// If no good wrap point found, hard wrap at maxWidth
			if wrapPoint == i && i > lineStart {
				wrapPoint = lineStart + maxWidth
			}

			// Add the wrapped line
			result = append(result, runes[lineStart:wrapPoint]...)
			result = append(result, '\n')

			// Skip trailing spaces on new line
			for wrapPoint < len(runes) && (runes[wrapPoint] == ' ' || runes[wrapPoint] == '\t') {
				wrapPoint++
			}

			lineStart = wrapPoint
			i = wrapPoint - 1 // -1 because loop will increment
		}
	}

	// Add remaining text
	if lineStart < len(runes) {
		result = append(result, runes[lineStart:]...)
	}

	return string(result)
}

// truncateText truncates text to maxWidth and adds ellipsis if needed
func truncateText(text string, maxWidth int) string {
	if maxWidth <= 0 {
		return text
	}

	runes := []rune(text)
	if len(runes) <= maxWidth {
		return text
	}

	// Reserve 3 characters for ellipsis
	if maxWidth <= 3 {
		return string(runes[:maxWidth])
	}

	return string(runes[:maxWidth-3]) + "..."
}

// performSearch returns every cell in b matching spec. The matcher is compiled
// once for the whole scan; an unusable pattern yields no results.
func performSearch(b *Buffer, spec MatchSpec) []SearchResult {
	results := []SearchResult{}

	matcher, err := CompileMatcher(spec, colTypeStr)
	if err != nil {
		return results
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	// Scan column by column (same column first, then next column).
	for c := 0; c < b.colLen; c++ {
		for r := 0; r < b.rowLen; r++ {
			if c >= len(b.cont[r]) {
				continue
			}
			if matcher.MatchCell(b.cont[r][c]) {
				results = append(results, SearchResult{Row: r, Col: c})
			}
		}
	}

	return results
}

// formatCount renders a count with thousands separators, so a row tally stays
// legible as it grows.
func formatCount(n int) string {
	if n < 0 {
		return strconv.Itoa(n)
	}
	s := strconv.Itoa(n)
	var out strings.Builder
	for i := 0; i < len(s); i++ {
		if i > 0 && (len(s)-i)%3 == 0 {
			out.WriteByte(',')
		}
		out.WriteByte(s[i])
	}
	return out.String()
}

// progressBar renders percent (0-100) as a bar of the given cell width.
func progressBar(percent float64, width int) string {
	if width <= 0 {
		return ""
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	filled := int(percent / 100 * float64(width))
	if filled > width {
		filled = width
	}
	// Show a sliver as soon as there is any progress at all, so the bar never
	// looks stalled while rows are arriving.
	if filled == 0 && percent > 0 {
		filled = 1
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}
