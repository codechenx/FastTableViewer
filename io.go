package main

import (
	"compress/gzip"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// progressTracker helps display loading progress
type progressTracker struct {
	total        int64
	current      int64
	lastUpdate   time.Time
	updateEvery  int
	lineCount    int
	showProgress bool
	startTime    time.Time
}

func newProgressTracker(total int64, showProgress bool) *progressTracker {
	return &progressTracker{
		total:        total,
		current:      0,
		lastUpdate:   time.Now(),
		updateEvery:  5000, // update every 5000 lines
		lineCount:    0,
		showProgress: showProgress,
		startTime:    time.Now(),
	}
}

// update records absolute counts reported by the loader and refreshes the
// console line at most a few times a second.
func (p *progressTracker) update(lines int, loaded int64) {
	if !p.showProgress {
		return
	}
	p.lineCount = lines
	p.current = loaded

	if time.Since(p.lastUpdate) > 200*time.Millisecond {
		p.display()
		p.lastUpdate = time.Now()
	}
}

func (p *progressTracker) display() {
	if !p.showProgress {
		return
	}

	elapsed := time.Since(p.startTime).Seconds()
	if elapsed == 0 {
		elapsed = 0.001 // avoid division by zero
	}
	linesPerSec := float64(p.lineCount) / elapsed

	if p.total > 0 {
		percent := float64(p.current) * 100.0 / float64(p.total)
		if percent > 100 {
			percent = 100
		}
		fmt.Printf("\r\033[K%s %5.1f%%  %s lines  %.0f lines/sec",
			progressBar(percent, progressBarWidth), percent, formatCount(p.lineCount), linesPerSec)
	} else {
		// A stream or a compressed file: no total, so report the tally only.
		fmt.Printf("\r\033[K📊 Loading  %s lines  %.0f lines/sec",
			formatCount(p.lineCount), linesPerSec)
	}
}

func (p *progressTracker) finish() {
	if !p.showProgress {
		return
	}

	elapsed := time.Since(p.startTime).Seconds()
	if elapsed == 0 {
		elapsed = 0.001
	}
	linesPerSec := float64(p.lineCount) / elapsed

	// Clear the progress line and show final summary
	fmt.Printf("\r\033[K✓ Loaded %s lines in %.2fs (%.0f lines/sec)\n", formatCount(p.lineCount), elapsed, linesPerSec)
}

// check a line whether should bu skip, according to prefix
func skipLine(line string, sy []string) bool {
	for _, sy := range sy {
		if strings.HasPrefix(line, sy) {
			return true
		}

	}
	return false
}

// check columns that should be displayed
func getVisCol(showNumL, hideNumL []int, colLen int) ([]int, error) {
	for _, i := range showNumL {
		if i > colLen || i <= 0 {
			return nil, errors.New("Column number " + I2S(i) + " does not exist")
		}
	}

	for _, i := range hideNumL {
		if i > colLen || i <= 0 {
			return nil, errors.New("Column number " + I2S(i) + " does not exist")
		}
	}

	var visCol []int
	for i := 0; i < colLen; i++ {
		flag, err := checkVisible(showNumL, hideNumL, i)
		if err != nil {
			return nil, err
		}
		if flag {
			visCol = append(visCol, i)
		}
	}
	return visCol, nil

}

// check ith column should be displayed or not
func checkVisible(showNumL, hideNumL []int, col int) (bool, error) {
	if len(showNumL) != 0 && len(hideNumL) != 0 {
		return false, errors.New("you can only set visible column or hidden column")
	}

	if len(showNumL) != 0 {
		for _, colTestS := range showNumL {
			if col+1 == colTestS {
				return true, nil
			}
		}
		return false, nil
	}
	if len(hideNumL) != 0 {
		for _, colTestH := range hideNumL {
			if col+1 == colTestH {
				return false, nil
			}
		}
	}
	return true, nil
}

// use go csv library to parse a string line into csv format
// Optimized version with reusable reader
func lineCSVParse(s string, sep rune) ([]string, error) {
	r := csv.NewReader(strings.NewReader(s))
	r.Comma = sep
	r.LazyQuotes = true
	r.ReuseRecord = true //reuse backing array for performance
	//r.TrimLeadingSpace = true //disable, because it will remove NULL item and cause issue.
	record, err := r.Read()
	if err != nil {
		return nil, err
	}
	//make a copy since ReuseRecord=true reuses the backing array
	result := make([]string, len(record))
	copy(result, record)
	return result, err
}

// hasQuotes reports whether a line needs the full CSV parser.
func hasQuotes(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '"' {
			return true
		}
	}
	return false
}

// splitFieldsInto appends the fields of an unquoted line into block, returning
// the row and the extended block. Carving rows from a shared block costs a
// handful of allocations per batch instead of one per row.
//
// The row is returned with its capacity equal to its length, so widening it
// later (when a longer row grows the table) reallocates rather than writing
// over the next row's storage.
func splitFieldsInto(s string, sep rune, block []string) (row, out []string) {
	start := len(block)
	begin := 0
	for i := 0; i < len(s); i++ {
		if rune(s[i]) == sep {
			block = append(block, s[begin:i])
			begin = i + 1
		}
	}
	block = append(block, s[begin:])
	return block[start:len(block):len(block)], block
}

// Fast CSV parser for simple cases (no quotes, no escaping)
// Falls back to standard parser if needed
func lineCSVParseFast(s string, sep rune) ([]string, error) {
	// Use fast path for simple CSV lines
	if !hasQuotes(s) {
		// Count separators to pre-allocate slice
		sepCount := 0
		for i := 0; i < len(s); i++ {
			if rune(s[i]) == sep {
				sepCount++
			}
		}

		result := make([]string, 0, sepCount+1)
		start := 0
		for i := 0; i < len(s); i++ {
			if rune(s[i]) == sep {
				result = append(result, s[start:i])
				start = i + 1
			}
		}
		// Add last field
		result = append(result, s[start:])
		return result, nil
	}

	// Fall back to standard parser for complex cases
	return lineCSVParse(s, sep)
}

// openFileSource opens a delimited file, transparently decompressing .gz, and
// returns it as a Source. The Source owns the file handle and closes it, which
// nothing did when a bare scanner was handed out instead.
func openFileSource(fn string) (Source, error) {
	info, err := os.Stat(fn)
	if err != nil {
		return Source{}, err
	}
	if info.IsDir() {
		return Source{}, errors.New(fn + " is a directory")
	}

	file, err := os.Open(fn)
	if err != nil {
		return Source{}, err
	}

	src := Source{Name: fn, Reader: file, Size: info.Size(), Close: file.Close}

	if strings.HasSuffix(fn, ".gz") {
		gz, err := gzip.NewReader(file)
		if err != nil {
			_ = file.Close()
			return Source{}, err
		}
		src.Reader = gz
		src.Close = func() error {
			gzErr := gz.Close()
			fileErr := file.Close()
			if gzErr != nil {
				return gzErr
			}
			return fileErr
		}
		// Progress counts decompressed bytes, which the compressed file size
		// cannot measure, so report an unknown total and let the UI show rows.
		src.Size = 0
	}

	return src, nil
}

// pipeSource wraps a stream, whose total size is unknown.
func pipeSource(r io.Reader) Source {
	return Source{Reader: r}
}
