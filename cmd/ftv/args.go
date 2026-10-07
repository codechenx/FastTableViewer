package main

// Args struct
type Args struct {
	FileName   string
	Sep        string
	SkipSymbol []string //ignore line with specified prefix
	SkipNum    int      //Number of lines that should be skipped
	ShowNum    []int    //columns that should be displayed
	HideNum    []int    //columns that should be hidden
	Header     int      //header display mode
	NLine      int      //number of lines that should be displayed
	Strict     bool     // check for missing data
	AsyncLoad  bool     // enable async loading for progressive rendering
}

func (args *Args) setDefault() {
	args.Sep = ""
	args.SkipSymbol = []string{}
	args.SkipNum = 0
	args.ShowNum = []int{}
	args.HideNum = []int{}
	args.Header = 0
	args.NLine = 0
	args.Strict = false
	args.AsyncLoad = true // default to async loading
}

// intakeConfig snapshots the CLI flags as the immutable rules for one load.
// Intake never writes to the result, so loading twice behaves the same twice;
// the loaders used to decrement SkipNum on the shared Args as they read.
func (args Args) intakeConfig() IntakeConfig {
	sep := args.Sep
	if sep == "\\t" {
		sep = "\t"
	}
	var r rune
	if runes := []rune(sep); len(runes) > 0 {
		r = runes[0]
	}

	return IntakeConfig{
		Sep:        r,
		SkipPrefix: args.SkipSymbol,
		SkipLines:  args.SkipNum,
		MaxLines:   args.NLine,
		ShowCols:   args.ShowNum,
		HideCols:   args.HideNum,
		Strict:     args.Strict,
	}
}
