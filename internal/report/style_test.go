package report_test

import (
	"strings"
	"testing"

	"github.com/lvcas-dotcom/pgfathom/internal/report"
)

// truecolor is how a 24-bit foreground opens. Terminals that predate it print
// the sequence as text, so its presence at the wrong emphasis level is not a
// cosmetic slip — it is garbage on somebody's screen.
const truecolor = "\x1b[38;2;"

// TestEachRoleIsItsOwnColour proves the palette carries information rather than
// decoration. A report is read by scanning it, and scanning only works when
// broken and confirmed do not look alike.
func TestEachRoleIsItsOwnColour(t *testing.T) {
	for _, level := range []struct {
		name string
		e    report.Emphasis
	}{
		{"24-bit", report.FullEmphasis},
		{"4-bit", report.BasicEmphasis},
	} {
		alert := level.e.Alert("x")
		confirm := level.e.Confirm("x")
		dim := level.e.Dim("x")

		if alert == confirm {
			t.Errorf("%s: broken and confirmed must not look alike: %q", level.name, alert)
		}
		if alert == dim || confirm == dim {
			t.Errorf("%s: a verdict must not look like secondary text", level.name)
		}
	}
}

// TestBasicEmphasisNeverEmitsTruecolor is the whole reason emphasis is a level
// and not a switch: the brand's red does not exist in four bits.
func TestBasicEmphasisNeverEmitsTruecolor(t *testing.T) {
	var b strings.Builder
	for _, paint := range []func(string) string{
		report.BasicEmphasis.Alert,
		report.BasicEmphasis.Confirm,
		report.BasicEmphasis.Dim,
		report.BasicEmphasis.Warn,
		report.BasicEmphasis.Bold,
	} {
		b.WriteString(paint("x"))
	}

	if strings.Contains(b.String(), truecolor) {
		t.Errorf("a 16-colour terminal must never receive a 24-bit sequence: %q", b.String())
	}
	if !strings.Contains(b.String(), "\x1b[") {
		t.Error("emphasis is on, so something must be emitted")
	}
}

// TestNoEmphasisIsBytePlain guards the pipe from below. The golden files cover
// the same ground through the renderer; this covers it at the source, so a new
// role cannot be added that forgets to ask.
func TestNoEmphasisIsBytePlain(t *testing.T) {
	for name, got := range map[string]string{
		"alert":   report.NoEmphasis.Alert("x"),
		"confirm": report.NoEmphasis.Confirm("x"),
		"dim":     report.NoEmphasis.Dim("x"),
		"warn":    report.NoEmphasis.Warn("x"),
		"bold":    report.NoEmphasis.Bold("x"),
		"paint":   report.NoEmphasis.Paint(report.BrandRed, "x"),
	} {
		if got != "x" {
			t.Errorf("%s: emphasis off must return the text untouched, got %q", name, got)
		}
	}
}
