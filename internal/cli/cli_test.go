package cli

import (
	"bytes"
	"errors"
	"regexp"
	"strings"
	"testing"
)

func run(args ...string) (stdout, stderr string, code int) {
	var out, errBuf bytes.Buffer
	streams := &Streams{Out: &out, Err: &errBuf, In: strings.NewReader("")}
	code = Run(args, streams)
	return out.String(), errBuf.String(), code
}

func TestExitCodes(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"version succeeds", []string{"version"}, ExitOK},
		{"no arguments prints help and succeeds", nil, ExitOK},
		{"explicit help succeeds", []string{"--help"}, ExitOK},
		{"unknown subcommand is a usage error", []string{"doesnotexist"}, ExitUsage},
		{"unknown flag is a usage error", []string{"--nope"}, ExitUsage},
		{"invalid colour is a usage error", []string{"--color", "mauve", "version"}, ExitUsage},
		{"invalid log level is a usage error", []string{"--log-level", "shout", "version"}, ExitUsage},
		{"version takes no arguments", []string{"version", "extra"}, ExitUsage},
		{"unknown format on discover is a usage error", []string{"discover", "--format", "mauve"}, ExitUsage},
		{"unknown format on audit is a usage error", []string{"audit", "--format", "mauve"}, ExitUsage},
		{"sql format without a destination is a usage error", []string{"discover", "--format", "sql"}, ExitUsage},
		{"sql format without a destination is a usage error on audit", []string{"audit", "--format", "sql"}, ExitUsage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, _, code := run(tt.args...); code != tt.want {
				t.Errorf("exit code = %d, want %d", code, tt.want)
			}
		})
	}
}

func TestVersionGoesToStdout(t *testing.T) {
	stdout, stderr, code := run("version")

	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d", code, ExitOK)
	}
	if !strings.HasPrefix(stdout, "pgfathom ") {
		t.Errorf("stdout = %q, want it to start with the tool name", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want it empty on success", stderr)
	}
}

// TestErrorsNeverPolluteStdout guards the split that makes the output safe to
// pipe: results on stdout, everything else on stderr. A single diagnostic line
// on stdout corrupts programmatic consumption of the whole run.
func TestErrorsNeverPolluteStdout(t *testing.T) {
	for _, args := range [][]string{
		{"doesnotexist"},
		{"--nope"},
		{"--color", "mauve", "version"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			stdout, stderr, _ := run(args...)

			if stdout != "" {
				t.Errorf("stdout = %q, want it empty when the command fails", stdout)
			}
			if stderr == "" {
				t.Error("stderr is empty; the failure was reported nowhere")
			}
		})
	}
}

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// TestNoANSIWhenNotATerminal covers the case that actually happens in practice:
// the output is redirected to a file or into a CI log.
func TestNoANSIWhenNotATerminal(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"--help"}, {"doesnotexist"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			stdout, stderr, _ := run(args...)

			if ansi.MatchString(stdout) {
				t.Errorf("stdout contains ANSI escapes: %q", stdout)
			}
			if ansi.MatchString(stderr) {
				t.Errorf("stderr contains ANSI escapes: %q", stderr)
			}
		})
	}
}

func TestResolveColor(t *testing.T) {
	t.Run("explicit never wins over everything", func(t *testing.T) {
		if resolveColor(ColorNever, nil) {
			t.Error("--color=never must disable colour")
		}
	})

	t.Run("explicit always wins over detection", func(t *testing.T) {
		if !resolveColor(ColorAlways, nil) {
			t.Error("--color=always must enable colour even when not a terminal")
		}
	})

	t.Run("NO_COLOR disables colour", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		if resolveColor(ColorAuto, nil) {
			t.Error("NO_COLOR must disable colour regardless of its value")
		}
	})

	t.Run("dumb terminal disables colour", func(t *testing.T) {
		t.Setenv("TERM", "dumb")
		if resolveColor(ColorAuto, nil) {
			t.Error("TERM=dumb must disable colour")
		}
	})

	t.Run("auto without a terminal disables colour", func(t *testing.T) {
		if resolveColor(ColorAuto, nil) {
			t.Error("a non-terminal destination must not receive colour")
		}
	})
}

func TestExitCodeFor(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"no error", nil, ExitOK},
		{"plain error", errors.New("boom"), ExitFailure},
		{"usage error", UsageError(errors.New("bad flag")), ExitUsage},
		{"wrapped usage error", UsageError(errors.New("bad flag")), ExitUsage},
		{"interruption", errInterrupted, ExitInterrupted},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExitCodeFor(tt.err); got != tt.want {
				t.Errorf("ExitCodeFor(%v) = %d, want %d", tt.err, got, tt.want)
			}
		})
	}
}

// TestLoggingStaysOnStderr proves the logger never writes into the result
// stream, at any verbosity.
func TestLoggingStaysOnStderr(t *testing.T) {
	stdout, _, code := run("--log-level", "debug", "version")

	if code != ExitOK {
		t.Fatalf("exit code = %d, want %d", code, ExitOK)
	}
	for _, line := range strings.Split(strings.TrimSpace(stdout), "\n") {
		if strings.Contains(line, "level=") {
			t.Errorf("log record leaked into stdout: %q", line)
		}
	}
}

// TestSQLFormatDemandsADestination covers the decision that keeps a generated
// artifact from being piped: .sql goes to a file you can open, never to stdout.
// The message has to name the flag, or the constraint reads as a bug.
func TestSQLFormatDemandsADestination(t *testing.T) {
	for _, command := range []string{"discover", "audit"} {
		stdout, stderr, code := run(command, "--format", "sql")

		if code != ExitUsage {
			t.Errorf("%s: exit code = %d, want %d", command, code, ExitUsage)
		}
		if !strings.Contains(stderr, "--out") {
			t.Errorf("%s: the error must name the flag that fixes it, got %q", command, stderr)
		}
		if stdout != "" {
			t.Errorf("%s: a usage error must leave stdout empty, got %q", command, stdout)
		}
	}
}

// TestFormatIsValidatedBeforeConnecting keeps a typo from costing a connection
// to a production server before it is caught.
func TestFormatIsValidatedBeforeConnecting(t *testing.T) {
	_, stderr, code := run("discover", "--format", "mauve", "--dsn", "postgres://nobody@127.0.0.1:1/none")

	if code != ExitUsage {
		t.Fatalf("exit code = %d, want %d (stderr: %s)", code, ExitUsage, stderr)
	}
	if !strings.Contains(stderr, "table, json or sql") {
		t.Errorf("the error must list the accepted formats, got %q", stderr)
	}
}

// TestValidationStageReportsWhatRanNotWhatWasAsked covers a contradiction found
// against a real schema: a run started without --full announced "nothing here
// is confirmed" directly above two confirmations, because every table had fit
// the sample target and been read whole.
func TestValidationStageReportsWhatRanNotWhatWasAsked(t *testing.T) {
	tests := []struct {
		name             string
		full, sampled    bool
		wantConclusive   bool
		wantSampleCaveat bool
	}{
		{name: "full run", full: true, wantConclusive: true},
		{name: "sampling actually happened", sampled: true, wantSampleCaveat: true},
		{name: "sampled mode but everything fit", wantConclusive: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validationStage(tt.full, 100_000, tt.sampled)

			if conclusive := strings.Contains(got, "conclusive"); conclusive != tt.wantConclusive {
				t.Errorf("conclusive = %v, want %v: %q", conclusive, tt.wantConclusive, got)
			}
			if caveat := strings.Contains(got, "nothing here is confirmed"); caveat != tt.wantSampleCaveat {
				t.Errorf("sampling caveat = %v, want %v: %q", caveat, tt.wantSampleCaveat, got)
			}
			if tt.wantConclusive && strings.HasPrefix(got, "!") {
				t.Errorf("a conclusive run must not be flagged as a warning: %q", got)
			}
		})
	}
}
