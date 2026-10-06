package main

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

// errNoInput means neither a file argument nor piped input was supplied.
var errNoInput = errors.New("no input")

const (
	// progressRefreshInterval is how often the loading readout is repainted.
	progressRefreshInterval = 20 * time.Millisecond

	// measureEveryTicks is how many refresh ticks pass between re-measuring
	// the columns while rows stream in.
	measureEveryTicks = 25
)

func main() {
	initView()
	args.setDefault()
	RootCmd := &cobra.Command{
		Use:     "ftv {File_Name}",
		Version: "0.8",
		Short:   "Fast table viewer for delimited file in terminal",
		Run:     runViewer,
	}

	RootCmd.Flags().StringVarP(&args.Sep, "separator", "s", "", "Delimiter/separator character (use \\t for tab)")
	RootCmd.Flags().IntVarP(&args.NLine, "lines", "n", 0, "Display only first N lines")
	RootCmd.Flags().StringSliceVar(&args.SkipSymbol, "skip-prefix", []string{}, "Skip lines starting with prefix (comma-separated)")
	RootCmd.Flags().IntVar(&args.SkipNum, "skip-lines", 0, "Skip first N lines")
	RootCmd.Flags().IntSliceVar(&args.ShowNum, "columns", []int{}, "Show only specified columns (comma-separated)")
	RootCmd.Flags().IntSliceVar(&args.HideNum, "hide-columns", []int{}, "Hide specified columns (comma-separated)")
	RootCmd.Flags().IntVarP(&args.Header, "freeze", "f", 0, "Freeze mode: -1=none, 0=row+col, 1=row only, 2=col only")
	RootCmd.Flags().BoolVar(&args.Strict, "strict", false, "Strict mode: fail on missing/inconsistent data")
	RootCmd.Flags().BoolVar(&args.AsyncLoad, "async", true, "Progressive rendering while loading")
	RootCmd.Flags().SortFlags = false
	err := RootCmd.Execute()
	fatalError(err)
}

// runViewer is the one startup path: resolve where the rows come from, load
// them, then show them. Whether loading is progressive is a choice about when
// to draw, not a second way to load.
func runViewer(cmd *cobra.Command, cmdargs []string) {
	src, piped, err := resolveSource(cmdargs)
	if errors.Is(err, errNoInput) {
		stopView()
		_ = cmd.Help()
		return
	}
	if err != nil {
		stopView()
		if errors.Is(err, os.ErrNotExist) {
			fmt.Printf("⚠️  File not found: %s\n", cmdargs[0])
		} else {
			fmt.Printf("⚠️  Cannot access input: %s\n", err)
		}
		os.Exit(1)
	}

	if piped {
		args.FileName = "From Shell Pipe"
	} else {
		args.FileName = src.Name
	}

	buf := view.Original()
	applyFreezeMode(buf, args.Header)

	intake := Intake{Source: src, Config: args.intakeConfig()}

	if args.AsyncLoad {
		loadProgressively(intake, buf, piped)
		return
	}
	loadAndShow(intake, buf, piped)
}

// resolveSource decides where rows come from: the named file, or standard input
// when something was piped in.
func resolveSource(cmdargs []string) (Source, bool, error) {
	info, err := os.Stdin.Stat()
	if err != nil {
		return Source{}, false, err
	}

	// A character device on stdin means nothing was piped in, so a file
	// argument is required.
	if info.Mode()&os.ModeCharDevice != 0 {
		if len(cmdargs) < 1 {
			return Source{}, false, errNoInput
		}
		src, err := openFileSource(cmdargs[0])
		return src, false, err
	}

	return pipeSource(os.Stdin), true, nil
}

// applyFreezeMode maps the --freeze flag onto the frozen row and column counts.
// It runs before loading so that type detection and strict-mode messages see
// the header the user asked for.
func applyFreezeMode(b *Buffer, mode int) {
	rows, cols := 1, 1
	switch mode {
	case -1:
		rows, cols = 0, 0
	case 1:
		rows, cols = 1, 0
	case 2:
		rows, cols = 0, 1
	}

	b.mu.Lock()
	b.rowFreeze, b.colFreeze = rows, cols
	b.mu.Unlock()
}

// loadAndShow loads everything, reporting progress to the console, then draws.
func loadAndShow(intake Intake, buf *Buffer, piped bool) {
	console := newProgressTracker(intake.Source.Size, true)
	intake.OnProgress = func(rows int, loaded, total int64) {
		loadProgress.Set(rows, loaded, total)
		console.update(rows, loaded)
	}

	err := intake.Into(buf)
	loadProgress.Finish()
	console.finish()
	fatalError(err)

	if exitIfEmpty(buf, piped) {
		return
	}
	fatalError(drawUI())
	runApp()
}

// loadProgressively draws as soon as the first rows land and refreshes while
// the rest arrive.
func loadProgressively(intake Intake, buf *Buffer, piped bool) {
	userMovedCursor = false

	ready := make(chan struct{}, 1)
	done := make(chan error, 1)
	var first sync.Once

	intake.OnProgress = func(rows int, loaded, total int64) {
		loadProgress.Set(rows, loaded, total)
		first.Do(func() { ready <- struct{}{} })
	}

	go func() { done <- intake.Into(buf) }()

	// Wait for something to draw, or for a load that failed or finished first.
	select {
	case <-ready:
	case err := <-done:
		loadProgress.Finish()
		fatalError(err)
		if exitIfEmpty(buf, piped) {
			return
		}
		fatalError(drawUI())
		runApp()
		return
	}

	if exitIfEmpty(buf, piped) {
		return
	}
	rows, loaded, total, _ := loadProgress.Snapshot()
	statusMessage = buildLoadingStatus(rows, loaded, total, 0)
	fatalError(drawUI())

	go refreshWhileLoading(done)
	runApp()
}

// refreshWhileLoading repaints the table until the load finishes.
func refreshWhileLoading(done <-chan error) {
	ticker := time.NewTicker(progressRefreshInterval)
	defer ticker.Stop()

	tick := 0
	for {
		select {
		case err := <-done:
			loadProgress.Finish()
			if err != nil {
				fatalError(err)
				return
			}
			rows, _, _, _ := loadProgress.Snapshot()
			app.QueueUpdateDraw(func() {
				// Column widths and types are only knowable once the rows are
				// in, and drawUI measured them as soon as the first batch
				// landed. Measure again, and refresh the readout rather than
				// leaving the pre-load one on screen.
				view.MeasureColumns()
				fileNameStr = buildFileInfoStr()

				row, col := bufferTable.GetSelection()
				cursorPosStr = buildCursorPosStr(row, col)
				updateFooterWithStatus("Loaded " + formatCount(rows) + " rows")
			})
			return

		case <-ticker.C:
			tick++
			// Widen columns to fit as longer values arrive, rather than
			// leaving them at whatever the first batch happened to need.
			remeasure := tick%measureEveryTicks == 0

			app.QueueUpdateDraw(func() {
				if remeasure {
					view.MeasureColumns()
					fileNameStr = buildFileInfoStr()
				}
				// Keep the cursor on the first row until the user moves it.
				if !userMovedCursor {
					row, col := bufferTable.GetSelection()
					if row != 0 {
						bufferTable.Select(0, col)
					}
				}

				rows, loaded, total, _ := loadProgress.Snapshot()
				updateFooterWithStatus(buildLoadingStatus(rows, loaded, total, tick))
			})
		}
	}
}

// exitIfEmpty reports whether there is nothing to show, having said so.
func exitIfEmpty(b *Buffer, piped bool) bool {
	b.mu.RLock()
	rowLen, rowFreeze := b.rowLen, b.rowFreeze
	b.mu.RUnlock()

	if rowLen > 0 && rowLen-rowFreeze > 0 {
		return false
	}

	stopView()
	switch {
	case piped && rowLen == 0:
		fmt.Println("⚠️  No data received from pipe (empty input)")
	case piped:
		fmt.Println("⚠️  No data received from pipe (only header, no data rows)")
	case rowLen == 0:
		fmt.Println("⚠️  File is empty (no rows)")
	default:
		fmt.Println("⚠️  File is empty (only header, no data rows)")
	}
	os.Exit(0)
	return true
}

// runApp starts the event loop unless the process is under test.
func runApp() {
	if debug {
		return
	}
	if err := app.SetRoot(UI, true).SetFocus(UI).Run(); err != nil {
		// The screen never came up — there is no terminal, for instance when
		// output is redirected. Asking tview to stop it would panic inside
		// tcell on a half-built screen, so report and leave directly.
		fmt.Fprintf(os.Stderr, "⚠️  Cannot start the terminal UI: %s\n", err)
		os.Exit(1)
	}
}
