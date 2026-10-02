# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Toolchain

`go` is **not on the default PATH**. The Go toolchain is provisioned by [pixi](https://pixi.sh) (`pixi.toml`, go 1.25.4). Prefix commands with `pixi run`, or call `./.pixi/envs/default/bin/go` directly.

```sh
pixi run go build -o ftv .          # build
pixi run go test ./...              # all tests (~0.3s, 120 test funcs)
pixi run go test -race ./...        # what `make test` runs
pixi run go test -run TestIntake_PreservesRowOrder -v ./...   # single test
pixi run go vet ./...
pixi run gofmt -l .                 # must print nothing
```

`make build|test|check|clean|install|snapshot` wrap the same thing but call bare `go`, so they need pixi's env active (`pixi shell`) or a system Go. `make lint` requires `golangci-lint`, which is not installed locally — CI (`.github/workflows/linter.yml`) runs it on every push.

CI tests against Go 1.21–1.25, so avoid language/stdlib features newer than 1.21.

Test fixtures live in `data/test/` (CSV, TSV, gzip, pipe-delimited, quoted, ragged, empty, header-only). Prefer adding a fixture there over constructing files in test code.

## Architecture

Single flat `package main`, ~3.9k lines across 13 non-test files, no subpackages. Work flows through five modules, each with a small interface and the complexity behind it.

```
ftv.go ──► Intake ──► Buffer ◄── ViewState ◄── ui.go
(CLI)    (intake.go)  (buffer.go)  (viewstate.go)    │
             │            │            │             └─ bufferContent
          Source       ColumnType    Matcher            (content.go)
       (io.go adapters) (coltype.go) (match.go)
```

### Intake — loading (`intake.go`)

**One load path.** `Intake{Source, Config, OnProgress}.Into(buffer)` is the only way data enters. Progressive rendering is the `OnProgress` callback, not a second implementation: `ftv.go`'s `loadProgressively` runs `Into` in a goroutine and refreshes on a 20 ms ticker, while `loadAndShow` passes a console printer and blocks. Do not add a parallel loader — the four that used to exist drifted from each other.

- **`Source`** is the seam, with adapters `openFileSource` (handles `.gz`, owns and closes the handle) and `pipeSource`; tests add a third via `loadReaderInto`. `Source.Name` supplies the extension hint and is empty for streams.
- **`IntakeConfig`** is an immutable snapshot built by `Args.intakeConfig()`. Intake never writes to it, so loading twice behaves identically.
- **Parsing preserves input order.** Lines are batched (`intakeBatchSize`), parsed in parallel across up to 8 workers, then appended in input order. The previous pipeline fed a worker pool and appended in *completion* order, scrambling most rows of every file — `TestIntake_PreservesRowOrder*` guards against regressing this.
- **Separator resolution:** explicit `Config.Sep` → `.csv`/`.tsv` suffix → `sepDetect` over the first 10 surviving lines → error. `sepDetecor.sepDetect` in `sepDector.go` (the typo is in both the filename and the type) is deep: `[]string → rune` over ~200 lines of scored heuristics.
- `lineFeed` applies the skip rules once for every source, with its own countdown. It reads `scanner.Bytes()` and matches skip prefixes against precomputed `[][]byte`, so filtering a line allocates nothing.
- **A batch costs a handful of allocations, not one per row.** Lines accumulate as bytes and become a single string per batch, from which each line and then each field is a substring; each worker carves its rows' `[]string` from one block. Rows come back with `cap == len` deliberately, so widening one (`resizeColUnsafe` padding a short row) reallocates instead of writing over the next row's storage. Changing this is how you reintroduce 1.2M allocations per load.
- **Progress is reported on a clock as well as per N rows**, and a partial batch is flushed when a report is due. A row-count trigger alone goes silent on a slow stream — a pipe delivering fewer rows than the interval never reports, and never renders, until its producer closes.

### Buffer — the table (`buffer.go`)

`Buffer` holds `cont [][]string` plus per-column types and freeze counts. **Every method takes the lock it needs**, so the loading goroutine and the tview event loop can both hold one; callers never need to know which operations are concurrency-safe. Keep it that way when adding methods, and never call one locked method from inside another (`sync.RWMutex` is not reentrant — read fields directly instead, as `SortBy` and `filterByColumn` do).

`SortBy(col, desc)` is the only sort entry point. It looks up the column's type and orders accordingly, parsing each cell once, so nothing outside switches on type.

`appendRows` takes the lock once for a whole batch and `reserveRows` sizes the row index up front from the source's size; `contAppendSli` remains for single rows. Taking the write lock per row, and letting the index grow by appending, together accounted for most of a large load's cost.

### ViewState — what is on screen (`viewstate.go`)

Owns the visible buffer, the filter set, the search and its match set, width-limited columns, and the cursor column. Every change goes through an intent method (`ApplyFilter`, `ClearFilter`, `Search`, `CycleColType`, `ToggleWrap`, `SortBy`) and each leaves derived state consistent, so the table, the highlighting and the footer cannot disagree.

- Filtering **swaps the buffer**: `original` keeps the unfiltered data, and any filter change rebuilds from it by chaining `filterByColumn` over the active filters in column order. `Visible()` is what the table renders.
- Changing a filter re-runs the search, because match coordinates index the visible buffer.
- A filter matching nothing is discarded rather than blanking the view.
- **Reached only from the tview event loop** (key handlers and `QueueUpdateDraw` callbacks), never from the loading goroutine, so it carries no lock of its own.

`drawUI()` deliberately takes **no buffer parameter**. It used to, and that parameter shadowed a package-level `b` for the whole function body, so reassigning it on filter left the top-level footer helpers reading the old buffer. Don't reintroduce a local named `b` in `ui.go`.

### ColumnType — interpretation (`coltype.go`)

`colTypeStr|colTypeFloat|colTypeDate`, with one registry entry per type in `columnTypes` carrying its display name and its parser. Detection, ordering and filter comparisons all go through that same parser, so they cannot disagree.

- `parseNumeric` and `parseDate` are the single authorities; `isNumericValue`/`parseNumericValueFast`/`isDateValue`/`parseDateValueFast` are thin views on them.
- `dateFormats` is the one layout table. `parseDate`'s fast-path guard admits separators *or* letters, so month-name layouts stay eligible.
- Commas must form valid thousands groups (`1,234.56` parses, `1,2,3` does not); underscores are free-form digit separators.
- Detection samples (≤100 rows: all; otherwise first 50 + middle 25 + last 25), needs 90% agreement, and prefers `detectionOrder` (Date before Number). `t` walks `typeCycle`.

Adding a type means adding one registry entry plus its place in `detectionOrder`/`typeCycle`.

### Matcher — "does this cell match" (`match.go`)

`CompileMatcher(spec, colType)` compiles a `MatchSpec` once; `MatchCell` answers per cell. Both search (`performSearch` in `utils.go`) and filter (`Buffer.filterByColumn`) use it, so case folding and regex semantics are defined in one place. `FilterOptions` is a type alias of `MatchSpec` — a filter *is* a match spec.

Comparison operators (`>`, `<`, `>=`, `<=`) use the column type's parser, so date columns compare dates and a cell carrying no value never satisfies a comparison. On a string column they fall back to a text contains. An invalid pattern is an error, not a silent empty result.

### Stats (`stats.go`)

`statsSummary` has two adapters, `ContinuousStats` and `DiscreteStats`. Its interface carries an **ordering constraint**: `summary(col)` populates internal fields, so `getSummaryData()` and `getPlot()` return empty/"No data to plot" if called first. `ui.go` calls `summary` before `showStatsDialog`. This is the one module not yet deepened.

### UI (`ui.go`)

`drawUI()` runs once after the first rows land and wires everything: it builds `bufferTable` and installs one large `SetInputCapture` closure holding every key binding, plus mouse and selection handlers. Search and filter modals are built inline there.

**The table is virtual.** `bufferContent` in `content.go` implements tview's `TableContent`, so cells are built only as they are painted, and it is the single place header styling, match highlighting, filter markers and truncation are decided. There is deliberately no full-table painter: materialising every cell cost roughly 1.7KB per row, so a 48MB file needed about 2GB and a third of a second per repaint, and the UI could not keep up with a load. Nothing needs to repaint after a state change — tview redraws after each event and pulls what it needs.

While loading, `refreshWhileLoading` repaints only the footer, on a 20 ms ticker, showing a determinate bar when the source size is known and a spinner with a row tally when it is not (a stream or a `.gz`, where no honest percentage exists). `buildLoadingStatus` renders it and `drawFooter` is the one footer renderer — the loading path used to rebuild the footer itself, in a different palette and without the filter strip.

`init.go` holds what is left at package level: the tview handles, `view`, `args`, `debug`, `loadProgress`, the three footer strings, and the input state for recognising `gg` (a timestamp, checked on the event loop — the flag it replaced was cleared by a timer goroutine).

## Testing conventions

Table-driven subtests throughout, plus `regression_test.go`, which holds one test per defect fixed during the module work, grouped by the module it belongs to. When fixing a bug here, add its case there.

- `createNewBufferWithData` returns an independent buffer. It used to assign the package-level one, so fixtures clobbered each other.
- `ftv_test.go` invokes `main()` for real. It passes because stdin is a character device under `go test` (terminal or `/dev/null`), so it hits the no-args help branch. Piping into `go test` sends it down the pipe loader instead. The `debug` global that guards `app.Run()` and `fatalError`'s `os.Exit(1)` is declared but never set true, so `fatalError` will terminate the test binary.
- The TUI itself has no test coverage. To check it by hand, drive the built binary through a pty (`python3 -c` with the `pty` module), set a window size, then send keys — `go test` cannot reach `drawUI`.

## Known remaining rough edges

Deliberately left alone; don't treat them as accidents:

- `stats.go`'s four-method interface and its ordering constraint (above).
- Symbols referenced only by their own tests: `selectBySearch`, `resizeCol`, `usefulInfo`, `S2F`, `uniqueChar`, `allIntItemEqual`, `getSummaryStr`.
- `lineFeed.next` keeps a `line == "\n"` check that can never fire, because `bufio.ScanLines` strips the newline. Genuinely blank lines therefore become rows. Preserved to avoid silently changing output; remove it only as a deliberate behaviour change.
- `sepDetecor` is a stateless empty struct, so its methods could be plain functions.

## Release

goreleaser (`.goreleaser.yml`) on tag push, targeting Homebrew, AUR, snap and PKGBUILD. `make version` rewrites the version across `ftv.go`, `README.md`, `snap/snapcraft.yaml` and `PKGBUILD` — the only sanctioned way to bump, since the version is hardcoded in `main`'s cobra command.

## Agent skills

### Issue tracker

Issues live in GitHub Issues at `codechenx/FastTableViewer`, via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Triage labels

The five canonical roles, each label string equal to its name. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: one `CONTEXT.md` plus `docs/adr/` at the repo root. See `docs/agents/domain.md`.
