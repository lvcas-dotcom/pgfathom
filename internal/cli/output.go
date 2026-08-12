package cli

import (
	"io"
	"os"
)

// ColorMode controls ANSI emission.
type ColorMode string

// Colour modes. Auto resolves against the destination and the environment.
const (
	ColorAuto   ColorMode = "auto"
	ColorAlways ColorMode = "always"
	ColorNever  ColorMode = "never"
)

// Streams is where a command writes. The split is absolute: Out carries the
// result meant for consumption, Err carries everything else. This output gets
// piped into files and CI logs, where one stray diagnostic line on stdout
// corrupts programmatic consumption of the rest.
type Streams struct {
	// Out receives results. Never diagnostics.
	Out io.Writer

	// Err receives diagnostics. Never results.
	Err io.Writer

	In io.Reader

	// Interactive reports whether both In and Out are a real terminal — never
	// set from a flag. A command gates any prompt on this, not on a setting a
	// script would have to remember to pass: piped output, redirected input,
	// and CI never see one. Zero value is false, so a Streams built by literal
	// in a test stays silent unless a test opts in explicitly.
	Interactive bool

	color bool
}

// StdStreams wires a Streams to the process.
func StdStreams(mode ColorMode) *Streams {
	s := &Streams{Out: os.Stdout, Err: os.Stderr, In: os.Stdin}
	s.color = resolveColor(mode, os.Stdout)
	s.Interactive = isTerminal(os.Stdin) && isTerminal(os.Stdout)
	return s
}

// Color reports whether ANSI sequences may be emitted.
func (s *Streams) Color() bool { return s.color }

// resolveColor honours the explicit override first, then NO_COLOR, then
// whether the destination is an interactive terminal.
func resolveColor(mode ColorMode, out *os.File) bool {
	switch mode {
	case ColorAlways:
		return true
	case ColorNever:
		return false
	}

	// https://no-color.org — presence is enough, whatever the value.
	if _, set := os.LookupEnv("NO_COLOR"); set {
		return false
	}

	// A dumb terminal cannot render escapes meaningfully.
	if os.Getenv("TERM") == "dumb" {
		return false
	}

	return isTerminal(out)
}

// isTerminal reports whether f is an interactive character device. Checking the
// file mode keeps this dependency-free.
func isTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
