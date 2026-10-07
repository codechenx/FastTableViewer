package main

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/montanaflynn/stats"
)

type statsSummary interface {
	summary(a []string)
	getSummaryData() [][]string
	getSummaryStr(a []string) string
	getPlot() string
}

type DiscreteStats struct {
	data        []string
	summaryData [][]string
	count       int
	unique      int
	missing     int
	counter     map[string]int
}

type ContinuousStats struct {
	data        []float64
	summaryData [][]string
	count       int
	min         float64
	max         float64
	mean        float64
	median      float64
	sd          float64
	variance    float64
	sum         float64
	q1          float64
	q2          float64
	q3          float64
	iqr         float64
	mode        float64
	modeCount   int
	missing     int
}

func (s *ContinuousStats) summary(a []string) {
	originalCount := len(a)
	data := stats.LoadRawData(a)
	s.data = data
	s.count = len(data)
	s.missing = originalCount - s.count

	if s.count == 0 {
		s.summaryData = [][]string{{"Total values", I2S(originalCount)}, {"Missing/Invalid", I2S(s.missing)}, {"No numeric data", ""}}
		return
	}

	s.min, _ = stats.Min(data)
	s.max, _ = stats.Max(data)
	s.mean, _ = stats.Mean(data)
	s.median, _ = stats.Median(data)
	s.sd, _ = stats.StandardDeviation(data)
	s.variance, _ = stats.Variance(data)
	s.sum, _ = stats.Sum(data)

	q, _ := stats.Quartile(data)
	s.q1, s.q2, s.q3 = q.Q1, q.Q2, q.Q3
	s.iqr = s.q3 - s.q1

	// Calculate mode
	s.mode, s.modeCount = calculateMode(data)

	summaryArray := [][]string{
		{"Total values", I2S(originalCount)},
		{"Valid numbers", I2S(s.count)},
		{"Missing/Invalid", I2S(s.missing)},
		{"", ""},
		{"Min", formatStat(s.min)},
		{"Max", formatStat(s.max)},
		{"Range", formatStat(s.max - s.min)},
		{"Sum", formatStat(s.sum)},
		{"", ""},
		{"Mean", formatStat(s.mean)},
		{"Median", formatStat(s.median)},
		{"Mode", describeMode(s.mode, s.modeCount)},
		{"", ""},
		{"Std Dev", formatStat(s.sd)},
		{"Variance", formatStat(s.variance)},
		{"", ""},
		{"Q1 (25%)", formatStat(s.q1)},
		{"Q2 (50%)", formatStat(s.q2)},
		{"Q3 (75%)", formatStat(s.q3)},
		{"IQR", formatStat(s.iqr)},
	}
	s.summaryData = summaryArray
}

// calculateMode returns the most frequent value and how often it occurs. Ties
// go to the smallest value: picking whichever the map happened to yield first
// meant the reported mode changed between runs on the same data.
func calculateMode(data []float64) (float64, int) {
	if len(data) == 0 {
		return 0, 0
	}

	freq := make(map[float64]int)
	for _, v := range data {
		freq[v]++
	}

	var mode float64
	maxCount := 0
	for k, v := range freq {
		if v > maxCount || (v == maxCount && k < mode) {
			mode, maxCount = k, v
		}
	}
	return mode, maxCount
}

func (s *ContinuousStats) getSummaryData() [][]string {
	return s.summaryData
}

func (s *ContinuousStats) getSummaryStr(a []string) string {
	result := ""
	s.summary(a)
	summaryArray := s.getSummaryData()

	for _, i := range summaryArray {
		var n, v string
		n, v = i[0], i[1]
		result = result + "#" + n + " : " + v + "\n"
	}

	return result
}

// getPlot generates a histogram visualization for continuous data
func (s *ContinuousStats) getPlot() string {
	if len(s.data) == 0 {
		return "No data to plot"
	}

	// Create histogram bins
	numBins := 20
	if len(s.data) < 100 {
		numBins = 10
	}
	if len(s.data) < 20 {
		numBins = 5
	}

	// Calculate bin width
	binWidth := (s.max - s.min) / float64(numBins)
	if binWidth == 0 {
		return "All values are identical"
	}

	// Initialize bins
	bins := make([]float64, numBins)

	// Count values in each bin
	for _, v := range s.data {
		binIndex := int((v - s.min) / binWidth)
		if binIndex >= numBins {
			binIndex = numBins - 1
		}
		if binIndex < 0 {
			binIndex = 0
		}
		bins[binIndex]++
	}

	// Label each bin by the range it covers, so the axis is readable rather
	// than implied.
	decimals := 0
	if binWidth < 1 {
		decimals = 2
	}

	rows := make([]barRow, 0, numBins)
	total := float64(len(s.data))
	for i, count := range bins {
		low := s.min + float64(i)*binWidth
		rows = append(rows, barRow{
			label: strconv.FormatFloat(low, 'f', decimals, 64),
			value: count,
			note:  fmt.Sprintf("%d (%.0f%%)", int(count), count/total*100),
		})
	}

	return barChart(rows, chartBarWidth)
}

// kv pairs a value with how often it occurs.
type kv struct {
	Key   string
	Value int
}

// ranked returns the distinct values ordered by descending frequency, ties
// broken by value so repeated runs agree. The three places that needed this
// each built and sorted their own copy, and map order left ties unstable.
func (s *DiscreteStats) ranked() []kv {
	out := make([]kv, 0, len(s.counter))
	for k, v := range s.counter {
		out = append(out, kv{k, v})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Value != out[j].Value {
			return out[i].Value > out[j].Value
		}
		return out[i].Key < out[j].Key
	})
	return out
}

func (s *DiscreteStats) summary(a []string) {
	s.data = a
	s.count = len(a)

	// Count empty/missing values
	s.missing = 0
	s.counter = make(map[string]int)

	for _, row := range a {
		if row == "" {
			s.missing++
		}
		s.counter[row]++
	}

	s.unique = len(s.counter)

	ss := s.ranked()

	// Build summary with overview first
	s.summaryData = [][]string{
		{"Total values", I2S(s.count)},
		{"Unique values", I2S(s.unique)},
		{"Empty/Missing", I2S(s.missing)},
		{"", ""},
		{"Value", "Frequency"},
		{"─────", "─────────"},
	}

	// Add top values (limit to prevent excessive display)
	maxDisplay := 50
	for i, kv := range ss {
		if i >= maxDisplay {
			remaining := len(ss) - maxDisplay
			s.summaryData = append(s.summaryData, []string{"...", "(" + I2S(remaining) + " more)"})
			break
		}

		displayKey := kv.Key
		if displayKey == "" {
			displayKey = "(empty)"
		}
		// Truncate very long values
		if len(displayKey) > 40 {
			displayKey = displayKey[:37] + "..."
		}

		percent := float64(kv.Value) / float64(s.count) * 100
		s.summaryData = append(s.summaryData, []string{
			displayKey,
			I2S(kv.Value) + " (" + formatStat(percent) + "%)",
		})
	}
}
func (s *DiscreteStats) getSummaryData() [][]string {
	return s.summaryData
}

func (s *DiscreteStats) getSummaryStr(a []string) string {
	s.summary(a)
	summaryArray := s.getSummaryData()
	result := ""
	for _, i := range summaryArray {
		var n, v string
		n, v = i[0], i[1]
		result = result + "#" + n + " : " + v + "\n"
	}
	result = result + "----------\n" + "Top 20 variable\n\n"

	for _, kv := range s.ranked() {
		result = result + "#" + kv.Key + " : " + I2S(kv.Value) + "\n"
	}

	return result
}

// getPlot draws the most frequent values as bars.
func (s *DiscreteStats) getPlot() string {
	if len(s.counter) == 0 {
		return "No data to plot"
	}

	ranked := s.ranked()
	if len(ranked) > 15 {
		ranked = ranked[:15]
	}

	rows := make([]barRow, 0, len(ranked))
	total := float64(s.count)
	for _, kv := range ranked {
		label := kv.Key
		if label == "" {
			label = "(empty)"
		}
		rows = append(rows, barRow{
			label: label,
			value: float64(kv.Value),
			note:  fmt.Sprintf("%d (%.0f%%)", kv.Value, float64(kv.Value)/total*100),
		})
	}

	return barChart(rows, chartBarWidth)
}

// describeMode reports the most common value, or says plainly that there is no
// repeated value rather than presenting an arbitrary one as "the mode".
func describeMode(mode float64, count int) string {
	if count <= 1 {
		return "none (all distinct)"
	}
	return formatStat(mode) + " (×" + I2S(count) + ")"
}

const (
	// The chart has to fit the stats panel's right-hand pane, so its parts are
	// bounded rather than sized by their content.
	chartLabelWidth = 11
	chartBarWidth   = 16
)

// barRow is one labelled bar in a chart.
type barRow struct {
	label string
	value float64
	note  string
}

// barChart draws labelled horizontal bars, scaled to the largest value.
//
// Both plots used to go through asciigraph, which draws a *line* through the
// values: a frequency distribution came out as a near-flat line that said
// nothing. Bars are what a histogram and a value count actually are.
func barChart(rows []barRow, barWidth int) string {
	if len(rows) == 0 {
		return "No data to plot"
	}

	labelWidth, noteWidth, maxValue := 0, 0, 0.0
	for _, r := range rows {
		if n := runeCount(r.label); n > labelWidth {
			labelWidth = n
		}
		if n := runeCount(r.note); n > noteWidth {
			noteWidth = n
		}
		if r.value > maxValue {
			maxValue = r.value
		}
	}
	if labelWidth > chartLabelWidth {
		labelWidth = chartLabelWidth
	}
	if maxValue <= 0 {
		maxValue = 1
	}

	var out strings.Builder
	for _, r := range rows {
		label := truncateText(r.label, labelWidth)
		filled := int(r.value / maxValue * float64(barWidth))
		if filled == 0 && r.value > 0 {
			filled = 1
		}

		fmt.Fprintf(&out, "%-*s %s%s %*s\n",
			labelWidth, label,
			strings.Repeat("█", filled),
			strings.Repeat(" ", barWidth-filled),
			noteWidth, r.note)
	}
	return out.String()
}

// formatStat renders a statistic without trailing zeros, so a column of whole
// numbers does not report its minimum as "25.0000".
func formatStat(f float64) string {
	if f == math.Trunc(f) && math.Abs(f) < 1e15 {
		return strconv.FormatFloat(f, 'f', 0, 64)
	}
	s := strconv.FormatFloat(f, 'f', 4, 64)
	return strings.TrimSuffix(strings.TrimRight(s, "0"), ".")
}
