package main

import (
	"strconv"
	"strings"
	"time"
	"unicode"
)

// ColumnType identifies how a column's values are interpreted when detecting,
// ordering and summarising them.
type ColumnType int

const (
	colTypeStr ColumnType = iota
	colTypeFloat
	colTypeDate
)

// dateFormats is the one list of layouts the viewer recognises, most common
// first. Detection and ordering both read it, so they cannot disagree.
var dateFormats = []string{
	"2006-01-02",          // ISO date: 2024-10-17
	"2006-01-02 15:04:05", // ISO datetime: 2024-10-17 15:30:00
	"01/02/2006",          // US date: 10/17/2024
	"02/01/2006",          // EU date: 17/10/2024
	"2006/01/02",          // Alt ISO: 2024/10/17
	time.RFC3339,          // RFC3339: 2024-10-17T15:30:00Z
	"2006-01-02T15:04:05", // ISO8601 without timezone
	"Jan 02, 2006",        // Mon DD, YYYY
	"January 02, 2006",    // Month DD, YYYY
	"02-Jan-2006",         // DD-Mon-YYYY
	"02 Jan 2006",         // DD Mon YYYY
	"2006.01.02",          // Dotted date
}

// missingValues are the spellings treated as "no value" by every type. The
// buffer itself writes "NaN" when padding short rows, so it belongs here.
var missingValues = map[string]bool{
	"":     true,
	"NA":   true,
	"N/A":  true,
	"NaN":  true,
	"null": true,
}

// isMissing reports whether s carries no value.
func isMissing(s string) bool {
	return missingValues[s]
}

// columnTypeSpec is everything a column type knows about itself: what to call
// it, whether a value belongs to it, and how to order its values.
type columnTypeSpec struct {
	name string
	// parse reports whether s belongs to this type, and returns the key used
	// to order it. A nil parse means the type orders lexically on the raw
	// string and accepts every value.
	parse func(s string) (float64, bool)
}

// columnTypes is the registry. Adding a type means adding one entry here;
// detection, ordering and the type-cycling key all read from it.
var columnTypes = map[ColumnType]columnTypeSpec{
	colTypeStr:   {name: "Str"},
	colTypeFloat: {name: "Num", parse: parseNumeric},
	colTypeDate:  {name: "Date", parse: parseDateKey},
}

// detectionOrder is the priority used when a column's values satisfy more than
// one type. Dates are more specific than numbers, so they win.
var detectionOrder = []ColumnType{colTypeDate, colTypeFloat}

// typeCycle is the order the 't' key walks through types.
var typeCycle = []ColumnType{colTypeStr, colTypeFloat, colTypeDate}

// type2name returns a column type's display name.
func type2name(t ColumnType) string {
	if spec, ok := columnTypes[t]; ok {
		return spec.name
	}
	return columnTypes[colTypeStr].name
}

// nextColumnType returns the type that follows t in the cycle.
func nextColumnType(t ColumnType) ColumnType {
	for i, c := range typeCycle {
		if c == t {
			return typeCycle[(i+1)%len(typeCycle)]
		}
	}
	return colTypeStr
}

// parseNumeric reports whether s is a number and returns its value. It is the
// single authority: detection asks whether ok, ordering uses the value, so a
// value can never be detected as numeric but ordered as something else.
//
// Underscores are accepted anywhere as digit separators. Commas must form
// valid thousands groups, so "1,234.56" parses and "1,2,3" does not.
func parseNumeric(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if isMissing(s) {
		return 0, false
	}

	if strings.ContainsAny(s, ",_") {
		normalised, ok := stripDigitSeparators(s)
		if !ok {
			return 0, false
		}
		s = normalised
	}

	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// stripDigitSeparators removes underscores and thousands commas, rejecting
// comma placement that is not a valid grouping.
func stripDigitSeparators(s string) (string, bool) {
	s = strings.ReplaceAll(s, "_", "")
	if !strings.Contains(s, ",") {
		return s, true
	}

	// Commas are only meaningful in the integer part, so split the exponent
	// and fraction off first and require them comma-free.
	intPart := s
	rest := ""
	if i := strings.IndexAny(s, ".eE"); i >= 0 {
		intPart, rest = s[:i], s[i:]
	}
	if strings.Contains(rest, ",") {
		return "", false
	}

	sign := ""
	if strings.HasPrefix(intPart, "+") || strings.HasPrefix(intPart, "-") {
		sign, intPart = intPart[:1], intPart[1:]
	}

	groups := strings.Split(intPart, ",")
	if len(groups) < 2 {
		return "", false
	}
	for i, g := range groups {
		if !allDigits(g) {
			return "", false
		}
		if i == 0 {
			if len(g) < 1 || len(g) > 3 {
				return "", false
			}
		} else if len(g) != 3 {
			return "", false
		}
	}

	return sign + strings.Join(groups, "") + rest, true
}

// hasLetter reports whether s contains any letter.
func hasLetter(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

// allDigits reports whether s is non-empty and entirely ASCII digits.
func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// parseDate reports whether s is a date and returns its unix timestamp.
func parseDate(s string) (int64, bool) {
	s = strings.TrimSpace(s)
	if isMissing(s) {
		return 0, false
	}

	// Cheap rejection before trying twelve layouts: every layout carries either
	// a separator or a month name. Checking for a letter rather than a comma
	// keeps "02 Jan 2006" eligible, which the table lists but the old guard
	// rejected, while still rejecting bare digit runs like "20241017".
	if !strings.ContainsAny(s, "-/.:T") && !hasLetter(s) {
		return 0, false
	}

	for _, format := range dateFormats {
		if t, err := time.Parse(format, s); err == nil {
			return t.Unix(), true
		}
	}
	return 0, false
}

// parseDateKey adapts parseDate to the ordering key signature.
func parseDateKey(s string) (float64, bool) {
	ts, ok := parseDate(s)
	return float64(ts), ok
}

// isNumericValue reports whether s is a number.
func isNumericValue(s string) bool {
	_, ok := parseNumeric(s)
	return ok
}

// isDateValue reports whether s is a date.
func isDateValue(s string) bool {
	_, ok := parseDate(s)
	return ok
}

// parseNumericValueFast returns s as a float64, or 0 when s is not a number.
func parseNumericValueFast(s string) float64 {
	v, _ := parseNumeric(s)
	return v
}

// parseDateValueFast returns s as a unix timestamp, or 0 when s is not a date.
func parseDateValueFast(s string) int64 {
	ts, _ := parseDate(s)
	return ts
}
