package report

import "fmt"

// Emphasis carries the decision about ANSI, taken once at the process boundary
// and passed in. The renderer never consults the environment: a report written
// to a buffer in a test has to be byte-identical to one written to a pipe, and
// that stops being true the moment the formatting code can ask questions.
//
// It is a level rather than a flag because the brand palette is defined in hex,
// and a terminal that cannot render twenty-four bit colour has to get the four
// bit approximation instead of a sequence it will print as garbage.
type Emphasis int

// The levels, in increasing capability.
const (
	// NoEmphasis is a pipe, a file, NO_COLOR, or a dumb terminal.
	NoEmphasis Emphasis = iota

	// BasicEmphasis is a terminal that renders the sixteen ANSI colours.
	BasicEmphasis

	// FullEmphasis is a terminal that renders twenty-four bit colour, which is
	// where the brand palette shows as itself.
	FullEmphasis
)

// The brand palette. Hex is the source of truth; everything below is derived
// from it, and the four-bit fallbacks are the closest honest approximation.
//
// Red carries breakage, which is the finding this tool exists to produce and
// the one nobody may miss. Aqua carries confirmation. Stone is for everything
// said quietly — counts, reasons, what was not looked at.
const (
	BrandRed   = "#e2483d"
	BrandAqua  = "#7ec9b8"
	BrandStone = "#8c7d73"
	BrandBone  = "#ddd5d0"

	// BrandInk is bone's counterpart for a light terminal, where bone is paper on
	// paper. Only the interactive guide uses it: it needs to know the background,
	// and this package deliberately does not look.
	BrandInk = "#3f3733"
)

// Escape sequences. A dependency for these would cost a line in go.mod, which
// is read by whoever authorizes running this against production — and a colour
// library that inspects the terminal would break the property in the comment
// above this file's type.
const (
	ansiReset = "\x1b[0m"
	ansiBold  = "\x1b[1m"
	ansiDim   = "\x1b[2m"
	ansiRed   = "\x1b[31m"
	ansiAmber = "\x1b[33m"
	ansiCyan  = "\x1b[36m"
)

// truecolor renders a hex colour as a foreground sequence.
func truecolor(hex string) string {
	var r, g, b int
	if _, err := fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b); err != nil {
		return ""
	}
	return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", r, g, b)
}

// Paint renders s in a hex colour, for a caller drawing something the semantic
// roles below do not cover — the banner's gradient is the only one so far.
func (e Emphasis) Paint(hex, s string) string { return e.wrap(e.paint(hex, ""), s) }

// paint picks the sequence for a brand colour at this level: the exact one when
// the terminal can render it, the nearest of the sixteen when it cannot.
func (e Emphasis) paint(hex, fallback string) string {
	switch e {
	case FullEmphasis:
		return truecolor(hex)
	case BasicEmphasis:
		return fallback
	default:
		return ""
	}
}

func (e Emphasis) wrap(seq, s string) string {
	if e == NoEmphasis || seq == "" || s == "" {
		return s
	}
	return seq + s + ansiReset
}

// Bold marks a group heading.
func (e Emphasis) Bold(s string) string { return e.wrap(ansiBold, s) }

// Alert marks what the reader must not miss: the sampling caveat and the
// heading of the broken group.
func (e Emphasis) Alert(s string) string { return e.wrap(ansiBold+e.paint(BrandRed, ansiRed), s) }

// Confirm marks a relationship the data upheld.
func (e Emphasis) Confirm(s string) string { return e.wrap(ansiBold+e.paint(BrandAqua, ansiCyan), s) }

// Warn marks incomplete coverage — real, but not an emergency.
func (e Emphasis) Warn(s string) string { return e.wrap(ansiAmber, s) }

// Dim recedes what is present for completeness rather than for reading.
func (e Emphasis) Dim(s string) string { return e.wrap(ansiDim+e.paint(BrandStone, ""), s) }
