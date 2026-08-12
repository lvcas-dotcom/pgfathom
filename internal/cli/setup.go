package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/lvcas-dotcom/pgfathom/internal/catalog"
	"github.com/lvcas-dotcom/pgfathom/internal/db"
)

// dsnEnv is where a connection string belongs: an environment variable is not
// visible in ps and does not land in shell history, which a --dsn flag does.
const dsnEnv = "PGFATHOM_DSN"

// plan is what the answers compose. It exists as a value so the command that
// gets printed and the command that gets run are the same object.
type plan struct {
	dsnFromEnv bool
	dsn        string

	schemas []string
	full    bool
	out     string
}

// Command renders the plan as the command line that reproduces it. Printing it
// is the point of the guide: the goal is not to spare anyone the twenty-one
// flags, it is to teach them. Whoever sees the command their own answers
// composed copies it, keeps it, and never needs the guide again.
func (p plan) Command() string {
	parts := []string{"pgfathom", "discover"}

	if !p.dsnFromEnv {
		parts = append(parts, "--dsn", quoteArg(p.dsn))
	}
	if len(p.schemas) > 0 {
		parts = append(parts, "--schema", strings.Join(p.schemas, ","))
	}
	if p.full {
		parts = append(parts, "--full")
	}
	if p.out != "" {
		parts = append(parts, "--out", quoteArg(p.out))
	}
	return strings.Join(parts, " ")
}

// Args is the same plan as arguments, so what runs is what was shown.
func (p plan) Args() []string {
	args := []string{"discover"}
	if !p.dsnFromEnv {
		args = append(args, "--dsn", p.dsn)
	}
	if len(p.schemas) > 0 {
		args = append(args, "--schema", strings.Join(p.schemas, ","))
	}
	if p.full {
		args = append(args, "--full")
	}
	if p.out != "" {
		args = append(args, "--out", p.out)
	}
	return args
}

func quoteArg(s string) string {
	if strings.ContainsAny(s, " \t'\"$`\\") {
		return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
	}
	return s
}

func newSetupCommand(streams *Streams, root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "setup",
		Short: "Guide a first run and print the command it composes",
		Long: "Ask what a first run needs — connection, scope, validation mode, where the\n" +
			"reviewable SQL goes — and print the discover command those answers compose.\n\n" +
			"It never asks for a password: the connection string comes from " + dsnEnv + ",\n" +
			"or is built from host and database and left to your ~/.pgpass. And it runs\n" +
			"nothing without showing the command first and asking.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runSetup(cmd.Context(), streams, root)
		},
	}
}

func runSetup(ctx context.Context, streams *Streams, root *cobra.Command) error {
	// Checked before anything is asked. Unlike emphasis and progress, a guide
	// has no degraded mode: without a terminal it would wait for input that
	// never arrives and hang the process.
	if !isTerminal(os.Stdin) || !isTerminal(os.Stderr) {
		return UsageError(errors.New(
			"setup needs an interactive terminal; run `pgfathom discover --help` for the flags directly"))
	}

	p, err := composePlan(ctx, streams)
	if err != nil {
		if errors.Is(err, errCancelled) {
			_, _ = fmt.Fprintln(streams.Err, "\ncancelled; nothing was run")
			return nil
		}
		return err
	}

	// The command goes to stdout, alone, so it can be copied or piped. Every
	// prompt above it was drawn on stderr for exactly this reason.
	_, _ = fmt.Fprintf(streams.Err, "\n  %s\n\n", titleStyle.Render("The command your answers compose:"))
	_, _ = fmt.Fprintln(streams.Out, p.Command())

	ok, err := confirm("Run it now?")
	if err != nil {
		if errors.Is(err, errCancelled) {
			return nil
		}
		return err
	}
	if !ok {
		_, _ = fmt.Fprintln(streams.Err, "\nnot run; the command above reproduces these answers")
		return nil
	}

	_, _ = fmt.Fprintln(streams.Err)
	root.SetArgs(p.Args())
	return root.ExecuteContext(ctx)
}

func composePlan(ctx context.Context, streams *Streams) (plan, error) {
	var p plan

	if err := askConnection(&p); err != nil {
		return p, err
	}

	// Connecting before asking anything else is what makes the scope question
	// answerable: the list of schemas comes from the server, not from guesses.
	cfg := db.DefaultConfig()
	cfg.DSN = p.dsn
	pool, err := db.Open(ctx, cfg)
	if err != nil {
		return p, err
	}
	defer pool.Close()

	_, _ = fmt.Fprintf(streams.Err, "\n  connected to PostgreSQL %s\n", pool.ServerVersion())

	if err := askScope(ctx, pool, &p); err != nil {
		return p, err
	}
	if err := askMode(&p); err != nil {
		return p, err
	}
	return p, askArtifacts(&p)
}

// askConnection never reads a password. The environment variable is preferred
// when it is already set, and otherwise the parts are assembled and the
// credential left to ~/.pgpass — which is what this tool tells people to do
// anyway, on the --dsn flag's own help text.
func askConnection(p *plan) error {
	if dsn := os.Getenv(dsnEnv); dsn != "" {
		use, err := selectOne(dsnEnv+" is set. Use it?", []Option{
			{Label: "Yes", Detail: redact(dsn)},
			{Label: "No", Detail: "enter the connection details instead"},
		})
		if err != nil {
			return err
		}
		if use == 0 {
			p.dsn, p.dsnFromEnv = dsn, true
			return nil
		}
	}

	host, err := askLine("Host", "the server to analyse", "localhost", "localhost")
	if err != nil {
		return err
	}
	port, err := askLine("Port", "", "5432", "5432")
	if err != nil {
		return err
	}
	database, err := askLine("Database", "", "postgres", "postgres")
	if err != nil {
		return err
	}
	user, err := askLine("User", "a read-only role is recommended", os.Getenv("USER"), os.Getenv("USER"))
	if err != nil {
		return err
	}

	// No password, on purpose. Reading one would put it in this process's
	// memory and on somebody's screen, and the tool has stayed out of the
	// credential business deliberately.
	p.dsn = fmt.Sprintf("postgres://%s@%s:%s/%s", user, host, port, database)
	return nil
}

func askScope(ctx context.Context, pool *db.Pool, p *plan) error {
	schemas, err := catalog.SummarizeSchemas(ctx, pool)
	if err != nil {
		return err
	}
	if len(schemas) == 0 {
		return errors.New("the connected role cannot open any schema")
	}

	options := make([]Option, 0, len(schemas))
	preselected := []int{}
	for i, s := range schemas {
		options = append(options, Option{
			Label:  s.Name,
			Detail: plural(s.Tables, "table", "tables"),
		})
		if s.Name == "public" {
			preselected = append(preselected, i)
		}
	}

	// The default is offered, not imposed. A server with dozens of schemas and
	// almost nothing in public is ordinary in public-sector systems, and
	// pointing at the default there is the most common way a first run ends in
	// "nothing found" against a database full of relationships.
	picked, err := selectMany("Which schemas?", options, preselected...)
	if err != nil {
		return err
	}
	for _, i := range picked {
		p.schemas = append(p.schemas, schemas[i].Name)
	}
	return nil
}

func askMode(p *plan) error {
	mode, err := selectOne("How thoroughly should relationships be validated?", []Option{
		{Label: "Sampled", Detail: "faster; finds broken relationships, confirms none"},
		{Label: "Full", Detail: "reads every row; the only mode that can confirm"},
	})
	if err != nil {
		return err
	}
	p.full = mode == 1
	return nil
}

func askArtifacts(p *plan) error {
	want, err := selectOne("Write the reviewable SQL to disk?", []Option{
		{Label: "No", Detail: "just the report on screen"},
		{Label: "Yes", Detail: "one file per category, for you to read before running any of it"},
	})
	if err != nil {
		return err
	}
	if want == 0 {
		return nil
	}

	dir, err := askLine("Which directory?", "it is created if missing", "./pgfathom-out", "./pgfathom-out")
	if err != nil {
		return err
	}
	p.out = dir
	return nil
}

// redact hides everything between the scheme and the host, which is where a
// password lives when somebody put one in the connection string.
func redact(dsn string) string {
	at := strings.LastIndex(dsn, "@")
	scheme := strings.Index(dsn, "://")
	if at < 0 || scheme < 0 || at < scheme {
		return dsn
	}

	creds := dsn[scheme+3 : at]
	if i := strings.Index(creds, ":"); i >= 0 {
		creds = creds[:i] + ":•••"
	}
	return dsn[:scheme+3] + creds + dsn[at:]
}

func plural(n int, one, many string) string {
	word := many
	if n == 1 {
		word = one
	}
	return strconv.Itoa(n) + " " + word
}
