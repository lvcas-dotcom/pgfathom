package cli

import (
	"io"
	"os"

	"github.com/lvcas-dotcom/pgfathom/internal/report"
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

	color report.Emphasis

	// progress is the decision about drawing a self-rewriting line on Err,
	// taken at the boundary alongside colour and carried as a value.
	progress *Progress
}

// StdStreams wires a Streams to the process.
func StdStreams(mode ColorMode) *Streams {
	s := &Streams{Out: os.Stdout, Err: os.Stderr, In: os.Stdin}
	s.Interactive = isTerminal(os.Stdin) && isTerminal(os.Stdout)
	s.color = resolveEmphasis(mode, os.Stdout)

	// Progress is drawn on Err and therefore decided against Err. A run whose
	// result is piped but whose diagnostics still go to a terminal is the
	// ordinary case, and it is precisely the one that wants progress.
	s.progress = newProgress(resolveEmphasis(mode, os.Stderr) != report.NoEmphasis, os.Stderr)
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
func (s *Streams) Color() bool { return s.color != report.NoEmphasis }

// Emphasis is the level the destination supports, decided once here.
func (s *Streams) Emphasis() report.Emphasis { return s.color }

// resolveEmphasis honours the explicit override first, then NO_COLOR, then
// whether the destination is an interactive terminal — and finally whether that
// terminal can render the brand palette as itself.
func resolveEmphasis(mode ColorMode, out *os.File) report.Emphasis {
	switch mode {
	case ColorAlways:
		return depth()
	case ColorNever:
		return report.NoEmphasis
	}

	// https://no-color.org — presence is enough, whatever the value.
	if _, set := os.LookupEnv("NO_COLOR"); set {
		return report.NoEmphasis
	}

	// A dumb terminal cannot render escapes meaningfully.
	if os.Getenv("TERM") == "dumb" {
		return report.NoEmphasis
	}

	if !isTerminal(out) {
		return report.NoEmphasis
	}
	return depth()
}

// depth reads the de-facto signal for twenty-four bit colour. Getting it wrong
// costs an approximation, never garbage: a terminal that lies about COLORTERM
// is rarer than one that would print an unsupported sequence literally.
func depth() report.Emphasis {
	switch os.Getenv("COLORTERM") {
	case "truecolor", "24bit":
		return report.FullEmphasis
	}
	return report.BasicEmphasis
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
