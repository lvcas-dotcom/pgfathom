package report

// Emphasis carries the decision about ANSI, taken once at the process boundary
// and passed in. The renderer never consults the environment: a report written
// to a buffer in a test has to be byte-identical to one written to a pipe, and
// that stops being true the moment the formatting code can ask questions.
type Emphasis bool

// Escape sequences, kept to what the report actually uses. A dependency for six
// constants would cost a line in go.mod, which is read by whoever authorizes
// running this against production.
const (
	ansiReset = "\x1b[0m"
	ansiBold  = "\x1b[1m"
	ansiDim   = "\x1b[2m"
	ansiRed   = "\x1b[31m"
	ansiAmber = "\x1b[33m"
)

func (e Emphasis) wrap(seq, s string) string {
	if !e || s == "" {
		return s
	}
	return seq + s + ansiReset
}

// Bold marks a group heading.
func (e Emphasis) Bold(s string) string { return e.wrap(ansiBold, s) }

// Alert marks what the reader must not miss: the sampling caveat and the
// heading of the broken group.
func (e Emphasis) Alert(s string) string { return e.wrap(ansiBold+ansiRed, s) }

// Warn marks incomplete coverage — real, but not an emergency.
func (e Emphasis) Warn(s string) string { return e.wrap(ansiAmber, s) }

// Dim recedes what is present for completeness rather than for reading.
func (e Emphasis) Dim(s string) string { return e.wrap(ansiDim, s) }
