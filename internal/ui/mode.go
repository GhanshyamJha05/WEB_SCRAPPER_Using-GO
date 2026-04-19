// Package ui handles all terminal output for the goscraper CLI.
// Nothing outside this package should call fmt.Print directly.
package ui

// Mode controls how much output is printed.
type Mode int

const (
	ModeNormal  Mode = iota // default: progress + summary
	ModeQuiet               // errors and final save line only
	ModeVerbose             // normal + per-URL timing + retry info
)

// Active is the output mode for the current run.
// Set it once in the command's PersistentPreRun before any printing.
var Active Mode = ModeNormal
