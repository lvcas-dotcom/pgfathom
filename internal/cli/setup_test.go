package cli

import (
	"bytes"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/lvcas-dotcom/pgfathom/internal/report"
)

// TestPlanComposesWhatWasAnswered is the guide's central promise: the command
// it prints is the command those answers mean. If this drifts, the guide starts
// teaching a line that does something else.
func TestPlanComposesWhatWasAnswered(t *testing.T) {
	tests := []struct {
		name string
		plan plan
		want string
	}{
		{
			"connection from the environment stays out of the command",
			plan{dsnFromEnv: true, dsn: "postgres://u@h/db", schemas: []string{"public"}},
			"pgfathom discover --schema public",
		},
		{
			"schemas, full mode and artifacts",
			plan{dsnFromEnv: true, schemas: []string{"geral", "gestaoobras"}, full: true, out: "/tmp/out"},
			"pgfathom discover --schema geral,gestaoobras --full --out /tmp/out",
		},
		{
			"a composed connection is carried explicitly",
			plan{dsn: "postgres://u@h:5432/db", schemas: []string{"public"}},
			"pgfathom discover --dsn postgres://u@h:5432/db --schema public",
		},
		{
			"a path with a space survives copying",
			plan{dsnFromEnv: true, schemas: []string{"public"}, out: "/tmp/my out"},
			"pgfathom discover --schema public --out '/tmp/my out'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.plan.Command(); got != tt.want {
				t.Errorf("command =\n  %s\nwant\n  %s", got, tt.want)
			}
		})
	}
}

// TestPrintedCommandIsTheExecutedOne closes the gap the guide would otherwise
// open: showing one thing and running another is worse than not showing at all.
func TestPrintedCommandIsTheExecutedOne(t *testing.T) {
	p := plan{schemas: []string{"geral"}, full: true, out: "/tmp/out", dsn: "postgres://u@h/db"}

	printed := p.Command()
	for _, arg := range p.Args() {
		if !strings.Contains(printed, arg) {
			t.Errorf("argument %q is executed but does not appear in the printed command %q", arg, printed)
		}
	}
	if n := strings.Count(printed, "--"); n != len(flagsIn(p.Args())) {
		t.Errorf("printed command has %d flags, the executed one has %d", n, len(flagsIn(p.Args())))
	}
}

func flagsIn(args []string) []string {
	var out []string
	for _, a := range args {
		if strings.HasPrefix(a, "--") {
			out = append(out, a)
		}
	}
	return out
}

// TestRedactHidesThePassword covers the one place a credential can reach the
// screen: an environment variable somebody put a password into.
func TestRedactHidesThePassword(t *testing.T) {
	tests := map[string]string{
		"postgres://user:hunter2@host:5432/db": "postgres://user:•••@host:5432/db",
		"postgres://user@host:5432/db":         "postgres://user@host:5432/db",
		"host=localhost dbname=x":              "host=localhost dbname=x",
	}

	for in, want := range tests {
		if got := redact(in); got != want {
			t.Errorf("redact(%q) = %q, want %q", in, got, want)
		}
		if strings.Contains(redact(in), "hunter2") {
			t.Errorf("redact(%q) leaked the password", in)
		}
	}
}

// TestListFitsTheTerminal is the guard on the failure the guide's first real
// run produced: sixty-one schemas in a thirty-two line terminal scrolled the
// title and the biggest schemas off the top, so the one option the list exists
// to show was invisible, and the default got accepted by someone who could not
// see the alternative.
func TestListFitsTheTerminal(t *testing.T) {
	options := make([]Option, 61)
	for i := range options {
		options[i] = Option{Label: "schema", Detail: "tables"}
	}

	m := chooser{title: "Which schemas?", options: options, multi: true,
		picked: map[int]bool{}, height: 12}

	for _, cursor := range []int{0, 30, 60} {
		m.cursor = cursor
		view := m.View()

		// Twelve option rows plus title, markers and hint: seven lines of frame.
		if lines := strings.Count(view, "\n"); lines > 19 {
			t.Errorf("cursor at %d drew %d lines into a 12-row window", cursor, lines)
		}
		if !strings.Contains(view, "Which schemas?") {
			t.Errorf("cursor at %d scrolled the title away", cursor)
		}

		start, end := m.slice()
		if cursor < start || cursor >= end {
			t.Errorf("cursor at %d is outside the drawn window %d..%d", cursor, start, end)
		}
	}
}

// TestListSaysHowMuchIsHidden keeps the rule that silence is never absence: a
// window that shows ten of sixty-one has to say the other fifty-one exist.
func TestListSaysHowMuchIsHidden(t *testing.T) {
	options := make([]Option, 61)
	for i := range options {
		options[i] = Option{Label: "schema"}
	}
	m := chooser{title: "t", options: options, picked: map[int]bool{}, height: 10}

	m.cursor = 0
	if !strings.Contains(m.View(), "↓ 51 more") {
		t.Errorf("the top of the list must say how many are below:\n%s", m.View())
	}

	m.cursor = 60
	if !strings.Contains(m.View(), "↑ 51 more") {
		t.Errorf("the bottom of the list must say how many are above:\n%s", m.View())
	}
}

// TestShortListNeedsNoWindow keeps the common case clean: four options do not
// get paging hints or "more" markers.
func TestShortListNeedsNoWindow(t *testing.T) {
	m := chooser{title: "t", options: []Option{{Label: "a"}, {Label: "b"}}, picked: map[int]bool{}}
	view := m.View()

	for _, unwanted := range []string{"more", "pgup"} {
		if strings.Contains(view, unwanted) {
			t.Errorf("a two-option list should not mention %q:\n%s", unwanted, view)
		}
	}
}

// TestEnterTakesTheCursorWhenNothingIsTicked covers what the guide's second
// real run got wrong: the user moved to the schema they wanted, pressed enter,
// and the list answered with a tick they could not see.
func TestEnterTakesTheCursorWhenNothingIsTicked(t *testing.T) {
	m := chooser{
		options: []Option{{Label: "geral"}, {Label: "public"}},
		multi:   true,
		picked:  map[int]bool{},
		cursor:  0,
	}

	final, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := final.(chooser)

	if !got.done {
		t.Fatal("enter on a highlighted item must accept it")
	}
	if chosen := got.chosen(); len(chosen) != 1 || chosen[0] != 0 {
		t.Errorf("chose %v, want the item under the cursor", chosen)
	}
}

// TestEnterKeepsWhatWasTicked is the other half: once the user has ticked
// something deliberately, the cursor position stops deciding.
func TestEnterKeepsWhatWasTicked(t *testing.T) {
	m := chooser{
		options: []Option{{Label: "geral"}, {Label: "gestaoobras"}, {Label: "public"}},
		multi:   true,
		picked:  map[int]bool{0: true, 1: true},
		cursor:  2,
	}

	final, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := final.(chooser)

	if chosen := got.chosen(); len(chosen) != 2 || chosen[0] != 0 || chosen[1] != 1 {
		t.Errorf("chose %v, want the two ticked ones and not the cursor", chosen)
	}
}

// TestBannerNeverReachesANonTerminal is the same discipline the report and the
// progress line follow, applied to the one piece of output that is pure
// decoration: a logo in the middle of a redirected stream is corruption.
func TestBannerNeverReachesANonTerminal(t *testing.T) {
	var b bytes.Buffer
	Banner(&b, report.FullEmphasis, "a tagline")

	if b.Len() != 0 {
		t.Errorf("a buffer is not a terminal; nothing may be written, got %q", b.String())
	}
}
