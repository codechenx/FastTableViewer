package main

import (
	"bufio"
	"errors"
	"io"
	"runtime"
	"strings"
	"sync"
)

// Source is where rows come from. A file and a pipe are the two adapters that
// satisfy it, which is what makes this a real seam rather than a hypothetical
// one: tests supply a third, an in-memory reader.
type Source struct {
	// Name is shown in the footer and supplies the .csv / .tsv separator hint.
	// Empty for a stream.
	Name string

	Reader io.Reader

	// Size is the total byte count when it is known, 0 for a stream.
	Size int64

	// Close releases the source. nil when there is nothing to release.
	Close func() error
}

// IntakeConfig is an immutable snapshot of the rules for one load. Intake
// never writes to it, so loading the same source twice gives the same result
// twice.
type IntakeConfig struct {
	Sep        rune     // 0 means detect
	SkipPrefix []string // drop lines starting with any of these
	SkipLines  int      // drop this many leading lines
	MaxLines   int      // stop after this many rows, 0 for no limit
	ShowCols   []int    // 1-based columns to keep, empty for all
	HideCols   []int    // 1-based columns to drop, empty for none
	Strict     bool     // fail on a row with the wrong column count
}

const (
	// intakeDetectLines is how many lines are buffered to detect a separator.
	intakeDetectLines = 10

	// intakeBatchSize is how many lines are parsed in parallel at a time.
	// Batching keeps parsing concurrent while appending stays in input order.
	intakeBatchSize = 2048

	// intakeMaxWorkers caps the parse fan-out.
	intakeMaxWorkers = 8

	// intakeScanBuffer raises the scanner's line limit from the 64KB default.
	intakeScanBuffer = 1024 * 1024
)

// Intake loads one Source into one Buffer. Progressive rendering is the
// OnProgress callback, not a second implementation: a caller that wants to
// render while loading runs Into in a goroutine and refreshes from the
// callback, and a caller that does not simply leaves it nil.
type Intake struct {
	Source Source
	Config IntakeConfig

	// OnProgress, when set, is called from the loading goroutine every
	// ProgressEvery rows with the rows appended so far and the bytes consumed.
	OnProgress func(rows int, loaded, total int64)

	// ProgressEvery is the row interval between OnProgress calls. Zero uses a
	// sensible default.
	ProgressEvery int
}

// Into loads the source into b. It closes the source before returning.
func (in Intake) Into(b *Buffer) error {
	if in.Source.Close != nil {
		defer func() { _ = in.Source.Close() }()
	}

	scanner := bufio.NewScanner(in.Source.Reader)
	scanner.Split(bufio.ScanLines)
	scanner.Buffer(make([]byte, intakeScanBuffer), intakeScanBuffer)

	lines := &lineFeed{scanner: scanner, cfg: in.Config, skipLeft: in.Config.SkipLines}

	// Buffer enough lines to settle the separator. These are real rows and are
	// appended below, in order, before anything is streamed.
	var head []string
	for len(head) < intakeDetectLines {
		line, ok := lines.next()
		if !ok {
			break
		}
		head = append(head, line)
	}

	sep, err := in.separator(head)
	if err != nil {
		return err
	}
	b.mu.Lock()
	b.sep = sep
	b.mu.Unlock()

	every := in.ProgressEvery
	if every <= 0 {
		every = 500
	}

	state := &intakeState{
		in:    in,
		b:     b,
		sep:   sep,
		every: every,
		total: in.Source.Size,
	}

	if err := state.appendBatch(head); err != nil {
		return err
	}
	// Tell the caller the first rows are in, so a progressive renderer can
	// paint something immediately rather than waiting for a full batch.
	state.report()

	// Stream the rest in batches. Each batch parses in parallel and appends in
	// input order, so row order always matches the source.
	batch := make([]string, 0, intakeBatchSize)
	for !state.done() {
		line, ok := lines.next()
		if !ok {
			break
		}
		batch = append(batch, line)
		if len(batch) >= intakeBatchSize {
			if err := state.appendBatch(batch); err != nil {
				return err
			}
			batch = batch[:0]
		}
	}
	if err := state.appendBatch(batch); err != nil {
		return err
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	b.detectAllColumnTypes()
	state.report()
	return nil
}

// separator resolves the column separator: an explicit setting wins, then the
// file extension, then detection over the buffered lines.
func (in Intake) separator(head []string) (rune, error) {
	if in.Config.Sep != 0 {
		return in.Config.Sep, nil
	}

	switch {
	case strings.HasSuffix(in.Source.Name, ".csv"):
		return ',', nil
	case strings.HasSuffix(in.Source.Name, ".tsv"):
		return '\t', nil
	}

	sd := sepDetecor{}
	if sep := sd.sepDetect(head); sep != 0 {
		return sep, nil
	}
	return 0, errors.New("ftv can't identify separator, you need to set it manually with -s")
}

// lineFeed applies the skip rules once, for every source. It keeps its own
// countdown so the configuration it reads is never modified.
type lineFeed struct {
	scanner  *bufio.Scanner
	cfg      IntakeConfig
	skipLeft int
}

// next returns the next line that survives the skip rules.
func (lf *lineFeed) next() (string, bool) {
	for lf.scanner.Scan() {
		line := lf.scanner.Text()

		// Preserves existing behaviour: ScanLines strips the newline, so this
		// never fires and genuinely blank lines become rows.
		if line == "\n" {
			continue
		}
		if lf.skipLeft > 0 {
			lf.skipLeft--
			continue
		}
		if skipLine(line, lf.cfg.SkipPrefix) {
			continue
		}
		return line, true
	}
	return "", false
}

// intakeState carries the running totals for one load.
type intakeState struct {
	in     Intake
	b      *Buffer
	sep    rune
	every  int
	rows   int
	loaded int64
	total  int64
	sinceR int
}

// done reports whether the row limit has been reached.
func (s *intakeState) done() bool {
	return s.in.Config.MaxLines > 0 && s.rows >= s.in.Config.MaxLines
}

// report invokes the progress callback, if there is one.
func (s *intakeState) report() {
	if s.in.OnProgress != nil {
		s.in.OnProgress(s.rows, s.loaded, s.total)
	}
}

// appendBatch parses lines in parallel then appends them in input order.
func (s *intakeState) appendBatch(lines []string) error {
	if len(lines) == 0 {
		return nil
	}

	parsed := make([][]string, len(lines))
	errs := make([]error, len(lines))

	workers := runtime.NumCPU()
	if workers > intakeMaxWorkers {
		workers = intakeMaxWorkers
	}
	if workers > len(lines) {
		workers = len(lines)
	}

	var wg sync.WaitGroup
	chunk := (len(lines) + workers - 1) / workers
	for w := 0; w < workers; w++ {
		lo := w * chunk
		hi := lo + chunk
		if hi > len(lines) {
			hi = len(lines)
		}
		if lo >= hi {
			continue
		}
		wg.Add(1)
		go func(lo, hi int) {
			defer wg.Done()
			for i := lo; i < hi; i++ {
				fields, err := lineCSVParseFast(lines[i], s.sep)
				if err != nil {
					errs[i] = err
					continue
				}
				parsed[i] = fields
			}
		}(lo, hi)
	}
	wg.Wait()

	for i, line := range lines {
		if s.done() {
			return nil
		}
		if errs[i] != nil {
			return errs[i]
		}

		fields, err := visibleFields(parsed[i], s.in.Config.ShowCols, s.in.Config.HideCols)
		if err != nil {
			return err
		}
		if err := s.b.contAppendSli(fields, s.in.Config.Strict); err != nil {
			return err
		}

		s.rows++
		s.sinceR++
		s.loaded += int64(len(line) + 1) // +1 for the stripped newline
		if s.sinceR >= s.every {
			s.sinceR = 0
			s.report()
		}
	}
	return nil
}

// visibleFields reduces a parsed row to the columns the configuration keeps.
func visibleFields(fields []string, showCols, hideCols []int) ([]string, error) {
	if len(showCols) == 0 && len(hideCols) == 0 {
		return fields, nil
	}

	visCol, err := getVisCol(showCols, hideCols, len(fields))
	if err != nil {
		return nil, err
	}

	out := make([]string, 0, len(visCol))
	for _, i := range visCol {
		if i < len(fields) {
			out = append(out, fields[i])
		}
	}
	return out, nil
}

// LoadProgress tracks how far a load has got. The loader writes it and the UI
// reads it from another goroutine, so access is guarded.
type LoadProgress struct {
	mu          sync.RWMutex
	Rows        int
	TotalBytes  int64
	LoadedBytes int64
	IsComplete  bool
}

// Set records the latest counts.
func (lp *LoadProgress) Set(rows int, loaded, total int64) {
	lp.mu.Lock()
	defer lp.mu.Unlock()
	lp.Rows, lp.LoadedBytes, lp.TotalBytes = rows, loaded, total
}

// Finish marks the load complete.
func (lp *LoadProgress) Finish() {
	lp.mu.Lock()
	defer lp.mu.Unlock()
	lp.IsComplete = true
}

// Snapshot returns a consistent view of the counts.
func (lp *LoadProgress) Snapshot() (rows int, loaded, total int64, complete bool) {
	lp.mu.RLock()
	defer lp.mu.RUnlock()
	return lp.Rows, lp.LoadedBytes, lp.TotalBytes, lp.IsComplete
}

// GetPercentage returns the loading percentage (0-100), or 0 when the total is
// unknown, as it is for a pipe.
func (lp *LoadProgress) GetPercentage() float64 {
	lp.mu.RLock()
	defer lp.mu.RUnlock()

	if lp.TotalBytes <= 0 {
		return 0
	}
	percent := float64(lp.LoadedBytes) * 100.0 / float64(lp.TotalBytes)
	if percent > 100 {
		percent = 100
	}
	return percent
}
