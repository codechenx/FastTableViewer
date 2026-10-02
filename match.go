package main

import (
	"regexp"
	"strings"
)

// Match operators. The text operators apply to every column; the comparison
// operators need a column whose type can order its values.
const (
	opContains   = "contains"
	opEquals     = "equals"
	opStartsWith = "starts with"
	opEndsWith   = "ends with"
	opRegex      = "regex"
	opGT         = ">"
	opLT         = "<"
	opGTE        = ">="
	opLTE        = "<="
)

// matchOperators is the list offered in the filter dialog, in display order.
var matchOperators = []string{
	opContains, opEquals, opStartsWith, opEndsWith, opRegex,
	opGT, opLT, opGTE, opLTE,
}

// MatchSpec is a match request: what to look for, how to compare it, and
// whether case matters. Search and filter are both expressed in these terms,
// so there is one answer to "does this cell match" rather than two.
type MatchSpec struct {
	Query         string
	Operator      string // "" is treated as opContains
	CaseSensitive bool
}

// FilterOptions is the parameters of a column filter. A filter is a match
// spec, so this is the same type under the name the UI uses.
type FilterOptions = MatchSpec

// Matcher answers "does this cell match" for one compiled MatchSpec. Building
// it once keeps regex compilation and query case folding out of the per-cell
// path, which previously recompiled the pattern for every cell examined.
type Matcher struct {
	test func(cell string) bool
}

// isComparison reports whether op orders values rather than comparing text.
func isComparison(op string) bool {
	switch op {
	case opGT, opLT, opGTE, opLTE:
		return true
	}
	return false
}

// CompileMatcher builds a Matcher for spec against a column of the given type.
//
// Comparison operators use the column type's own parser, so a date column
// compares dates and a numeric column compares numbers; a cell that carries no
// value never satisfies a comparison. On a column type that cannot order its
// values, a comparison operator falls back to a text contains.
//
// An invalid regex is returned as an error rather than silently matching
// nothing, leaving the caller free to say so.
func CompileMatcher(spec MatchSpec, colType ColumnType) (Matcher, error) {
	operator := spec.Operator
	if operator == "" {
		operator = opContains
	}

	typeSpec := columnTypes[colType]

	if isComparison(operator) && typeSpec.parse != nil {
		threshold, ok := typeSpec.parse(spec.Query)
		if !ok {
			// The query is not a value this column can be compared against.
			return Matcher{test: func(string) bool { return false }}, nil
		}
		return Matcher{test: func(cell string) bool {
			v, ok := typeSpec.parse(cell)
			if !ok {
				return false
			}
			switch operator {
			case opGT:
				return v > threshold
			case opLT:
				return v < threshold
			case opGTE:
				return v >= threshold
			case opLTE:
				return v <= threshold
			}
			return false
		}}, nil
	}

	if operator == opRegex {
		pattern := spec.Query
		if !spec.CaseSensitive {
			pattern = "(?i)" + pattern
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			return Matcher{}, err
		}
		return Matcher{test: re.MatchString}, nil
	}

	// Text operators. Fold the query once; fold each cell as it is examined.
	query := spec.Query
	if !spec.CaseSensitive {
		query = strings.ToLower(query)
	}
	fold := func(cell string) string { return cell }
	if !spec.CaseSensitive {
		fold = strings.ToLower
	}

	switch operator {
	case opEquals:
		return Matcher{test: func(cell string) bool { return fold(cell) == query }}, nil
	case opStartsWith:
		return Matcher{test: func(cell string) bool { return strings.HasPrefix(fold(cell), query) }}, nil
	case opEndsWith:
		return Matcher{test: func(cell string) bool { return strings.HasSuffix(fold(cell), query) }}, nil
	default: // opContains, and comparisons on an unordered column
		return Matcher{test: func(cell string) bool { return strings.Contains(fold(cell), query) }}, nil
	}
}

// MatchCell reports whether cell satisfies the compiled spec. A zero Matcher
// matches nothing.
func (m Matcher) MatchCell(cell string) bool {
	if m.test == nil {
		return false
	}
	return m.test(cell)
}
