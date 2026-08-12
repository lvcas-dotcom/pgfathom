// Package cli assembles the command tree and the conventions every command
// inherits: stream discipline, colour resolution, exit codes, logging and
// cancellation.
package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/lvcas-dotcom/pgfathom/internal/buildinfo"
)

type globalOptions struct {
	color    string
	logLevel string
}

// NewRootCommand builds the command tree writing to the given streams.
func NewRootCommand(streams *Streams) *cobra.Command {
	root, _ := newRootCommand(streams)
	return root
}

// newRootCommand also returns a flag reporting whether execution got past
// parsing.
//
// Cobra reports "unknown command", "unknown flag" and argument-count failures
// as ordinary errors, and matching on their message text would break the moment
// cobra rewords anything. All three happen before PersistentPreRunE, so the
// position in the lifecycle is the reliable signal.
func newRootCommand(streams *Streams) (*cobra.Command, *bool) {
	opts := &globalOptions{color: string(ColorAuto), logLevel: "warn"}
	parsed := new(bool)

	root := &cobra.Command{
		Use:   "pgfathom",
		Short: "Sound the depth of a legacy PostgreSQL schema",
		Long: `pgfathom finds the relationships your database has but never declared,
and proves them against the data instead of guessing from column names.

It is strictly read-only: it never issues a statement that modifies the
database under analysis, and no value from your tables ever appears in its
output, logs or generated files.`,

		SilenceUsage:  true,
		SilenceErrors: true,

		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			mode := ColorMode(opts.color)
			switch mode {
			case ColorAuto, ColorAlways, ColorNever:
			default:
				return UsageError(fmt.Errorf("invalid --color %q: want auto, always or never", opts.color))
			}
			streams.color = resolveColor(mode, os.Stdout)

			level, err := parseLogLevel(opts.logLevel)
			if err != nil {
				return UsageError(err)
			}
			// Diagnostics go to Err, never to Out. See Streams.
			slog.SetDefault(slog.New(slog.NewTextHandler(streams.Err, &slog.HandlerOptions{Level: level})))

			cmd.SilenceUsage = true
			*parsed = true
			return nil
		},

		// Running a tool with no arguments to find out what it does is not an
		// error.
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	root.SetOut(streams.Out)
	root.SetErr(streams.Err)
	root.SetIn(streams.In)

	root.PersistentFlags().StringVar(&opts.color, "color", opts.color,
		"colour output: auto, always or never")
	root.PersistentFlags().StringVar(&opts.logLevel, "log-level", opts.logLevel,
		"diagnostic verbosity on stderr: debug, info, warn or error")

	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return UsageError(err)
	})

	root.AddCommand(newVersionCommand(streams))
	root.AddCommand(newAuditCommand(streams))
	root.AddCommand(newDiscoverCommand(streams))
	root.AddCommand(newSetupCommand(streams, root))

	return root, parsed
}

func newVersionCommand(streams *Streams) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version, commit and build date",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			version, commit, date := buildinfo.Resolve()
			_, err := fmt.Fprintf(streams.Out, "pgfathom %s\ncommit  %s\nbuilt   %s\n", version, commit, date)
			return err
		},
	}
}

func parseLogLevel(s string) (slog.Level, error) {
	switch s {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid --log-level %q: want debug, info, warn or error", s)
	}
}

// Run executes the command tree and returns the process exit code.
//
// The root context is cancelled on SIGINT and SIGTERM. Nothing needs cancelling
// yet, which is why it is established now: retrofitting it once three layers
// exist reliably leaves one path uncovered, and an uncovered path is a query
// left running on someone's production server.
func Run(args []string, streams *Streams) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	root, parsed := newRootCommand(streams)
	root.SetArgs(args)

	err := root.ExecuteContext(ctx)

	// A cancelled context outranks whatever error the interruption produced.
	if ctx.Err() != nil && errors.Is(ctx.Err(), context.Canceled) {
		_, _ = fmt.Fprintln(streams.Err, "interrupted")
		return ExitInterrupted
	}

	if err == nil {
		return ExitOK
	}

	_, _ = fmt.Fprintf(streams.Err, "error: %v\n", err)
	if !*parsed {
		// The run never got past parsing, so this is a usage problem however
		// cobra worded it.
		return ExitUsage
	}
	return ExitCodeFor(err)
}
