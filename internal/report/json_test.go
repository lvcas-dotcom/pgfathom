package report_test

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/lvcas-dotcom/pgfathom/internal/model"
	"github.com/lvcas-dotcom/pgfathom/internal/report"
	"github.com/lvcas-dotcom/pgfathom/internal/testutil"
)

// keyPaths walks the document and returns every key path it contains, with
// array indices collapsed. Comparing values would make the golden fail on any
// number change, which is noise; comparing the shape fails exactly when the
// contract moves, which is the signal.
func keyPaths(v any, prefix string, seen map[string]bool) {
	switch t := v.(type) {
	case map[string]any:
		for k, child := range t {
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}
			seen[path] = true
			keyPaths(child, path, seen)
		}
	case []any:
		for _, child := range t {
			keyPaths(child, prefix+"[]", seen)
		}
	}
}

// contractResult populates every field of the model, so an unpopulated one
// cannot hide behind omitempty and slip out of the contract unnoticed.
func contractResult() *model.Result {
	resetAt := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

	r := model.NewResult(goldenVersion, "pt-br", goldenTime(), model.Coverage{
		TablesTotal: 12, TablesAnalyzed: 9,
		TablesNoPrivilege: []string{"public.folha"},
		TablesExcluded:    []string{"public.tmp_carga"},
		TablesUnsupported: []model.SkippedTable{{Table: "public.evento", Reason: model.ReasonNoPrimaryKey}},
		CandidatesFound:   9, CandidatesValidated: 3, CandidatesTimedOut: 1,
		StatsPrefilter: true, CandidatesStatsChecked: 9,
		CandidatesStatsRejected: 4, CandidatesWithoutStats: 1,
		StatsResetAt: &resetAt, PgStatStatements: true,
	})
	r.Duration = goldenDuration
	r.ServerVersion = goldenServer
	r.Naming = model.NamingDetection{
		Enabled:        true,
		ColumnSuffixes: []model.NamingEvidence{{Affix: "_idkey", Occurrences: 102, Share: 0.22}},
		ColumnPrefixes: []model.NamingEvidence{{Affix: "cod_", Occurrences: 41, Share: 0.09}},
		TablePrefixes:  []model.NamingEvidence{{Affix: "tpl_", Occurrences: 88, Share: 0.26}},
		DeclaredKeys:   470, Tables: 338,
	}

	r.Schemas = []model.Schema{{
		Name: "public",
		Tables: []model.Table{{
			Schema: "public", Name: "pedido",
			Columns: []model.Column{{
				Name: "cliente_id", Type: "integer", BaseType: "int4",
				Nullable: true, Default: "NULL", Position: 2, Comment: "referencia ao cliente",
			}},
			PrimaryKey: []string{"id"},
			Uniques:    [][]string{{"numero"}},
			ForeignKeys: []model.ForeignKey{{
				Name: "pedido_cliente_fkey", Columns: []string{"cliente_id"},
				RefSchema: "public", RefTable: "cliente", RefColumns: []string{"id"},
				Validated: false, HasIndex: true,
			}},
			Indexes: []model.Index{{Name: "pedido_pkey", Columns: []string{"id"}, Unique: true, Primary: true}},
			Stats: model.TableStats{
				EstimatedRows: 1_284_000, TotalBytes: 98_000_000,
				Usage: model.NewUsageStats(
					model.UsageCounters{SeqScans: 3, IdxScans: 900, Inserts: 12, Updates: 4, Deletes: 1},
					resetAt),
			},
			Comment: "pedidos de venda",
		},
			// Two shapes the analysis recognizes but cannot target. They are here
			// so that `partitioned` and `inherits` — both omitempty — appear in
			// the contract: a path that only materializes once someone sets the
			// field would make this test fail long after the decision was made.
			{Schema: "public", Name: "medicao", Partitioned: true},
			{Schema: "public", Name: "evento_legado", Inherits: true},
		},
	}}

	r.Candidates = []model.Candidate{verdictCandidate("pedido", "cliente_id", "cliente",
		model.VerdictBroken, validated(model.MethodFull, 1_284_000, 48_120, 1284, 12),
		"orphan counts are a floor: this table was sampled")}
	r.Discarded = []model.Candidate{verdictCandidate("log", "status_id", "status",
		model.VerdictRejected, nil, "low containment: the name match is a coincidence")}
	r.Findings = []model.Finding{{
		Kind: model.FindingNotValidConstraint, Object: "public.pedido",
		Detail: "never verified", Metrics: map[string]int64{"rows": 1_284_000},
	}}

	return r
}

// TestJSONContractIsFrozen turns any change to the document's shape into a
// reviewable diff. The document is what the CI baseline and third-party tools
// consume, so a field that moves without an intent behind it is a break.
func TestJSONContractIsFrozen(t *testing.T) {
	var b bytes.Buffer
	if err := report.JSON(&b, contractResult()); err != nil {
		t.Fatalf("JSON: %v", err)
	}

	var doc any
	if err := json.Unmarshal(b.Bytes(), &doc); err != nil {
		t.Fatalf("the emitted document must be valid JSON: %v", err)
	}

	seen := make(map[string]bool)
	keyPaths(doc, "", seen)

	paths := make([]string, 0, len(seen))
	for p := range seen {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	testutil.Golden(t, "json_contract", strings.Join(paths, "\n")+"\n")
}

// TestDiscardedNeverSitBesideSurvivors covers the distinction the contract
// makes structurally: no consumer should have to read a score to learn whether
// a candidate made it through triage.
func TestDiscardedNeverSitBesideSurvivors(t *testing.T) {
	var b bytes.Buffer
	if err := report.JSON(&b, contractResult()); err != nil {
		t.Fatalf("JSON: %v", err)
	}

	var doc struct {
		Candidates []struct {
			Child struct{ Column string } `json:"child"`
		} `json:"candidates"`
		Discarded []struct {
			Child struct{ Column string } `json:"child"`
		} `json:"discarded"`
	}
	if err := json.Unmarshal(b.Bytes(), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(doc.Discarded) != 1 {
		t.Fatalf("expected the discarded candidate in its own field, got %d", len(doc.Discarded))
	}
	for _, c := range doc.Candidates {
		if c.Child.Column == "status_id" {
			t.Error("a discarded candidate must not appear among the survivors")
		}
	}
}

// TestSchemaVersionIsAlwaysPresent guards the field the whole contract hangs
// from: without it a consumer cannot tell which shape it is reading.
func TestSchemaVersionIsAlwaysPresent(t *testing.T) {
	for name, r := range map[string]*model.Result{
		"populated": contractResult(),
		"empty":     model.NewResult(goldenVersion, "", goldenTime(), model.Coverage{}),
	} {
		var b bytes.Buffer
		if err := report.JSON(&b, r); err != nil {
			t.Fatalf("%s: JSON: %v", name, err)
		}

		var doc map[string]any
		if err := json.Unmarshal(b.Bytes(), &doc); err != nil {
			t.Fatalf("%s: unmarshal: %v", name, err)
		}

		if doc["schema_version"] != model.SchemaVersion {
			t.Errorf("%s: schema_version is %v, want %q", name, doc["schema_version"], model.SchemaVersion)
		}
		if _, ok := doc["coverage"]; !ok {
			t.Errorf("%s: coverage must accompany every document, including the empty one", name)
		}
	}
}
