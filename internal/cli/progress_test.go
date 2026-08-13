package cli

import (
	"bytes"
	"strings"
	"sync"
	"testing"

	"github.com/lvcas-dotcom/pgfathom/internal/discovery"
)

// TestSilentWhenNotEnabled is the guard on the rule that matters most here: a
// destination that is not an interactive terminal — a pipe, a file, a CI log —
// receives nothing at all, not even a cleared line.
func TestSilentWhenNotEnabled(t *testing.T) {
	var b bytes.Buffer
	p := newProgress(false, &b)

	p.Report(discovery.Progress{Stage: discovery.StageCatalog})
	p.Report(discovery.Progress{Stage: discovery.StageValidation, Done: 3, Total: 9})
	p.Clear()

	if b.Len() != 0 {
		t.Errorf("a destination that should not receive escapes got %d bytes: %q", b.Len(), b.String())
	}
}

// TestZeroValueIsSafe covers the accessor contract: Progress is never nil, and a
// zero one draws nothing rather than panicking.
func TestZeroValueIsSafe(t *testing.T) {
	var p *Progress
	p.Report(discovery.Progress{Stage: discovery.StageCatalog})
	p.Clear()

	(&Progress{}).Report(discovery.Progress{Stage: discovery.StageCatalog})
	(&Progress{}).Clear()
}

func TestReportRewritesOneLine(t *testing.T) {
	var b bytes.Buffer
	p := newProgress(true, &b)

	p.Report(discovery.Progress{Stage: discovery.StageCatalog})
	p.Report(discovery.Progress{Stage: discovery.StageValidation, Done: 312, Total: 1654})

	out := b.String()
	if strings.Count(out, "\n") != 0 {
		t.Errorf("progress must stay on one line; got %q", out)
	}
	for _, want := range []string{"\r\x1b[K", "catalog", "validation 312/1654"} {
		if !strings.Contains(out, want) {
			t.Errorf("output %q is missing %q", out, want)
		}
	}
}

// TestClearOnlyWhenDrawn keeps a clean destination clean: clearing a line that
// was never drawn would emit escapes into somebody's terminal for nothing.
func TestClearOnlyWhenDrawn(t *testing.T) {
	var b bytes.Buffer
	p := newProgress(true, &b)

	p.Clear()
	if b.Len() != 0 {
		t.Errorf("clearing before drawing wrote %q", b.String())
	}

	p.Report(discovery.Progress{Stage: discovery.StageCatalog})
	b.Reset()
	p.Clear()
	if b.String() != "\r\x1b[K" {
		t.Errorf("clear wrote %q", b.String())
	}
	b.Reset()
	p.Clear()
	if b.Len() != 0 {
		t.Errorf("clearing twice wrote %q", b.String())
	}
}

// TestConcurrentReportsDoNotInterleave covers where the reports come from: the
// validation concurrency group. A line that scrambles itself reads as a defect
// at the moment somebody is deciding whether to trust the tool.
func TestConcurrentReportsDoNotInterleave(t *testing.T) {
	var b bytes.Buffer
	p := newProgress(true, &b)

	var wg sync.WaitGroup
	for i := 1; i <= 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.Report(discovery.Progress{Stage: discovery.StageValidation, Done: i, Total: 50})
		}()
	}
	wg.Wait()

	// Every write is one complete report, so the count of erase sequences has to
	// match the count of reports exactly.
	if n := strings.Count(b.String(), "\r\x1b[K"); n != 50 {
		t.Errorf("saw %d complete reports, want 50: writes interleaved", n)
	}
}
