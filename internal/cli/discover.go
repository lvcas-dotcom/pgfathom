package cli

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/lvcas-dotcom/pgfathom/internal/buildinfo"
	"github.com/lvcas-dotcom/pgfathom/internal/catalog"
	"github.com/lvcas-dotcom/pgfathom/internal/db"
	"github.com/lvcas-dotcom/pgfathom/internal/discovery"
	"github.com/lvcas-dotcom/pgfathom/internal/infer"
	"github.com/lvcas-dotcom/pgfathom/internal/profile"
	"github.com/lvcas-dotcom/pgfathom/internal/report"
	"github.com/lvcas-dotcom/pgfathom/internal/validate"
)

type discoverOptions struct {
	connection      connectionOptions
	profile         string
	minScore        float64
	format          string
	out             string
	includeRejected bool
	noDetect        bool
	noStats         bool
	noProbe         bool
	full            bool
	sampleRows      int64
}

// Output formats. sql is deliberately absent from stdout: a format that fits in
// a pipe invites the pipe, and then the mandatory-review header is decoration
// nobody read because nobody opened the file.
const (
	formatTable = "table"
	formatJSON  = "json"
	formatSQL   = "sql"
)

// checkOutput validates the pair of flags every result-producing command
// shares. An unknown format is a usage error, never a silent fall back to a
// default: degrading quietly would hand back a report in a shape the caller
// did not ask for.
func checkOutput(format, out string) error {
	switch format {
	case formatTable, formatJSON:
	case formatSQL:
		if out == "" {
			return UsageError(fmt.Errorf(
				"--format sql writes reviewable files, not stdout: pass --out <directory>"))
		}
	default:
		return UsageError(fmt.Errorf("invalid --format %q: want table, json or sql", format))
	}
	return nil
}

// validationStage tells the user what the verdicts in front of them can and
// cannot claim. The sampled warning is not a footnote: a clean sample is not
// evidence of absence, and the report has to say so where it cannot be missed.
//
// It reports the mode that ran, not the one that was asked for. Sampling is
// decided per candidate, and a table that fits the target is read whole — so a
// run started without --full can still end up conclusive throughout. Announcing
// "nothing here is confirmed" above a list of confirmations is the tool
// contradicting itself, which costs more trust than the caveat buys.
func validationStage(full bool, sampleRows int64, sampled bool) string {
	switch {
	case full:
		return "full validation — every row was examined; verdicts are conclusive"
	case sampled:
		return fmt.Sprintf("! sampled validation (%d rows/table target) — orphan counts are floors, "+
			"nothing here is confirmed; re-run with --full to prove absence", sampleRows)
	default:
		return fmt.Sprintf("sampled mode (%d rows/table target), but every table fit the target "+
			"and was read whole; verdicts are conclusive", sampleRows)
	}
}

type connectionOptions struct {
	dsn              string
	schemas          []string
	allSchemas       bool
	excludeSchemas   []string
	exclude          []string
	statementTimeout time.Duration
	lockTimeout      time.Duration
	idleTxTimeout    time.Duration
	concurrency      int

	// schemaFlagSet reports whether --schema was actually given. Comparing the
	// value against the default instead would read "--schema public
	// --all-schemas" as if the flag were absent, and that is precisely the
	// ambiguous command line the exclusivity exists to refuse.
	schemaFlagSet func() bool
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

// registerConnectionFlags registers the flags every catalog-reading command
// shares.
//
// Scope belongs to the connection rather than to the command: an audit that only
// ever sees public while discover sees the whole database would be an
// inconsistency with nothing behind it, and two copies of this list is how that
// starts.
func registerConnectionFlags(cmd *cobra.Command, c *connectionOptions) {
	f := cmd.Flags()
	f.StringVar(&c.dsn, "dsn", "",
		"connection string (visible in ps and shell history; prefer "+db.EnvDSN+")")
	f.StringSliceVar(&c.schemas, "schema", c.schemas, "schemas to analyze")
	f.BoolVar(&c.allSchemas, "all-schemas", false,
		"analyze every non-system schema the role can access; cannot be combined with --schema")
	f.StringSliceVar(&c.excludeSchemas, "exclude-schema", nil,
		"glob patterns of schemas to drop from scope")
	f.StringSliceVar(&c.exclude, "exclude", nil, "glob patterns of tables to skip")
	f.DurationVar(&c.statementTimeout, "timeout", c.statementTimeout, "statement timeout per query")
	f.DurationVar(&c.lockTimeout, "lock-timeout", c.lockTimeout, "lock timeout per query")
	f.DurationVar(&c.idleTxTimeout, "idle-tx-timeout", c.idleTxTimeout, "idle transaction timeout")
	f.IntVar(&c.concurrency, "concurrency", c.concurrency, "maximum simultaneous queries")

	c.schemaFlagSet = func() bool { return f.Changed("schema") }
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

Candidates are raised from names, types and catalog metadata, then checked
against the data itself: every verdict comes with the containment and orphan
counts that produced it, and every score with its signals, so both can be
argued with rather than merely accepted.

Sampled validation is the default and cannot confirm anything — it is triage.
Pass --full for the conclusive mode.

With --out, the findings are also written as reviewable .sql files: the DDL for
what the data confirms, and the orphan queries for what it contradicts.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDiscover(cmd.Context(), streams, opts)
		},
	}

	registerConnectionFlags(cmd, &opts.connection)

	f := cmd.Flags()
	f.StringVar(&opts.profile, "profile", opts.profile,
		"naming profile: a built-in name or a path to a TOML file")
	f.Float64Var(&opts.minScore, "min-score", opts.minScore,
		"discard candidates scoring below this")
	f.StringVar(&opts.format, "format", opts.format, "output format: table, json or sql")
	f.StringVar(&opts.out, "out", "",
		"directory for the reviewable .sql artifacts; required by --format sql")
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
	if err := checkOutput(opts.format, opts.out); err != nil {
		return err
	}
	if opts.minScore < 0 || opts.minScore > 1 {
		return UsageError(fmt.Errorf("invalid --min-score %.2f: want a value between 0 and 1", opts.minScore))
	}

	naming, err := profile.Load(opts.profile)
	if err != nil {
		return UsageError(err)
	}

	// A warning clears the progress line before writing: the tail of a longer
	// line left at the end of a warning reads as a defect.
	progress := streams.Progress()
	warn := func(msg string) {
		progress.Clear()
		_, _ = fmt.Fprintln(streams.Err, "warning: "+msg)
	}

	pool, scope, err := connect(ctx, opts.connection, warn)
	if err != nil {
		return err
	}
	defer pool.Close()

	version, _, _ := buildinfo.Resolve()

	run, err := discovery.Run(ctx, pool, discovery.Options{
		Profile:     naming,
		Scope:       scope,
		Exclude:     opts.connection.exclude,
		MinScore:    opts.minScore,
		ToolVersion: version,
		NoDetect:    opts.noDetect,
		NoStats:     opts.noStats,
		NoProbe:     opts.noProbe,
		Validation: validate.Options{
			Full:        opts.full,
			TargetRows:  opts.sampleRows,
			Timeout:     opts.connection.statementTimeout,
			Concurrency: opts.connection.concurrency,
		},
		Warn:     func(_ discovery.Stage, msg string) { warn(msg) },
		Progress: progress.Report,
	})
	progress.Clear()
	if err != nil {
		return err
	}

	result := run.Result

	// Discarded live in their own field rather than beside the survivors: a
	// consumer must not have to inspect a score to learn whether a candidate
	// made it through triage.
	if opts.includeRejected {
		result.Discarded = run.Discarded
	}

	if opts.out != "" {
		artifacts := report.DiscoverArtifacts(result)
		if err := report.WriteArtifacts(opts.out, artifacts); err != nil {
			return err
		}
		if opts.format == formatSQL {
			return report.Manifest(streams.Out, opts.out, artifacts)
		}
	}

	if opts.format == formatJSON {
		return report.JSON(streams.Out, result)
	}

	return report.Discover(streams.Out, report.DiscoverView{
		Result:          result,
		Discarded:       run.Discarded,
		MinScore:        opts.minScore,
		ShowDiscarded:   opts.includeRejected,
		ValidationStage: validationStage(opts.full, opts.sampleRows, result.Sampled()),
		Detection:       run.Detection,
		Emphasis:        report.Emphasis(streams.Color()),
	})
}

// connect resolves credentials, opens the pool, resolves the schema scope and
// warns about write privileges.
func connect(ctx context.Context, opts connectionOptions, warn func(string)) (*db.Pool, *catalog.Scope, error) {
	// Checked before the connection, because no precedence between these two is
	// defensible: either answer analyzes a set the command line does not appear
	// to ask for.
	if opts.allSchemas && opts.schemaFlagSet != nil && opts.schemaFlagSet() {
		return nil, nil, UsageError(errors.New(
			"--schema and --all-schemas are mutually exclusive: pass an explicit list or the whole database, not both"))
	}

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

	scope, err := catalog.ResolveScope(ctx, pool, catalog.ScopeOptions{
		Schemas:        opts.schemas,
		All:            opts.allSchemas,
		ExcludeSchemas: opts.excludeSchemas,
	})
	if err != nil {
		pool.Close()
		if errors.Is(err, catalog.ErrEmptyScope) {
			return nil, nil, UsageError(err)
		}
		return nil, nil, err
	}

	// A failure here means the role cannot see the privilege catalog, which is
	// not a reason to stop.
	if writable, err := pool.HasWritePrivileges(ctx, scope.Schemas); err != nil {
		warn("could not verify privileges: " + err.Error())
	} else if writable {
		warn("the connected role can write to tables in scope; " +
			"a dedicated read-only role is recommended")
	}

	return pool, scope, nil
}
