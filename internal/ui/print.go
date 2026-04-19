package ui

import (
	"fmt"
	"os"
	"strings"
)

const labelWidth = 10 // width of the left-hand label column

// Header prints the command name and a separator line.
//
//	goscraper run
//	─────────────
func Header(command string) {
	fmt.Fprintf(os.Stderr, "\n  goscraper %s\n", command)
	fmt.Fprintf(os.Stderr, "  %s\n\n", strings.Repeat("─", len("goscraper ")+len(command)))
}

// Config prints a key/value pair in the config block.
//
//	input      urls.txt
func Config(key, value string) {
	if Active == ModeQuiet {
		return
	}
	fmt.Fprintf(os.Stderr, "  %-*s %s\n", labelWidth, key, value)
}

// Spacer prints a blank line — used between config block and progress.
func Spacer() {
	if Active == ModeQuiet {
		return
	}
	fmt.Fprintln(os.Stderr)
}

// Section prints a section label (e.g. "fetching").
//
//	fetching
func Section(label string) {
	if Active == ModeQuiet {
		return
	}
	fmt.Fprintf(os.Stderr, "  %s\n\n", label)
}

// Progress prints one [n/total] progress line.
// durationMs is only shown in verbose mode (pass 0 to omit).
//
//	[1/3]  https://example.com    30 results
//	[1/3]  https://example.com    30 results  (120ms)   ← verbose
func Progress(n, total int, url string, count int, durationMs int64, err error) {
	if Active == ModeQuiet {
		return
	}

	counter := fmt.Sprintf("[%d/%d]", n, total)

	if err != nil {
		fmt.Fprintf(os.Stderr, "  %-8s  %-50s  error: %s\n", counter, truncate(url, 50), shortErr(err))
		return
	}

	line := fmt.Sprintf("  %-8s  %-50s  %d results", counter, truncate(url, 50), count)
	if Active == ModeVerbose && durationMs > 0 {
		line += fmt.Sprintf("  (%dms)", durationMs)
	}
	fmt.Fprintln(os.Stderr, line)
}

// Done prints the summary line after all URLs are processed.
//
//	done  55 results  1 error  (1.2s)
func Done(results, errors int, elapsed float64) {
	if Active == ModeQuiet {
		return
	}
	fmt.Fprintln(os.Stderr)

	errPart := ""
	if errors > 0 {
		errPart = fmt.Sprintf("  %d error", errors)
		if errors > 1 {
			errPart += "s"
		}
	}
	fmt.Fprintf(os.Stderr, "  done  %d results%s  (%.1fs)\n", results, errPart, elapsed)
}

// Saved prints the output file confirmation.
//
//	saved  out.json
func Saved(path string) {
	fmt.Fprintf(os.Stderr, "\n  saved  %s\n\n", path)
}

// Error prints a short error message and exits with code 1.
//
//	error: failed to read input file
func Error(msg string) {
	fmt.Fprintf(os.Stderr, "\n  error: %s\n\n", msg)
}

// Fatal prints an error and exits 1.
func Fatal(msg string) {
	Error(msg)
	os.Exit(1)
}

// Warn prints a warning only in verbose mode.
func Warn(msg string) {
	if Active != ModeVerbose {
		return
	}
	fmt.Fprintf(os.Stderr, "  warn:  %s\n", msg)
}

// Links prints a numbered list of discovered links for interactive selection.
//
//	Available links
//
//	 1  /about
//	 2  /products
//	 3  /contact
func Links(links []string) {
	fmt.Fprintf(os.Stderr, "\n  Available links\n\n")
	for i, l := range links {
		fmt.Fprintf(os.Stderr, "  %3d  %s\n", i+1, l)
	}
	fmt.Fprintln(os.Stderr)
}

// Prompt prints an input prompt and returns the trimmed user input.
func Prompt(label string) string {
	fmt.Fprintf(os.Stderr, "  %s: ", label)
	var input string
	fmt.Fscan(os.Stdin, &input)
	return strings.TrimSpace(input)
}

// --- helpers ---

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

func shortErr(err error) string {
	msg := err.Error()
	// strip the URL prefix that scraper adds ("https://...: actual error")
	if i := strings.Index(msg, ": "); i != -1 {
		msg = msg[i+2:]
	}
	// keep only the first sentence / clause
	for _, sep := range []string{"\n", " (", ": dial"} {
		if i := strings.Index(msg, sep); i != -1 {
			msg = msg[:i]
		}
	}
	if len(msg) > 60 {
		msg = msg[:57] + "..."
	}
	return msg
}
