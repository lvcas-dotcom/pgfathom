package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/lvcas-dotcom/pgfathom/internal/audit"
	"github.com/lvcas-dotcom/pgfathom/internal/buildinfo"
	"github.com/lvcas-dotcom/pgfathom/internal/catalog"
	"github.com/lvcas-dotcom/pgfathom/internal/db"
	"github.com/lvcas-dotcom/pgfathom/internal/model"
	"github.com/lvcas-dotcom/pgfathom/internal/report"
)

type auditOptions struct {
	connection connectionOptions
	format     string
	out        string
}

func newAuditCommand(streams *Streams) *cobra.Command {
	opts := &auditOptions{connection: defaultConnectionOptions(), format: "table"}

	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Report structural findings that need no inference",
		Long: `Report structural findings taken straight from the catalog.

audit makes no inferences: every finding it emits is a fact about the schema,
not a hypothesis about the data. It reports foreign keys declared NOT VALID and
never verified, and foreign keys with no index on the child side.

Relationship inference is a separate command; one does not substitute for the
other.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runAudit(cmd.Context(), streams, opts)
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
	f.StringVar(&opts.format, "format", opts.format, "output format: table, json or sql")
	f.StringVar(&opts.out, "out", "",
		"directory for the reviewable .sql artifacts; required by --format sql")

	return cmd
}

func runAudit(ctx context.Context, streams *Streams, opts *auditOptions) error {
	if err := checkOutput(opts.format, opts.out); err != nil {
		return err
	}

	started := time.Now()

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

	version, _, _ := buildinfo.Resolve()
	result := model.NewResult(version, "", time.Now().UTC(), cat.Coverage)
	result.ServerVersion = pool.ServerVersion()
	result.Schemas = cat.Schemas
	result.Findings = audit.Findings(cat.Schemas)
	result.Duration = time.Since(started)

	if opts.out != "" {
		artifacts := report.AuditArtifacts(result)
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
	return report.Terminal(streams.Out, result, report.Emphasis(streams.Color()))
}
