// Package report renders results for consumption: a grouped table for a human,
// a versioned JSON document for a machine, and reviewable .sql artifacts for
// whoever will act on the findings.
//
// Nothing here touches the database or reads user data. It receives a finished
// model.Result and turns it into text, which is what makes every output format
// testable without infrastructure.
package report

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/lvcas-dotcom/pgfathom/internal/model"
)

var findingTitles = map[model.FindingKind]string{
	model.FindingNotValidConstraint: "NOT VALID — declared, never verified against existing rows",
	model.FindingFKWithoutIndex:     "UNINDEXED — foreign key with no index on the child side",
	model.FindingOrphanReference:    "DANGLING — reference to a table that no longer exists",
}

// Terminal writes the audit result as a grouped table.
func Terminal(w io.Writer, r *model.Result, e Emphasis) error {
	var b strings.Builder

	fmt.Fprintf(&b, "\n  %s %s · PostgreSQL %s · %d tables in scope\n\n",
		r.Tool, r.ToolVersion, r.ServerVersion, r.Coverage.TablesTotal)

	if len(r.Findings) == 0 {
		writeNothingFound(&b, r)
	} else {
		writeFindings(&b, e, r.Findings)
	}

	writeCoverage(&b, e, r.Coverage)

	_, err := io.WriteString(w, b.String())
	return err
}

// writeNothingFound states the conclusion instead of leaving it implied. An
// empty output is ambiguous: it can mean everything is fine, or that nothing was
// looked at.
func writeNothingFound(b *strings.Builder, r *model.Result) {
	if r.Coverage.Complete() {
		fmt.Fprintf(b, "  No structural findings. All %d tables analyzed.\n\n",
			r.Coverage.TablesAnalyzed)
		return
	}

	fmt.Fprintf(b, "  No structural findings in the %d tables that were analyzed.\n",
		r.Coverage.TablesAnalyzed)
	fmt.Fprintf(b, "  %d were skipped — see below. This is not a clean bill of health.\n\n",
		r.Coverage.SkippedCount())
}

func writeFindings(b *strings.Builder, e Emphasis, findings []model.Finding) {
	byKind := make(map[model.FindingKind][]model.Finding)
	for _, f := range findings {
		byKind[f.Kind] = append(byKind[f.Kind], f)
	}

	kinds := make([]model.FindingKind, 0, len(byKind))
	for k := range byKind {
		kinds = append(kinds, k)
	}
	sort.Slice(kinds, func(i, j int) bool { return kinds[i] < kinds[j] })

	for _, kind := range kinds {
		group := byKind[kind]

		title := findingTitles[kind]
		if title == "" {
			title = string(kind)
		}
		fmt.Fprintf(b, "  %s\n", e.Bold(fmt.Sprintf("%s  (%d)", title, len(group))))
		fmt.Fprintf(b, "  %s\n", strings.Repeat("─", ruleWidth))

		tw := tabwriter.NewWriter(b, 0, 0, 2, ' ', 0)
		for _, f := range group {
			writeRow(tw, f.Object, formatMetrics(f.Metrics))
		}
		_ = tw.Flush()
		b.WriteString("\n")
	}
}

func formatMetrics(m map[string]int64) string {
	if len(m) == 0 {
		return ""
	}

	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, humanCount(m[k])))
	}
	return strings.Join(parts, "  ")
}

func humanCount(n int64) string {
	switch {
	case n < 0:
		// The reltuples sentinel for "never analyzed". Printing -1 as a row
		// count would be worse than saying nothing.
		return "unknown"
	case n >= 1_000_000_000:
		return fmt.Sprintf("%.1fB", float64(n)/1e9)
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// writeCoverage always runs. A clean report has to mean "I looked and it is
// clean", never "I could not look".
func writeCoverage(b *strings.Builder, e Emphasis, c model.Coverage) {
	line := fmt.Sprintf("%d tables · %d analyzed", c.TablesTotal, c.TablesAnalyzed)
	if n := c.SkippedCount(); n > 0 {
		line = e.Warn(line + fmt.Sprintf(" · %d skipped", n))
	}
	fmt.Fprintf(b, "  %s\n", line)

	writeSkipped(b, "no SELECT privilege", c.TablesNoPrivilege)
	writeSkipped(b, "excluded by filter", c.TablesExcluded)

	if len(c.TablesUnsupported) > 0 {
		byReason := make(map[model.UnsupportedReason][]string)
		for _, s := range c.TablesUnsupported {
			byReason[s.Reason] = append(byReason[s.Reason], s.Table)
		}
		reasons := make([]string, 0, len(byReason))
		for r := range byReason {
			reasons = append(reasons, string(r))
		}
		sort.Strings(reasons)
		for _, r := range reasons {
			writeSkipped(b, strings.ReplaceAll(r, "_", " "), byReason[model.UnsupportedReason(r)])
		}
	}

	// The prefilter line distinguishes "I looked and dropped nothing" from "I
	// did not look". It only appears when there was an inference to filter, so
	// audit output stays free of it.
	if c.StatsPrefilter {
		fmt.Fprintf(b, "  stats prefilter: %d checked · %d rejected · %d without statistics\n",
			c.CandidatesStatsChecked, c.CandidatesStatsRejected, c.CandidatesWithoutStats)
	} else if c.CandidatesFound > 0 {
		b.WriteString("  stats prefilter off — no planner-statistics check was applied\n")
	}

	if c.StatsResetAt == nil {
		b.WriteString("  ! statistics reset time unknown — usage counters carry no meaning\n")
	}

	b.WriteString("\n")
}

func writeSkipped(b *strings.Builder, label string, tables []string) {
	if len(tables) == 0 {
		return
	}

	const maxListed = 5
	shown := tables
	if len(shown) > maxListed {
		shown = shown[:maxListed]
	}

	fmt.Fprintf(b, "    %d %s: %s", len(tables), label, strings.Join(shown, ", "))
	if len(tables) > maxListed {
		fmt.Fprintf(b, " and %d more", len(tables)-maxListed)
	}
	b.WriteString("\n")
}
