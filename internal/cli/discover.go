package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/lvcas-dotcom/pgfathom/internal/buildinfo"
	"github.com/lvcas-dotcom/pgfathom/internal/catalog"
	"github.com/lvcas-dotcom/pgfathom/internal/db"
	"github.com/lvcas-dotcom/pgfathom/internal/infer"
	"github.com/lvcas-dotcom/pgfathom/internal/model"
	"github.com/lvcas-dotcom/pgfathom/internal/profile"
	"github.com/lvcas-dotcom/pgfathom/internal/report"
	"github.com/lvcas-dotcom/pgfathom/internal/sqlprobe"
	"github.com/lvcas-dotcom/pgfathom/internal/stats"
	"github.com/lvcas-dotcom/pgfathom/internal/validate"
)

type discoverOptions struct {
	connection      connectionOptions
	profile         string
	minScore        float64
	format          string
	includeRejected bool
	noDetect        bool
	noStats         bool
	noProbe         bool
	full            bool
	sampleRows      int64
}

// validationStage tells the user what the verdicts in front of them can and
// cannot claim. The sampled warning is not a footnote: a clean sample is not
// evidence of absence, and the report has to say so where it cannot be missed.
func validationStage(full bool, sampleRows int64) string {
	if full {
		return "full validation — every row was examined; verdicts are conclusive"
	}
	return fmt.Sprintf("! sampled validation (%d rows/table target) — orphan counts are floors, "+
		"nothing here is confirmed; re-run with --full to prove absence", sampleRows)
}

type connectionOptions struct {
	dsn              string
	schemas          []string
	exclude          []string
	statementTimeout time.Duration
	lockTimeout      time.Duration
	idleTxTimeout    time.Duration
	concurrency      int
}

func defaultConnectionOptions() connectionOptions {
	return connectionOptions{
		schemas:          []string{"public"},
		statementTimeout: db.DefaultStatementTimeout,
		lockTimeout:      db.DefaultLockTimeout,
		idleTxTimeout:    db.DefaultIdleTxTimeout,
		concurrency:      db.DefaultConcurrency,
	}
}

func newDiscoverCommand(streams *Streams) *cobra.Command {
	opts := &discoverOptions{
		connection: defaultConnectionOptions(),
		profile:    profile.DefaultName,
		minScore:   infer.DefaultMinScore,
		format:     "table",
	}

	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Infer relationships the schema never declared",
		Long: `Infer foreign keys that exist in the data but not in the catalog.

This version generates and scores candidates from names, types and catalog
metadata. It does NOT read your data, so nothing it reports is confirmed — every
candidate is a question worth asking, not an answer.

Every score comes with the signals that produced it, so it can be argued with.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDiscover(cmd.Context(), streams, opts)
		},
	}

	f := cmd.Flags()
	c := &opts.connection
	f.StringVar(&c.dsn, "dsn", "",
		"connection string (visible in ps and shell history; prefer "+db.EnvDSN+")")
	f.StringSliceVar(&c.schemas, "schema", c.schemas, "schemas to analyze")
	f.StringSliceVar(&c.exclude, "exclude", nil, "glob patterns of tables to skip")
	f.DurationVar(&c.statementTimeout, "timeout", c.statementTimeout, "statement timeout per query")
	f.DurationVar(&c.lockTimeout, "lock-timeout", c.lockTimeout, "lock timeout per query")
	f.DurationVar(&c.idleTxTimeout, "idle-tx-timeout", c.idleTxTimeout, "idle transaction timeout")
	f.IntVar(&c.concurrency, "concurrency", c.concurrency, "maximum simultaneous queries")

	f.StringVar(&opts.profile, "profile", opts.profile,
		"naming profile: a built-in name or a path to a TOML file")
	f.Float64Var(&opts.minScore, "min-score", opts.minScore,
		"discard candidates scoring below this")
	f.StringVar(&opts.format, "format", opts.format, "output format: table or json")
	f.BoolVar(&opts.includeRejected, "include-rejected", false,
		"also show candidates discarded by the threshold")
	f.BoolVar(&opts.noDetect, "no-detect-naming", false,
		"do not read naming conventions from the schema; use only the profile")
	f.BoolVar(&opts.noStats, "no-stats", false,
		"do not use planner statistics to prefilter candidates")
	f.BoolVar(&opts.noProbe, "no-probe", false,
		"do not mine join predicates from view and function definitions")
	f.BoolVar(&opts.full, "full", false,
		"validate against every row; the only mode that can confirm a relationship")
	f.Int64Var(&opts.sampleRows, "sample-rows", validate.DefaultTargetRows,
		"target rows per table in sampled validation")

	return cmd
}

func runDiscover(ctx context.Context, streams *Streams, opts *discoverOptions) error {
	if opts.format != "table" && opts.format != "json" {
		return UsageError(fmt.Errorf("invalid --format %q: want table or json", opts.format))
	}
	if opts.minScore < 0 || opts.minScore > 1 {
		return UsageError(fmt.Errorf("invalid --min-score %.2f: want a value between 0 and 1", opts.minScore))
	}

	naming, err := profile.Load(opts.profile)
	if err != nil {
		return UsageError(err)
	}

	warn := func(msg string) { _, _ = fmt.Fprintln(streams.Err, "warning: "+msg) }

	pool, schemas, err := connect(ctx, opts.connection, warn)
	if err != nil {
		return err
	}
	defer pool.Close()

	cat, err := catalog.Read(ctx, pool, catalog.Options{Schemas: schemas, Exclude: opts.connection.exclude})
	if err != nil {
		return err
	}

	// Detection is on by default: whoever needs it is precisely whoever does not
	// know they need it. Measured against real schemas, a missing local
	// convention cost 78 points of recall and failed silently.
	var detection model.NamingDetection
	active := naming
	if !opts.noDetect {
		detection = naming.Detect(cat.Schemas)
		active = naming.WithDetected(detection)
	}

	// Usage evidence is what breaks the ceiling of name matching. A failure to
	// read it costs signal, never correctness, so it degrades to a warning.
	var evidence sqlprobe.Evidence
	if !opts.noProbe {
		probed, err := sqlprobe.Probe(ctx, pool, cat.Schemas)
		if err != nil {
			warn("usage evidence skipped: " + err.Error())
		} else {
			evidence = *probed
		}
	}

	inferred := infer.Generate(cat.Schemas, infer.Options{
		Profile:  active,
		MinScore: opts.minScore,
		Evidence: evidence.Joins,
	})

	coverage := cat.Coverage
	coverage.CandidatesFound = len(inferred.Candidates) + len(inferred.Discarded)
	coverage.PgStatStatements = evidence.StatementsAvailable

	// The prefilter runs on the threshold survivors only: every candidate it
	// kills is an anti-join that validation will not fire against someone's
	// production server. A failure to read statistics is degraded to a warning,
	// never to an opinion about candidates.
	if !opts.noStats {
		pre, err := stats.Prefilter(ctx, pool, cat.Schemas, inferred.Candidates, stats.Options{})
		if err != nil {
			warn("statistical prefilter skipped: " + err.Error())
		} else {
			inferred.Candidates = pre.Kept
			inferred.Discarded = append(inferred.Discarded, pre.Rejected...)
			coverage.StatsPrefilter = true
			coverage.CandidatesStatsChecked = pre.Checked
			coverage.CandidatesStatsRejected = len(pre.Rejected)
			coverage.CandidatesWithoutStats = pre.NoStats
		}
	}

	validated, err := validate.Run(ctx, pool, cat.Schemas, inferred.Candidates, validate.Options{
		Full:        opts.full,
		TargetRows:  opts.sampleRows,
		Timeout:     opts.connection.statementTimeout,
		Concurrency: opts.connection.concurrency,
	})
	if err != nil {
		return err
	}
	inferred.Candidates = validated.Candidates
	coverage.CandidatesValidated = validated.Validated
	coverage.CandidatesTimedOut = validated.TimedOut

	version, _, _ := buildinfo.Resolve()
	result := model.NewResult(version, naming.Name, time.Now().UTC(), coverage)
	result.ServerVersion = pool.ServerVersion()
	result.Naming = detection
	result.Schemas = cat.Schemas
	result.Candidates = inferred.Candidates
	result.Findings = observations(inferred)

	if opts.includeRejected {
		result.Candidates = append(result.Candidates, inferred.Discarded...)
	}

	if opts.format == "json" {
		return report.JSON(streams.Out, result)
	}

	return report.Discover(streams.Out, report.DiscoverView{
		Result:          result,
		Discarded:       inferred.Discarded,
		MinScore:        opts.minScore,
		ShowDiscarded:   opts.includeRejected,
		ValidationStage: validationStage(opts.full, opts.sampleRows),
		Detection:       detection,
	})
}

// observations turns what inference recognized but could not use into findings,
// so a deliberate limitation reads as an observation rather than as silence.
func observations(res *infer.Result) []model.Finding {
	out := make([]model.Finding, 0, len(res.Polymorphic)+len(res.Skipped))

	for _, p := range res.Polymorphic {
		out = append(out, model.Finding{
			Kind:   model.FindingPolymorphicPair,
			Object: p.Table + "." + p.ReferenceColumn,
			Detail: "only meaningful beside " + p.TypeColumn + "; polymorphic references are out of scope",
		})
	}

	for _, s := range res.Skipped {
		out = append(out, model.Finding{
			Kind:   model.FindingUnsupportedTarget,
			Object: s.Child.String(),
			Detail: string(s.Reason) + " (" + s.Target + ")",
		})
	}

	return out
}

// connect resolves credentials, opens the pool and warns about write privileges.
func connect(ctx context.Context, opts connectionOptions, warn func(string)) (*db.Pool, []string, error) {
	dsn, err := db.ResolveDSN(opts.dsn, warn)
	if err != nil {
		return nil, nil, err
	}

	pool, err := db.Open(ctx, db.Config{
		DSN:              dsn,
		StatementTimeout: opts.statementTimeout,
		LockTimeout:      opts.lockTimeout,
		IdleTxTimeout:    opts.idleTxTimeout,
		Concurrency:      opts.concurrency,
	})
	if err != nil {
		return nil, nil, err
	}

	schemas := catalog.SortedSchemas(opts.schemas)

	// A failure here means the role cannot see the privilege catalog, which is
	// not a reason to stop.
	if writable, err := pool.HasWritePrivileges(ctx, schemas); err != nil {
		warn("could not verify privileges: " + err.Error())
	} else if writable {
		warn("the connected role can write to tables in scope; " +
			"a dedicated read-only role is recommended")
	}

	return pool, schemas, nil
}
