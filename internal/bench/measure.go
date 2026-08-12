//go:build benchmark

package bench

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/lvcas-dotcom/pgfathom/internal/catalog"
	"github.com/lvcas-dotcom/pgfathom/internal/db"
	"github.com/lvcas-dotcom/pgfathom/internal/discovery"
	"github.com/lvcas-dotcom/pgfathom/internal/model"
	"github.com/lvcas-dotcom/pgfathom/internal/profile"
	"github.com/lvcas-dotcom/pgfathom/internal/validate"
)

// Regime is how much of the declared integrity was taken away before asking.
// The two are different questions, and neither answers the other.
type Regime string

const (
	// RegimePartial removes half the keys and measures recovery of that half.
	// The half left behind is evidence: naming detection reads it, exactly as it
	// would in a database that declared part of its integrity and forgot the
	// rest — which is the ordinary case, and the one a user is in.
	RegimePartial Regime = "partial"

	// RegimeGreenfield removes the rest too. It answers the hardest question,
	// about a database that declares no integrity at all, and it is where the
	// tool earns its argument. Measured alone it understates the tool: with
	// nothing declared, detection has nothing to learn from.
	RegimeGreenfield Regime = "greenfield"
)

// Regimes are measured in this order, because greenfield starts from exactly
// the state partial leaves behind.
var Regimes = []Regime{RegimePartial, RegimeGreenfield}

// Label is how the regime reads in the published report.
func (r Regime) Label() string {
	switch r {
	case RegimePartial:
		return "partial — half the keys declared"
	default:
		return "greenfield — nothing declared"
	}
}

// Split divides the ground truth in two, deterministically: the relations are
// ordered and alternate positions go to each half.
//
// Alternating rather than cutting the list in two spreads the removed keys
// across tables instead of concentrating them in a prefix of the alphabet. And
// ordering rather than sampling is what makes the published number stable: the
// recall report is a versioned file, and its diff has to mean "the behaviour
// changed", never "the draw came out differently".
func Split(truth []Relation) (removed, kept []Relation) {
	ordered := make([]Relation, len(truth))
	copy(ordered, truth)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].key() < ordered[j].key() })

	for i, r := range ordered {
		if i%2 == 0 {
			removed = append(removed, r)
			continue
		}
		kept = append(kept, r)
	}
	return removed, kept
}

// Config is one way of running the pipeline. The three of them are the
// decomposition the README exists to publish: how much name matching recovers
// on its own, and what each further source of signal adds.
type Config struct {
	Name     string
	NoDetect bool
	NoProbe  bool
}

// Configs are cumulative, in order.
var Configs = []Config{
	{Name: "profile alone", NoDetect: true, NoProbe: true},
	{Name: "+ detection", NoProbe: true},
	{Name: "+ usage evidence"},
}

// Relation is one foreign key, reduced to what a candidate can be compared
// against: two keys, columns in key order.
type Relation struct {
	Child  model.KeyRef
	Parent model.KeyRef
}

// key is the comparison form: case-insensitive, order-preserving. Order is
// preserved because a key over the same columns in another order is a
// different constraint, and crediting it would inflate the number being
// published.
func (r Relation) key() string {
	return strings.ToLower(r.Child.String() + "→" + r.Parent.String())
}

func (r Relation) String() string { return r.Child.String() + " → " + r.Parent.String() }

// Measurement is one configuration, in one regime, against one schema.
type Measurement struct {
	Regime Regime
	Config string

	// Truth is the size of the ground truth this measurement was scored
	// against: half the keys in the partial regime, all of them in greenfield.
	Truth int

	Recovered  int
	Candidates int

	// RecoveredComposite is how much of the recovery came from keys of more
	// than one column. Composite support was a whole phase; the corpus is where
	// it either shows up in the number or does not.
	RecoveredComposite int

	// Unmatched is candidates that match no dropped key. They are NOT false
	// positives: in a real schema a true relationship that was never declared
	// is this tool's product. The count is published as what it is and enters
	// no error rate.
	Unmatched int

	Coverage model.Coverage
	Stages   []discovery.StageTiming
	Duration time.Duration
	Warnings []string
}

// Recall is the share of the ground truth recovered.
func (m Measurement) Recall() float64 {
	if m.Truth == 0 {
		return 0
	}
	return float64(m.Recovered) / float64(m.Truth)
}

// EntryResult is everything measured about one corpus entry.
type EntryResult struct {
	Entry         Entry
	ServerVersion string

	Tables int

	// Truth is the ground truth: keys declared in the measured schema before
	// the harness dropped them.
	Truth int

	// TruthComposite is how many of them span more than one column.
	TruthComposite int

	// OutOfScope is declared keys reaching outside the measured schema. They
	// are excluded from the denominator and reported, because a denominator
	// that quietly includes what was never in reach understates the tool.
	OutOfScope int

	// LoadFailures is how many statements of a local dump did not apply. Zero
	// for a corpus entry that came from a published schema; rarely zero for a
	// dump taken from a live database. Published, because a recall measured on
	// half a schema is a recall about a schema nobody has.
	LoadFailures int

	Measurements []Measurement
}

// Truth reads the declared foreign keys, before anything is modified.
//
// Both queries here skip inherited constraints — conparentid is set — because
// a partition's copy of its parent's key is not an independent declaration. It
// would inflate the denominator, and the server refuses to drop it on its own
// anyway: it goes when the partitioned parent's does.
//
// It reads them with its own query rather than through the tool's catalog
// reader. The ground truth has to be independent of the code being measured:
// a catalog bug that dropped keys would otherwise remove them from the
// denominator too, and the recall would look perfect for the wrong reason.
func Truth(ctx context.Context, conn *pgx.Conn, schema string) (inScope []Relation, outOfScope int, err error) {
	const query = `
		SELECT cn.nspname, c.relname, array_agg(ca.attname ORDER BY k.ord),
		       pn.nspname, p.relname, array_agg(pa.attname ORDER BY k.ord)
		FROM pg_constraint con
		JOIN pg_class c        ON c.oid = con.conrelid
		JOIN pg_namespace cn   ON cn.oid = c.relnamespace
		JOIN pg_class p        ON p.oid = con.confrelid
		JOIN pg_namespace pn   ON pn.oid = p.relnamespace
		JOIN LATERAL unnest(con.conkey, con.confkey)
		     WITH ORDINALITY AS k(child, parent, ord) ON true
		JOIN pg_attribute ca   ON ca.attrelid = con.conrelid AND ca.attnum = k.child
		JOIN pg_attribute pa   ON pa.attrelid = con.confrelid AND pa.attnum = k.parent
		WHERE con.contype = 'f' AND con.conparentid = 0
		GROUP BY con.oid, cn.nspname, c.relname, pn.nspname, p.relname
		ORDER BY 1, 2, 4, 5`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		return nil, 0, fmt.Errorf("reading declared keys: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var r Relation
		if err := rows.Scan(
			&r.Child.Schema, &r.Child.Table, &r.Child.Columns,
			&r.Parent.Schema, &r.Parent.Table, &r.Parent.Columns,
		); err != nil {
			return nil, 0, fmt.Errorf("scanning declared key: %w", err)
		}

		if r.Child.Schema != schema || r.Parent.Schema != schema {
			outOfScope++
			continue
		}
		inScope = append(inScope, r)
	}
	return inScope, outOfScope, rows.Err()
}

// DropForeignKeys removes every declared foreign key, which is what turns the
// schema into the question the tool is asked.
//
// This writes. It is the one place in the repository that does, and it is not
// the tool: the connection here belongs to the harness, and it points at a
// throwaway server the harness started. The connection handed to discovery is
// a *db.Pool, which refuses writes by session policy, and the integration test
// that proves it still runs.
func DropForeignKeys(ctx context.Context, conn *pgx.Conn, schema string, only []Relation) (int, error) {
	wanted := make(map[string]bool, len(only))
	for _, r := range only {
		wanted[r.key()] = true
	}

	const query = `
		SELECT n.nspname, c.relname, con.conname,
		       array_agg(ca.attname ORDER BY k.ord),
		       pn.nspname, p.relname, array_agg(pa.attname ORDER BY k.ord)
		FROM pg_constraint con
		JOIN pg_class c        ON c.oid = con.conrelid
		JOIN pg_namespace n    ON n.oid = c.relnamespace
		JOIN pg_class p        ON p.oid = con.confrelid
		JOIN pg_namespace pn   ON pn.oid = p.relnamespace
		JOIN LATERAL unnest(con.conkey, con.confkey)
		     WITH ORDINALITY AS k(child, parent, ord) ON true
		JOIN pg_attribute ca   ON ca.attrelid = con.conrelid AND ca.attnum = k.child
		JOIN pg_attribute pa   ON pa.attrelid = con.confrelid AND pa.attnum = k.parent
		WHERE con.contype = 'f' AND con.conparentid = 0 AND n.nspname = $1
		GROUP BY con.oid, n.nspname, c.relname, pn.nspname, p.relname
		ORDER BY 1, 2, 3`

	rows, err := conn.Query(ctx, query, schema)
	if err != nil {
		return 0, fmt.Errorf("listing constraints to drop: %w", err)
	}

	type target struct{ schema, table, name string }
	var targets []target

	for rows.Next() {
		var t target
		var rel Relation
		if err := rows.Scan(&t.schema, &t.table, &t.name,
			&rel.Child.Columns, &rel.Parent.Schema, &rel.Parent.Table, &rel.Parent.Columns,
		); err != nil {
			rows.Close()
			return 0, err
		}
		rel.Child.Schema, rel.Child.Table = t.schema, t.table

		// Only the half this regime is asking about.
		if len(wanted) > 0 && !wanted[rel.key()] {
			continue
		}
		targets = append(targets, t)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	for _, t := range targets {
		stmt := fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT %s",
			pgx.Identifier{t.schema, t.table}.Sanitize(),
			pgx.Identifier{t.name}.Sanitize())
		if _, err := conn.Exec(ctx, stmt); err != nil {
			return 0, fmt.Errorf("dropping %s on %s.%s: %w", t.name, t.schema, t.table, err)
		}
	}
	return len(targets), nil
}

// Measure runs one configuration and compares what came back with the ground
// truth. It calls the pipeline in process: measuring through the binary would
// mean reparsing output that does not carry the funnel, and reimplementing the
// sequence would measure the reimplementation.
func Measure(ctx context.Context, pool *db.Pool, e Entry, regime Regime, cfg Config, truth []Relation) (Measurement, error) {
	naming, err := profile.Embedded(e.Profile)
	if err != nil {
		return Measurement{}, fmt.Errorf("loading profile %q: %w", e.Profile, err)
	}

	m := Measurement{Regime: regime, Config: cfg.Name, Truth: len(truth)}

	opts := discovery.Options{
		Profile:     naming,
		Scope:       &catalog.Scope{Schemas: []string{e.Schema}, Total: 1},
		ToolVersion: "benchmark",
		NoDetect:    cfg.NoDetect,
		NoProbe:     cfg.NoProbe,
		Validation:  validate.Options{Full: true},
		Warn: func(stage discovery.Stage, msg string) {
			m.Warnings = append(m.Warnings, string(stage)+": "+msg)
		},
	}

	start := time.Now()
	res, err := discovery.Run(ctx, pool, opts)
	if err != nil {
		return Measurement{}, fmt.Errorf("running discovery: %w", err)
	}
	m.Duration = time.Since(start)

	m.Coverage = res.Result.Coverage
	m.Stages = res.Stages
	m.Candidates = len(res.Result.Candidates)
	m.Recovered, m.RecoveredComposite, m.Unmatched = compare(res.Result.Candidates, truth)

	return m, nil
}

// compare counts exact matches against the ground truth. Exact is the only
// honest bar: half a key is not half a recovery, it is a different proposal,
// and partial credit on a published metric is how a number stops meaning what
// its name says.
func compare(candidates []model.Candidate, truth []Relation) (recovered, composite, unmatched int) {
	want := make(map[string]bool, len(truth))
	for _, r := range truth {
		want[r.key()] = true
	}

	found := make(map[string]bool, len(candidates))

	for _, c := range candidates {
		k := Relation{Child: c.Child, Parent: c.Parent}.key()
		if !want[k] {
			unmatched++
			continue
		}
		if found[k] {
			continue
		}
		found[k] = true
		if c.Child.Composite() {
			composite++
		}
	}
	return len(found), composite, unmatched
}

// CountComposite is how many relations span more than one column.
func CountComposite(rs []Relation) int {
	n := 0
	for _, r := range rs {
		if r.Child.Composite() {
			n++
		}
	}
	return n
}
