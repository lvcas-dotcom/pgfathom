package cli

import (
	"fmt"
	"io"
	"sync"

	"github.com/lvcas-dotcom/pgfathom/internal/discovery"
)

// Progress draws what is happening on a single line that rewrites itself.
//
// It exists because `discover` against a large schema is minutes of silence,
// and the first time somebody points this tool at production is exactly when
// that silence costs most: the person who authorised it is watching.
//
// The line goes to stderr, never stdout — progress in stdout would corrupt the
// programmatic consumption that the whole output discipline protects. And it is
// drawn only when the destination is an interactive terminal, decided once at
// the process boundary and carried as a value, the same way emphasis is.
type Progress struct {
	out io.Writer

	// mu serialises drawing. The reports come from inside the validation
	// concurrency group, and a progress line that scrambles itself reads as a
	// defect at the worst possible moment.
	mu    sync.Mutex
	dirty bool
}

// newProgress returns a drawer for out when enabled, and a silent one when not.
//
// The decision arrives already taken, from the same resolution that governs
// colour. NO_COLOR counts there even though a progress line is not colour:
// somebody who sets it is asking for a stream without escape sequences, and
// rewriting a line is an escape sequence. Being generous with that reading
// costs nothing and keeps somebody's log clean.
func newProgress(enabled bool, out io.Writer) *Progress {
	if !enabled {
		return &Progress{}
	}
	return &Progress{out: out}
}

// Report draws one update. Safe to call from several goroutines.
func (p *Progress) Report(r discovery.Progress) {
	if p == nil || p.out == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	line := string(r.Stage)
	if r.Total > 0 {
		line = fmt.Sprintf("%s %d/%d", r.Stage, r.Done, r.Total)
	}

	// Carriage return and erase-to-end-of-line: the next report overwrites this
	// one, and a shorter line never leaves the tail of a longer one behind.
	// A failed write to the terminal is nothing to report: the progress line is
	// decoration, and refusing to continue over it would abort a run that is
	// otherwise fine.
	_, _ = fmt.Fprintf(p.out, "\r\x1b[K  %s", line)
	p.dirty = true
}

// Clear removes the line, so whatever is written next starts clean. Calling it
// when nothing was drawn writes nothing.
func (p *Progress) Clear() {
	if p == nil || p.out == nil {
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.dirty {
		return
	}
	_, _ = fmt.Fprint(p.out, "\r\x1b[K")
	p.dirty = false
}
