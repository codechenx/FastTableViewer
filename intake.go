package main

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"runtime"
	"strings"
	"sync"
	"time"
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

	// intakeMaxReserveRows bounds the row index reserved up front, so a badly
	// estimated row count cannot commit an unreasonable amount of memory.
	intakeMaxReserveRows = 8 << 20

	// intakeReportInterval reports progress on a clock as well as per N rows.
	// A row count alone goes silent on a slow source — a pipe delivering fewer
	// rows than the row interval never reports at all — which is precisely
	// when a caller most wants to show that work is happening.
	intakeReportInterval = 100 * time.Millisecond
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

	lines := newLineFeed(scanner, in.Config)

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
		in:         in,
		b:          b,
		sep:        sep,
		every:      every,
		total:      in.Source.Size,
		lastReport: time.Now(),
	}

	if err := state.appendBatch(head); err != nil {
		return err
	}
	// Tell the caller the first rows are in, so a progressive renderer can
	// paint something immediately rather than waiting for a full batch.
	state.report()

	// Reserve the row index up front, estimating the count from the source
	// size and the average line length seen so far. Growing it by appending
	// reallocates and copies repeatedly, which was the single largest source
	// of allocation in a large load.
	b.reserveRows(state.estimateRows())

	// Stream the rest in batches. Each batch parses in parallel and appends in
	// input order, so row order always matches the source.
	//
	// Lines are gathered as bytes and turned into a single string per batch,
	// from which each line and then each field is a substring. That costs one
	// allocation for a batch's text rather than one per line, which was the
	// last per-row allocation in a load.
	var raw []byte
	spans := make([][2]int, 0, intakeBatchSize)

	flush := func() error {
		if len(spans) == 0 {
			return nil
		}
		blob := string(raw)
		batch := state.lines[:0]
		for _, sp := range spans {
			batch = append(batch, blob[sp[0]:sp[1]])
		}
		state.lines = batch

		raw = raw[:0]
		spans = spans[:0]
		return state.appendBatch(batch)
	}

	for !state.done() {
		line, ok := lines.nextBytes()
		if !ok {
			break
		}
		start := len(raw)
		raw = append(raw, line...)
		spans = append(spans, [2]int{start, len(raw)})

		// Flush a full batch, or a partial one once a report is due. Waiting
		// for a full batch means a slow stream shows nothing at all until its
		// producer closes, which defeats progressive rendering.
		if len(spans) >= intakeBatchSize || state.reportDue() {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := flush(); err != nil {
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
	prefixes [][]byte // SkipPrefix, converted once
}

// newLineFeed prepares a feed over scanner.
func newLineFeed(scanner *bufio.Scanner, cfg IntakeConfig) *lineFeed {
	prefixes := make([][]byte, 0, len(cfg.SkipPrefix))
	for _, p := range cfg.SkipPrefix {
		prefixes = append(prefixes, []byte(p))
	}
	return &lineFeed{scanner: scanner, cfg: cfg, skipLeft: cfg.SkipLines, prefixes: prefixes}
}

// nextBytes returns the next line that survives the skip rules. The bytes
// belong to the scanner and are only valid until the following call, so a
// caller that needs to keep them must copy them out.
func (lf *lineFeed) nextBytes() ([]byte, bool) {
	for lf.scanner.Scan() {
		line := lf.scanner.Bytes()

		// Preserves existing behaviour: ScanLines strips the newline, so this
		// never fires and genuinely blank lines become rows.
		if len(line) == 1 && line[0] == '\n' {
			continue
		}
		if lf.skipLeft > 0 {
			lf.skipLeft--
			continue
		}
		if bytesHasAnyPrefix(line, lf.prefixes) {
			continue
		}
		return line, true
	}
	return nil, false
}

// next returns the next surviving line as its own string.
func (lf *lineFeed) next() (string, bool) {
	line, ok := lf.nextBytes()
	if !ok {
		return "", false
	}
	return string(line), true
}

// bytesHasAnyPrefix reports whether line starts with any of the prefixes.
func bytesHasAnyPrefix(line []byte, prefixes [][]byte) bool {
	for _, p := range prefixes {
		if bytes.HasPrefix(line, p) {
			return true
		}
	}
	return false
}

// intakeState carries the running totals for one load.
type intakeState struct {
	in         Intake
	b          *Buffer
	sep        rune
	every      int
	rowCount   int
	loaded     int64
	total      int64
	sinceR     int
	lastReport time.Time

	// Scratch reused across batches. Allocating these per batch accounted for
	// a fifth of a large load's allocations.
	parsed [][]string
	errs   []error
	rows   [][]string
	lines  []string
}

// estimateRows guesses the source's total row count from its size and the
// average length of the rows read so far. It returns 0 when the size is not
// knowable, as for a stream, leaving the index to grow as it goes.
func (s *intakeState) estimateRows() int {
	if s.total <= 0 || s.rowCount == 0 || s.loaded == 0 {
		return 0
	}

	avg := float64(s.loaded) / float64(s.rowCount)
	est := int(float64(s.total)/avg) + intakeBatchSize

	// A short sample can mis-estimate badly, so bound the reservation.
	if est > intakeMaxReserveRows {
		est = intakeMaxReserveRows
	}
	if m := s.in.Config.MaxLines; m > 0 && est > m {
		est = m
	}
	return est
}

// done reports whether the row limit has been reached.
func (s *intakeState) done() bool {
	return s.in.Config.MaxLines > 0 && s.rowCount >= s.in.Config.MaxLines
}

// report invokes the progress callback, if there is one.
func (s *intakeState) report() {
	s.sinceR = 0
	s.lastReport = time.Now()
	if s.in.OnProgress != nil {
		s.in.OnProgress(s.rowCount, s.loaded, s.total)
	}
}

// reportDue reports whether enough rows or enough time have passed to warrant
// another progress report.
func (s *intakeState) reportDue() bool {
	return s.sinceR >= s.every || time.Since(s.lastReport) >= intakeReportInterval
}

// appendBatch parses lines in parallel then appends them in input order.
func (s *intakeState) appendBatch(lines []string) error {
	if len(lines) == 0 {
		return nil
	}

	if cap(s.parsed) < len(lines) {
		s.parsed = make([][]string, len(lines))
		s.errs = make([]error, len(lines))
		s.rows = make([][]string, 0, len(lines))
		s.lines = make([]string, 0, len(lines))
	}
	parsed, errs := s.parsed[:len(lines)], s.errs[:len(lines)]

	// Field storage is carved per worker from one block, so parsing a batch
	// costs a few allocations rather than one per row.
	fieldsPerRow := 8
	s.b.mu.RLock()
	if s.b.colLen > 0 {
		fieldsPerRow = s.b.colLen + 1
	}
	s.b.mu.RUnlock()

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
			block := make([]string, 0, (hi-lo)*fieldsPerRow)
			for i := lo; i < hi; i++ {
				errs[i] = nil
				if hasQuotes(lines[i]) {
					parsed[i], errs[i] = lineCSVParse(lines[i], s.sep)
					continue
				}
				parsed[i], block = splitFieldsInto(lines[i], s.sep, block)
			}
		}(lo, hi)
	}
	wg.Wait()

	// Collect the batch's visible rows, then append them under one lock.
	rows := s.rows[:0]
	var bytesRead int64
	for i, line := range lines {
		if s.in.Config.MaxLines > 0 && s.rowCount+len(rows) >= s.in.Config.MaxLines {
			break
		}
		if errs[i] != nil {
			return errs[i]
		}

		fields, err := visibleFields(parsed[i], s.in.Config.ShowCols, s.in.Config.HideCols)
		if err != nil {
			return err
		}
		rows = append(rows, fields)
		bytesRead += int64(len(line) + 1) // +1 for the stripped newline
	}

	if err := s.b.appendRows(rows, s.in.Config.Strict); err != nil {
		return err
	}

	s.rowCount += len(rows)
	s.sinceR += len(rows)
	s.loaded += bytesRead
	if s.reportDue() {
		s.report()
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
