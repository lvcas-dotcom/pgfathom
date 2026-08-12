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

	color bool

	// progress is the decision about drawing a self-rewriting line on Err,
	// taken at the boundary alongside colour and carried as a value.
	progress *Progress
}

// StdStreams wires a Streams to the process.
func StdStreams(mode ColorMode) *Streams {
	s := &Streams{Out: os.Stdout, Err: os.Stderr, In: os.Stdin}
	s.color = resolveColor(mode, os.Stdout)

	// Progress is drawn on Err and therefore decided against Err. A run whose
	// result is piped but whose diagnostics still go to a terminal is the
	// ordinary case, and it is precisely the one that wants progress.
	s.progress = newProgress(resolveColor(mode, os.Stderr), os.Stderr)
	return s
}

// Progress returns the drawer for this destination. It is never nil, and it
// writes nothing when the destination should not receive escapes.
func (s *Streams) Progress() *Progress {
	if s.progress == nil {
		return &Progress{}
	}
	return s.progress
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
