# Fast Table Viewer for Terminal

**A fast, feature-rich CSV/TSV/delimited file viewer for the command line**

[![Go Report Card](https://goreportcard.com/badge/github.com/codechenx/FastTableViewer)](https://goreportcard.com/report/github.com/codechenx/FastTableViewer)
![test](https://github.com/codechenx/FastTableViewer/workflows/test/badge.svg)
[![Coverage Status](https://coveralls.io/repos/github/codechenx/FastTableViewer/badge.svg?branch=main)](https://coveralls.io/github/codechenx/FastTableViewer?branch=main)
[![GitHub license](https://img.shields.io/github/license/codechenx/FastTableViewer.svg)](https://github.com/codechenx/FastTableViewer/blob/master/LICENSE)
[![GitHub release](https://img.shields.io/github/release/codechenx/FastTableViewer.svg)](http://GitHub.com/codechenx/FastTableViewer/releases)


<p align="center">
   <img src="data/icon_transparent.png"  style="width:200px;" alt="ftv icon"/>
</p>


## Demo

[![asciicast](https://asciinema.org/a/C1MPA6TB5h68NYYJXogCigSr2.svg)](https://asciinema.org/a/C1MPA6TB5h68NYYJXogCigSr2)

## Table of Contents

- [Features](#features)
- [Installation](#installation)
  - [Quick Install (Linux/macOS)](#quick-install-linuxmacos)
  - [Package Managers](#package-managers)
  - [Build from Source](#build-from-source)
- [Quick Start](#quick-start)
- [Command Line Flags](#command-line-flags)
- [Key Bindings](#key-bindings)
- [Features in Detail](#features-in-detail)
  - [Progressive Loading](#progressive-loading)
  - [Data Types and Sorting](#data-types-and-sorting)
  - [Statistics and Visualization](#statistics-and-visualization)
  - [Search](#search)
  - [Column Filter](#column-filter)
  - [Text Wrapping](#text-wrapping)
- [Filter Operators Guide](FILTER_OPERATORS.md)
- [Advanced Examples](#advanced-examples)
  - [Biological Data Formats](#biological-data-formats)

## Features

ftv brings spreadsheet-like functionality to your terminal with vim-inspired controls.

- **Spreadsheet interface** - Navigate and view tabular data with frozen headers
- **Smart parsing** - Automatically detects delimiters (CSV, TSV, custom separators)
- **Progressive loading** - Start viewing large files immediately while they load
- **Gzip support** - Read compressed files directly
- **Powerful search** - Find text across all cells with highlighting and regex pattern matching support
- **Advanced filtering** - Filter rows with complex regex queries
- **Flexible sorting** - Sort by any column with intelligent type detection
- **Text wrapping** - Wrap long cell content for better readability
- **Statistics & plots** - View column statistics with visual distribution charts
- **Vim keybindings** - Navigate naturally with h/j/k/l and more
- **Mouse support** - Click to select cells, scroll with mouse wheel, interact with dialogs
- **Pipe support** - Read from stdin for seamless integration with shell pipelines


## Installation

### Recommended: Install Script (Linux/macOS)

```bash
curl -sSL https://raw.githubusercontent.com/codechenx/FastTableViewer/master/install.sh | bash
sudo mv ftv /usr/local/bin/
```


### Package Managers

**macOS (homebrew)**

```bash
brew tap codechenx/tap
brew install codechenx-ftv
```

**Debian/Ubuntu (.deb)**

```bash
# Download from releases page
wget https://github.com/codechenx/FastTableViewer/releases/download/v0.9.1/FastTableViewer_0.9.1_linux_amd64.deb
sudo dpkg -i FastTableViewer_*.deb
```

**CentOS/Fedora (.rpm)**

```bash
# Download from releases page
wget https://github.com/codechenx/FastTableViewer/releases/download/v0.9.1/FastTableViewer_0.9.1_linux_amd64.rpm
sudo rpm -i FastTableViewer_*.rpm
```

**Arch Linux (AUR)**

```bash
yay -S ftv-bin
```

**Snap (Linux)**

[![Get it from the Snap Store](https://snapcraft.io/static/images/badges/en/snap-store-white.svg)](https://snapcraft.io/codechenx-tv)

```bash
sudo snap install codechenx-tv
sudo snap alias codechenx-tv ftv
```

**Go Install**

```bash
go install github.com/codechenx/FastTableViewer@latest
```


### Manual Download

Download pre-built binaries from [releases](https://github.com/codechenx/FastTableViewer/releases) for:
- Linux (x86_64, ARM, ARM64)
- macOS (Intel, Apple Silicon)
- Windows (x86_64, i386)

### Build from Source

Requires Go 1.21 or later:

```bash
git clone https://github.com/codechenx/FastTableViewer.git
cd FastTableViewer
go build -ldflags="-s -w" -o ftv
```

## Quick Start

View a CSV file:

```bash
ftv data.csv
```

View a TSV file (tab-separated):

```bash
ftv data.tsv
```

Read from stdin:

```bash
cat data.csv | ftv
ps aux | ftv
```

Specify a custom delimiter:

```bash
ftv data.txt -s "|"
```

View only specific columns:

```bash
ftv data.csv --columns 1,3,5
```

Skip header lines (e.g., for VCF files):

```bash
ftv file.vcf --skip-prefix "##"
```

## Command Line Flags

**Syntax:** `ftv [FILE] [flags]`

| Flag | Short | Description |
|------|-------|-------------|
| `--separator` | `-s` | Delimiter character (use `\t` for tab) |
| `--lines` | `-n` | Display only first N lines |
| `--skip-prefix` | | Skip lines starting with prefix (comma-separated) |
| `--skip-lines` | | Skip first N lines |
| `--columns` | | Show only specified columns (comma-separated) |
| `--hide-columns` | | Hide specified columns (comma-separated) |
| `--freeze` | `-f` | Freeze mode: `-1`=none, `0`=row+col, `1`=row only, `2`=col only |
| `--strict` | | Strict mode: fail on missing/inconsistent data |
| `--async` | | Progressive rendering while loading (default: `true`) |
| `--help` | `-h` | Show help |
| `--version` | `-v` | Show version |

**Examples:**

```bash
# Use custom delimiter
ftv data.txt -s ","

# View only columns 1, 3, and 5
ftv data.csv --columns 1,3,5

# Skip lines starting with "#"
ftv data.txt --skip-prefix "#"

# Disable header freezing
ftv data.csv -f -1

# Disable async loading for slow systems
ftv large.csv --async=false
```

## Key Bindings

ftv uses vim-inspired keybindings for intuitive navigation.

### Navigation

| Key | Action |
|-----|--------|
| `h` / `←` | Move left |
| `l` / `→` | Move right |
| `j` / `↓` | Move down |
| `k` / `↑` | Move up |
| `w` | Jump to next column |
| `b` | Jump to previous column |
| `gg` | Go to first row |
| `G` | Go to last row |
| `0` | Go to first column |
| `$` | Go to last column |
| `Ctrl-d` | Page down (half page) |
| `Ctrl-u` | Page up (half page) |

### Operations

| Key | Action |
|-----|--------|
| `/` | Search |
| `n` | Next search result |
| `N` | Previous search result |
| `Esc` | Clear search highlighting / Close dialogs |
| `f` | Filter by column |
| `r` | Remove filter for current column |
| `s` | Sort ascending |
| `S` | Sort descending |
| `t` | Toggle column type (String → Number → Date) |
| `W` | Toggle text wrapping |
| `i` | Show column statistics |
| `?` | Show help |
| `Esc` | Close dialogs / clear search |
| `q` | Quit |

### Mouse Support

| Action | Behavior |
|--------|----------|
| **Left Click** | Select cell at click position |
| **Scroll Wheel Up** | Scroll up one row |
| **Scroll Wheel Down** | Scroll down one row |
| **Click on Buttons** | Activate buttons in dialogs (Search, Filter, Stats) |
| **Click on Checkboxes** | Toggle checkboxes in forms (e.g., "Use Regex") |

**Note:** Mouse support works in most modern terminals. If your terminal doesn't support mouse events, you can still use keyboard navigation exclusively.

## Features in Detail

### Progressive Loading

Start viewing large files instantly without waiting for them to fully load. The UI appears immediately and updates as data streams in.

```bash
# Default behavior - UI appears instantly
ftv huge_dataset.csv

# Disable if you prefer traditional loading
ftv huge_dataset.csv --async=false
```

**Progress indicators:**
- Files show percentage: `Loading... 45.2%` → `Loaded 1,000,000 rows`
- Pipes show row count: `Loading... 5,234 rows` → `Loaded 10,000 rows`
- Updates at 50 FPS for smooth rendering

### Data Types and Sorting

tv automatically detects column types and provides intelligent sorting.

**Type Detection:** When loading data, ftv analyzes each column to determine if it contains strings, numbers, or dates using a 90% confidence threshold.

**Manual Type Toggle:** Press `t` to cycle through types for the current column:
- String → Number → Date → String

**Sorting Behavior:**
- **Strings:** Alphabetical order
- **Numbers:** Numeric order (supports integers, floats, scientific notation, thousands separators)
- **Dates:** Chronological order (supports ISO-8601, US format, EU format, and more)

### Statistics and Visualization

Analyze your data with comprehensive statistics and modern ASCII plots.

**View Statistics:** Press `i` on any column to open an interactive statistics dialog that displays:

**For numeric columns:**
- Summary stats: count, min, max, range, sum
- Central tendency: mean, median, mode
- Dispersion: standard deviation, variance
- Quartiles: Q1, Q2, Q3, and IQR
- **Visual distribution:** Histogram plot showing data distribution

**For categorical/string columns:**
- Total values, unique values, missing/empty count
- Frequency distribution with percentages
- **Visual distribution:** Bar chart of top 15 most frequent values

**Important:** When column filters are active, statistics are calculated **only on the filtered/visible data**, not the entire dataset. The dialog title will indicate when statistics are based on filtered data and show the number of active filters.

The statistics dialog features a split-pane layout with numerical stats on the left and an ASCII graph visualization on the right, powered by `asciigraph` for modern, clean plots.

### Search

Find text anywhere in your table with full highlighting support and powerful regex pattern matching.

**How to search:**

1. Press `/` to open the search dialog
2. Type your search term.
3. **Optional:** Press Tab to navigate to the checkboxes, then Space to enable:
    - **Use Regex**: for pattern matching with regular expressions.
    - **Case Sensitive**: for case-sensitive matching.
4. Press Enter to execute the search.
5. Navigate results with `n` (next) and `N` (previous).
6. Press `Esc` to clear highlighting.

**Search Modes:**

- **Plain Text (default):** Case-insensitive substring matching. Enable `Case Sensitive` for exact matching.
- **Regex:** Full regular expression support. By default, regex is case-insensitive. Enable `Case Sensitive` for case-sensitive regex matching.

**Navigation in Search Dialog:**
- Type your search query in the text field.
- Press `Tab` to move between the search field, checkboxes, and buttons.
- Press `Space` to toggle a checkbox when it is focused.
- Press `Enter` from anywhere in the form to execute the search.
- Press `Esc` to cancel and close the dialog.

**Visual feedback:**
- Current match: bright cyan highlight
- Other matches: gray highlight
- Footer shows position: `Match 3/12` or `regex matches 3/12`

**Example:**
```
# Simple text search (case-insensitive)
/ → type "error" → Enter → n → n → N → Esc

# Case-sensitive text search
/ → check "Case Sensitive" → type "Error" → Enter

# Regex search examples
/ → check "Use Regex" → type "^ERROR" → Enter     # Lines starting with ERROR (case-insensitive)
/ → check "Use Regex" and "Case Sensitive" → type "^Error" → Enter # Lines starting with Error (case-sensitive)
/ → check "Use Regex" → type "\\d{4}-\\d{2}-\\d{2}" → Enter     # Date patterns (YYYY-MM-DD)
/ → check "Use Regex" → type "user(name)?" → Enter           # Match "user" or "username"
/ → check "Use Regex" → type "error|warning|critical" → Enter # Match any of these words
/ → check "Use Regex" → type "@.*\\.(com|org)$" → Enter       # Email domains ending in .com or .org
```

**Common Regex Patterns:**

| Pattern | Description | Example Match |
|---------|-------------|---------------|
| `^start` | Match at beginning of cell | `^Error` matches "Error: failed" |
| `end$` | Match at end of cell | `\\.txt$` matches "file.txt" |
| `\\d+` | Match one or more digits | `\\d+` matches "123" |
| `\\w+@\\w+\\.\\w+` | Match email pattern | Matches "user@example.com" |
| `word1\\|word2` | Match either word (OR) | `success\\|complete` matches either |
| `[A-Z]+` | Match uppercase letters | `[A-Z]{3}` matches "USA" |
| `.*` | Match any characters | `start.*end` matches "start...end" |
| `\\s+` | Match whitespace | `\\s{2,}` matches 2+ spaces |

**Note:** For case-insensitive regex search, the `(?i)` flag is automatically added to your pattern. For case-sensitive regex, this flag is omitted.

### Column Filter

Show only rows where specific columns match your criteria. **Supports filtering on multiple columns simultaneously**.

**How to filter:**

1. Navigate to the column you want to filter
2. Press `f` to open the filter dialog
3. Use the dropdown to select an operator (e.g., `contains`, `equals`, `regex`, `>`).
4. Enter the value to filter by.
5. Optionally, check the `Case Sensitive` box.
6. Press Enter to apply the filter.
7. **Repeat on other columns to add more filters**
8. Press `r` on a filtered column to remove that column's filter

**Multi-Column Filtering:**

- Apply filters to **multiple columns** by pressing `f` on each column and entering criteria
- All filters are combined with AND logic (rows must match all active filters)
- Each column can have different filter criteria including operators
- Press `f` on a filtered column to edit or remove its filter (empty query removes the filter)
- The footer shows how many filters are active
- Filtered column headers display 🔎 icons with orange background

**Operators:**

| Operator | Description |
|---|---|
| `contains` | Matches cells containing the term |
| `equals` | Matches cells that are exactly the term |
| `starts with` | Matches cells that start with the term |
| `ends with` | Matches cells that end with the term |
| `regex` | Matches cells based on a regular expression |
| `>` | Numeric: greater than (number columns only) |
| `<` | Numeric: less than (number columns only) |
| `>=` | Numeric: greater than or equal (number columns only) |
| `<=` | Numeric: less than or equal (number columns only) |

**Key Features:**
- **Numeric operators** (`>`, `<`, `>=`, `<=`): Only work on numeric and date columns (automatically detected). Perform numeric comparisons instead of text matching.
- **Regex**: Provides the full power of regular expressions for complex pattern matching.
- **Case-Insensitive by default**: All string-based comparisons are case-insensitive unless the `Case Sensitive` box is checked.
- **Visual indicator**: Filtered column headers show 🔎 icons and an orange background

**Examples:**

```bash
# Simple filter - partial match
Navigate to "Status" column → f → select 'contains' → type "pending" → Enter
# Result: Matches "Pending", "Pending Review", etc.

# Exact match filter
Navigate to "Status" column → f → select 'equals' → type "active" → Enter
# Result: Rows where Status is exactly "active" (case-insensitive)

# Regex filter
Navigate to "Email" column → f → select 'regex' → type "^.+@gmail\.com$" → Enter
# Result: Rows where Email ends with "@gmail.com"

# Numeric comparison - greater than
Navigate to "Age" column → f → select '>' → type "30" → Enter
# Result: Rows where Age is greater than 30

# Multi-column filtering
Navigate to "City" column → f → select 'equals' → type "New York" → Enter
Navigate to "Department" column → f → select 'contains' → type "Engineering" → Enter
# Result: Rows where City is "New York" AND Department contains "Engineering"

# Edit existing filter
Navigate to filtered column → f → modify operator/value → Enter
# Or enter empty text to remove that column's filter

# Remove a specific filter
Navigate to filtered column → Press r
# Result: That column's filter removed, other filters remain
```

**Visual Feedback:**
- When a filter is active, the filtered column header displays 🔎 icons and an orange background
- A dedicated **filter strip** appears above the main footer showing the active filter on the current column.
- Filter strip format: `🔎 Filter Active: [Column Name] [operator] "query"  |  Press 'r' to clear`
- The strip automatically hides when you move to a different column
- Press `r` to clear the filter and return to normal view


### Text Wrapping

Handle long cell content without horizontal scrolling.

**How to wrap:**
- Press `W` (capital W) on any column to toggle wrapping
- First press: wraps at 25 characters with smart word breaks
- Second press: unwraps back to single line

**Smart wrapping:**
- Breaks at spaces and hyphens when possible
- Each column can be wrapped independently
- Useful for comments, descriptions, URLs

## Advanced Examples

### Biological Data Formats

ftv handles common bioinformatics file formats with comment/header prefixes.

**VCF files:**
```bash
# Skip VCF metadata lines
ftv sample.vcf --skip-prefix "##"
ftv sample.vcf.gz --skip-prefix "##"
```

**QIIME OTU tables:**
```bash
ftv otu_table.txt --skip-prefix "# "
```

**MAF (Mutation Annotation Format):**
```bash
ftv mutations.maf --skip-prefix "#"
```

**Interval lists:**
```bash
# Skip SAM header lines
ftv intervals.interval_list --skip-prefix "@HD","@SQ"

# Or skip all @ lines
ftv intervals.interval_list --skip-prefix "@"
```

**BED files with headers:**
```bash
ftv peaks.bed --skip-prefix "track","browser"
```

### General Examples

**Large log files:**
```bash
# View first 1000 lines only
ftv app.log -n 1000

# Skip timestamp lines
ftv app.log --skip-prefix "2024"
```

**CSV with specific columns:**
```bash
# Show only columns 1, 3, and 5
ftv data.csv --columns 1,3,5

# Hide sensitive columns 2 and 4
ftv data.csv --hide-columns 2,4
```

**Pipeline integration:**
```bash
# View process list
ps aux | ftv

# View git log as table
git log --pretty=format:"%h,%an,%ar,%s" | ftv -s ","

# Parse JSON with jq, view as table
cat data.json | jq -r '.[] | [.id, .name, .value] | @csv' | ftv
```

**Custom delimiters:**
```bash
# Pipe-separated
ftv data.txt -s "|"

# Semicolon-separated
ftv data.txt -s ";"

# Multiple spaces
ftv data.txt -s "  "
```

---

## Tips and Tricks

- **Large files?** Let async loading work its magic - the UI appears instantly
- **Can't find data?** Use `/` to search across all cells
- **Too many columns?** Use `--columns` to show only what you need
- **Long text?** Press `W` to wrap the current column
- **Wrong sort order?** Press `t` to change the column type, then `s` to re-sort
- **Complex filtering?** Use `OR` for alternatives, `AND` for requirements, `ROR` to combine results
- **Need insights?** Press `i` for comprehensive statistics with visual plots - histograms for numeric data, frequency charts for categorical data
- **Prefer mouse?** Click cells to select them, use scroll wheel to navigate, and click buttons in dialogs

## License

Apache License - see [LICENSE](LICENSE) file for details.
