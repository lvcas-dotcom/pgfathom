package cli

import (
	"strings"
	"testing"
)

// TestPlanComposesWhatWasAnswered is the guide's central promise: the command
// it prints is the command those answers mean. If this drifts, the guide starts
// teaching a line that does something else.
func TestPlanComposesWhatWasAnswered(t *testing.T) {
	tests := []struct {
		name string
		plan plan
		want string
	}{
		{
			"connection from the environment stays out of the command",
			plan{dsnFromEnv: true, dsn: "postgres://u@h/db", schemas: []string{"public"}},
			"pgfathom discover --schema public",
		},
		{
			"schemas, full mode and artifacts",
			plan{dsnFromEnv: true, schemas: []string{"geral", "gestaoobras"}, full: true, out: "/tmp/out"},
			"pgfathom discover --schema geral,gestaoobras --full --out /tmp/out",
		},
		{
			"a composed connection is carried explicitly",
			plan{dsn: "postgres://u@h:5432/db", schemas: []string{"public"}},
			"pgfathom discover --dsn postgres://u@h:5432/db --schema public",
		},
		{
			"a path with a space survives copying",
			plan{dsnFromEnv: true, schemas: []string{"public"}, out: "/tmp/my out"},
			"pgfathom discover --schema public --out '/tmp/my out'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.plan.Command(); got != tt.want {
				t.Errorf("command =\n  %s\nwant\n  %s", got, tt.want)
			}
		})
	}
}

// TestPrintedCommandIsTheExecutedOne closes the gap the guide would otherwise
// open: showing one thing and running another is worse than not showing at all.
func TestPrintedCommandIsTheExecutedOne(t *testing.T) {
	p := plan{schemas: []string{"geral"}, full: true, out: "/tmp/out", dsn: "postgres://u@h/db"}

	printed := p.Command()
	for _, arg := range p.Args() {
		if !strings.Contains(printed, arg) {
			t.Errorf("argument %q is executed but does not appear in the printed command %q", arg, printed)
		}
	}
	if n := strings.Count(printed, "--"); n != len(flagsIn(p.Args())) {
		t.Errorf("printed command has %d flags, the executed one has %d", n, len(flagsIn(p.Args())))
	}
}

func flagsIn(args []string) []string {
	var out []string
	for _, a := range args {
		if strings.HasPrefix(a, "--") {
			out = append(out, a)
		}
	}
	return out
}

// TestRedactHidesThePassword covers the one place a credential can reach the
// screen: an environment variable somebody put a password into.
func TestRedactHidesThePassword(t *testing.T) {
	tests := map[string]string{
		"postgres://user:hunter2@host:5432/db": "postgres://user:•••@host:5432/db",
		"postgres://user@host:5432/db":         "postgres://user@host:5432/db",
		"host=localhost dbname=x":              "host=localhost dbname=x",
	}

	for in, want := range tests {
		if got := redact(in); got != want {
			t.Errorf("redact(%q) = %q, want %q", in, got, want)
		}
		if strings.Contains(redact(in), "hunter2") {
			t.Errorf("redact(%q) leaked the password", in)
		}
	}
}
