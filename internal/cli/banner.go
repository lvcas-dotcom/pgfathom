package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/lvcas-dotcom/pgfathom/internal/report"
)

// mark is the project's glyph: a sounding line dropping into depth, which is
// what fathom means and what the tool does.
//
// Thirty-five columns wide, so it fits any terminal anybody still uses.
const mark = `
 █████          ███          █████
████████████ █████████ ████████████
███████████ ███████████ ███████████
███████████ ███████████ ███████████
██████████  ███ ███ ███  ██████████
 █████████  ███████████  █████████
   ███████  ███████████   ██████
            ███████████
            ███████████
              ███████
               █████
               █████
               █████
                ███`

// descent is the colour the glyph falls through, top to bottom. It stays red
// the whole way and only deepens, the way a colour does underwater: the shape
// is a sounding line, and the mark is the brand's red, not a fade out of it.
var descent = []string{
	"#e2483d", "#dd463b", "#d84439", "#d34137", "#ce3f36",
	"#c93d34", "#c43b32", "#bf3830", "#b9362e", "#b4342c",
	"#af322a", "#aa3029", "#a52d27", "#a02b25",
}

// Banner writes the glyph and the wordmark, and writes nothing at all when
// nobody is looking at a terminal.
//
// It goes to stderr on purpose. Stdout carries the result meant for
// consumption — a JSON document, a table somebody pipes, the command the guide
// composed — and a logo in the middle of that corrupts it. This is decoration,
// and decoration is diagnostic.
func Banner(out io.Writer, e report.Emphasis, tagline string) {
	// Thirty-five columns fits inside the eighty every terminal has had since
	// the punch card, so there is nothing to measure — only whether anybody is
	// looking.
	f, ok := out.(*os.File)
	if !ok || !isTerminal(f) {
		return
	}

	var b strings.Builder
	b.WriteString("\n")

	for i, line := range strings.Split(strings.TrimPrefix(mark, "\n"), "\n") {
		colour := descent[min(i, len(descent)-1)]
		fmt.Fprintf(&b, "  %s\n", e.Paint(colour, line))
	}

	fmt.Fprintf(&b, "\n  %s", e.Bold(e.Paint(report.BrandRed, "pgfathom")))
	if tagline != "" {
		fmt.Fprintf(&b, "  %s", e.Dim(tagline))
	}
	b.WriteString("\n")

	_, _ = io.WriteString(out, b.String())
}
