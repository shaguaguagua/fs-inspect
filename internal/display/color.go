// Package display centralises every piece of terminal decoration the CLI
// emits: ANSI colors, TTY detection, JSON pretty-printing and syntax
// highlighting. All functions are safe to call on a non-TTY: they
// degrade to plain text automatically and respect the NO_COLOR
// environment variable (https://no-color.org).
package display

import (
	"os"

	"golang.org/x/term"
)

// colorEnabled is resolved once at package init. Tests and re-runs do not
// change TTY state mid-process, so caching is fine.
var colorEnabled = func() bool {
	if _, noColor := os.LookupEnv("NO_COLOR"); noColor {
		return false
	}
	return term.IsTerminal(int(os.Stdout.Fd()))
}()

// ForceColor lets callers override TTY detection, e.g. the interactive
// shell wants colors even when writing into its own buffered viewport.
func ForceColor(on bool) { colorEnabled = on }

// Enabled reports whether colorized output is currently on.
func Enabled() bool { return colorEnabled }

// ANSI escape codes. Kept as constants so call sites read like
// `display.Green + text + display.Reset`.
const (
	reset = "\x1b[0m"

	bold = "\x1b[1m"
	dim  = "\x1b[2m"

	red     = "\x1b[31m"
	green   = "\x1b[32m"
	yellow  = "\x1b[33m"
	blue    = "\x1b[34m"
	magenta = "\x1b[35m"
	cyan    = "\x1b[36m"
	gray    = "\x1b[90m"
)

func wrap(code, s string) string {
	if !colorEnabled {
		return s
	}
	return code + s + reset
}

// Bold wraps s in ANSI bold if colors are enabled.
func Bold(s string) string { return wrap(bold, s) }

// Dim wraps s in ANSI dim if colors are enabled.
func Dim(s string) string { return wrap(dim, s) }

// Red wraps s in ANSI red.
func Red(s string) string { return wrap(red, s) }

// Green wraps s in ANSI green.
func Green(s string) string { return wrap(green, s) }

// Yellow wraps s in ANSI yellow.
func Yellow(s string) string { return wrap(yellow, s) }

// Blue wraps s in ANSI blue.
func Blue(s string) string { return wrap(blue, s) }

// Magenta wraps s in ANSI magenta.
func Magenta(s string) string { return wrap(magenta, s) }

// Cyan wraps s in ANSI cyan.
func Cyan(s string) string { return wrap(cyan, s) }

// Gray wraps s in ANSI bright black (gray).
func Gray(s string) string { return wrap(gray, s) }
