package report

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/lvcas-dotcom/pgfathom/internal/model"
)

// ruleWidth is the width of the separator under a group heading. Wide enough
// for the validated table, which carries seven columns.
const ruleWidth = 96

// verdictOrder is the presentation order, and it deliberately contradicts
// confidence. A confirmed relationship is housekeeping that can wait for the
// next window; a broken one is integrity that has been violated in production,
// probably for years. It is the finding this tool exists to produce, so it
// goes where nobody can miss it.
var verdictOrder = []model.Verdict{
	model.VerdictBroken,
	model.VerdictConfirmed,
	model.VerdictWeak,
	model.VerdictUnvalidated,
}

var verdictTitles = map[model.Verdict]string{
	model.VerdictBroken:      "BROKEN — the relationship is real; its integrity is not",
	model.VerdictConfirmed:   "CONFIRMED — total containment, verified row by row",
	model.VerdictWeak:        "WEAK — the data supports no conclusion either way",
	model.VerdictUnvalidated: "UNVALIDATED — no evidence gathered; not the same as clean",
	model.VerdictRejected:    "REJECTED — the data knocked the hypothesis down",
}

// DiscoverView is everything the discover report needs, including what was
// thrown away.
type DiscoverView struct {
	Result *model.Result

	// Discarded fell below the score threshold or was killed by the statistical
	// prefilter. Shown only on request, but always counted, so nobody has to
	// wonder why an obvious-looking column was ignored.
	Discarded []model.Candidate

	MinScore        float64
	ShowDiscarded   bool
	ValidationStage string

	// Detection is what the schema revealed about its own naming convention.
	// Reporting it is not decoration: a profile that changes itself without
	// saying so leaves the user unable to reproduce or to disagree.
	Detection model.NamingDetection

	// Emphasis is decided at the process boundary and passed in.
	Emphasis Emphasis
}

// Discover renders inferred candidates grouped by verdict.
func Discover(w io.Writer, v DiscoverView) error {
	var b strings.Builder
	r := v.Result
	e := v.Emphasis

	fmt.Fprintf(&b, "\n  %s %s · PostgreSQL %s · profile %s · threshold %.2f\n",
		r.Tool, r.ToolVersion, r.ServerVersion, r.Profile, v.MinScore)

	// The mode is not a footnote. A well-scored candidate is still a hypothesis,
	// and a sampled run cannot prove the absence of an orphan; presenting either
	// without the caveat invites creating a constraint from a name match.
	writeStage(&b, e, v.ValidationStage, r.Sampled())

	if len(r.Candidates) == 0 {
		fmt.Fprintf(&b, "  No candidate relationships above the threshold.\n\n")
	} else {
		writeGroups(&b, e, r)
	}

	if v.ShowDiscarded && len(v.Discarded) > 0 {
		writeDiscarded(&b, e, v.Discarded)
	}

	writeDetection(&b, v)
	writeObservations(&b, e, r.Findings)
	writeTally(&b, e, r, len(v.Discarded))
	writeCoverage(&b, e, r.Coverage)

	if r.Sampled() {
		b.WriteString("  " + e.Alert(sampledFooter) + "\n\n")
	}

	if !v.ShowDiscarded && len(v.Discarded) > 0 {
		fmt.Fprintf(&b, "  %s\n\n", e.Dim(fmt.Sprintf(
			"%d candidates below the threshold — pass --include-rejected to see them",
			len(v.Discarded))))
	}

	_, err := io.WriteString(w, b.String())
	return err
}

const sampledFooter = "! sampled run: nothing above is confirmed and every orphan count is a floor. " +
	"Re-run with --full before acting on absence."

func writeStage(b *strings.Builder, e Emphasis, stage string, sampled bool) {
	if stage == "" {
		b.WriteString("\n")
		return
	}
	if sampled || strings.HasPrefix(stage, "!") {
		fmt.Fprintf(b, "  %s\n\n", e.Alert(stage))
		return
	}
	fmt.Fprintf(b, "  %s\n\n", stage)
}

// writeGroups renders every verdict group, including the empty ones. A missing
// group is indistinguishable from a group that was never evaluated, which is
// the same silence the coverage block exists to prevent.
func writeGroups(b *strings.Builder, e Emphasis, r *model.Result) {
	for _, verdict := range verdictOrder {
		group := r.CandidatesByVerdict(verdict)

		heading := fmt.Sprintf("%s  (%d)", verdictTitles[verdict], len(group))
		if verdict == model.VerdictBroken && len(group) > 0 {
			heading = e.Alert(heading)
		} else {
			heading = e.Bold(heading)
		}
		fmt.Fprintf(b, "  %s\n", heading)
		fmt.Fprintf(b, "  %s\n", strings.Repeat("─", ruleWidth))

		if len(group) == 0 {
			fmt.Fprintf(b, "  %s\n\n", e.Dim("none"))
			continue
		}
		writeCandidateTable(b, group, verdict != model.VerdictUnvalidated)
	}
}

func writeDiscarded(b *strings.Builder, e Emphasis, discarded []model.Candidate) {
	fmt.Fprintf(b, "  %s\n", e.Bold(fmt.Sprintf(
		"DISCARDED — below the threshold or statistically impossible  (%d)", len(discarded))))
	fmt.Fprintf(b, "  %s\n", strings.Repeat("─", ruleWidth))
	writeCandidateTable(b, discarded, false)
}

// writeCandidateTable renders one group. Validated groups get the metric
// columns; the rest get the signals, which is all the evidence they have.
func writeCandidateTable(b *strings.Builder, candidates []model.Candidate, validated bool) {
	tw := tabwriter.NewWriter(b, 0, 0, 2, ' ', 0)

	if validated {
		writeRow(tw, "relation", "rows", "values", "orphan rows", "orphan values", "examined", "method")
	}

	for _, c := range candidates {
		relation := c.Child.String() + " → " + c.Parent.String()

		if !validated {
			writeRow(tw, relation, fmt.Sprintf("%.2f", c.MetaScore), describe(c))
			continue
		}

		v := c.Validation
		if v == nil {
			writeRow(tw, relation, "—", "—", "—", "—", "—", "—", describe(c))
			continue
		}

		writeRow(tw,
			relation,
			fmt.Sprintf("%.1f%%", 100*v.ContainmentRows()),
			fmt.Sprintf("%.1f%%", 100*v.ContainmentVals()),
			fmt.Sprintf("%d", v.OrphanRows),
			fmt.Sprintf("%d", v.OrphanVals),
			humanCount(v.SampledRows),
			string(v.Method),
			c.Reason,
		)
	}

	_ = tw.Flush()
	b.WriteString("\n")
}

// writeRow drops trailing empty cells so an absent reason does not leave a
// column of padding behind it.
func writeRow(tw io.Writer, cells ...string) {
	for len(cells) > 0 && cells[len(cells)-1] == "" {
		cells = cells[:len(cells)-1]
	}
	if len(cells) == 0 {
		return
	}
	_, _ = fmt.Fprintf(tw, "  %s\n", strings.Join(cells, "\t"))
}

// describe explains a candidate that carries no metrics: the reason it could
// not be validated, or the signals that produced its score.
func describe(c model.Candidate) string {
	if c.Reason != "" {
		return c.Reason
	}
	return summarize(c.Signals)
}

// writeTally closes the account. Zero counts are printed: "no broken
// relationships" and "broken relationships were not looked for" are different
// claims and must not render identically.
func writeTally(b *strings.Builder, e Emphasis, r *model.Result, discarded int) {
	counts := r.CountByVerdict()

	parts := make([]string, 0, len(verdictOrder)+2)
	for _, v := range verdictOrder {
		parts = append(parts, fmt.Sprintf("%d %s", counts[v], v))
	}
	parts = append(parts, fmt.Sprintf("%d discarded", discarded))

	line := strings.Join(parts, " · ")
	if r.Duration > 0 {
		line += fmt.Sprintf(" · %s", r.Duration.Round(1e6))
	}

	fmt.Fprintf(b, "  %s\n", e.Bold(line))
}

func writeObservations(b *strings.Builder, e Emphasis, findings []model.Finding) {
	relevant := make([]model.Finding, 0, len(findings))
	for _, f := range findings {
		if f.Kind == model.FindingPolymorphicPair || f.Kind == model.FindingUnsupportedTarget {
			relevant = append(relevant, f)
		}
	}
	if len(relevant) == 0 {
		return
	}

	fmt.Fprintf(b, "  %s\n", e.Bold(fmt.Sprintf(
		"NOT ANALYZED — recognized, out of scope for this version  (%d)", len(relevant))))
	fmt.Fprintf(b, "  %s\n", strings.Repeat("─", ruleWidth))

	tw := tabwriter.NewWriter(b, 0, 0, 2, ' ', 0)
	for _, f := range relevant {
		writeRow(tw, f.Object, f.Detail)
	}
	_ = tw.Flush()
	b.WriteString("\n")
}

// summarize renders the signals compactly, strongest first, so a score can be
// argued with rather than merely accepted.
func summarize(signals []model.Signal) string {
	ordered := make([]model.Signal, len(signals))
	copy(ordered, signals)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Weight > ordered[j].Weight })

	parts := make([]string, 0, len(ordered))
	for _, s := range ordered {
		if s.Weight < 0 {
			parts = append(parts, "-"+string(s.Kind))
			continue
		}
		parts = append(parts, string(s.Kind))
	}
	return strings.Join(parts, " ")
}

func writeDetection(b *strings.Builder, v DiscoverView) {
	if !v.Detection.Enabled {
		fmt.Fprintf(b, "  naming detection is off — only the %s profile applies\n\n", v.Result.Profile)
		return
	}

	if v.Detection.Empty() {
		fmt.Fprintf(b, "  Nothing detected from %d tables and %d declared keys; the %s profile applies alone.\n\n",
			v.Detection.Tables, v.Detection.DeclaredKeys, v.Result.Profile)
		return
	}

	fmt.Fprintf(b, "  %s\n", v.Emphasis.Bold(fmt.Sprintf(
		"DETECTED — conventions read from the schema itself, added to %s", v.Result.Profile)))
	fmt.Fprintf(b, "  %s\n", strings.Repeat("─", ruleWidth))

	tw := tabwriter.NewWriter(b, 0, 0, 2, ' ', 0)
	for _, group := range []struct {
		label string
		items []model.NamingEvidence
	}{
		{"reference suffix", v.Detection.ColumnSuffixes},
		{"reference prefix", v.Detection.ColumnPrefixes},
		{"table prefix", v.Detection.TablePrefixes},
	} {
		for _, e := range group.items {
			writeRow(tw, group.label, e.Affix,
				fmt.Sprintf("%d occurrences (%.0f%%)", e.Occurrences, 100*e.Share))
		}
	}
	_ = tw.Flush()
	b.WriteString("\n")
}
