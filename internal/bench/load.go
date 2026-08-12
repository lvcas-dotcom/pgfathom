//go:build benchmark

package bench

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Load applies a dump with psql, tolerating statements that fail, and reports
// how many failed.
//
// A dump taken from a real database almost never applies cleanly somewhere
// else. This one names roles that do not exist here, reaches into six other
// schemas that were not dumped, and wants PostGIS. Chasing that closure ends at
// "dump the whole database"; refusing the dump ends at "no real schema is ever
// measured". So the load tolerates failures and counts them, and the count is
// published beside the numbers: a schema that half-applied would otherwise
// produce a recall about a schema nobody has.
//
// psql does the applying rather than this package, because splitting SQL
// correctly means handling dollar quoting, COPY blocks and backslash commands —
// work that psql already does and that a hand-rolled splitter would do worse.
// It is a requirement of the benchmark path only, never of the tool.
func Load(ctx context.Context, dsn, path string) (failures int, err error) {
	if _, err := exec.LookPath("psql"); err != nil {
		return 0, fmt.Errorf("the benchmark needs psql to apply a local dump: %w", err)
	}

	cmd := exec.CommandContext(ctx, "psql",
		"--no-psqlrc",
		"--quiet",
		"--set", "ON_ERROR_STOP=0",
		"--file", path,
		dsn,
	)

	out, err := cmd.CombinedOutput()
	failures = countErrors(string(out))

	// A non-zero exit with ON_ERROR_STOP off means psql itself could not run —
	// a missing file, a refused connection — not a statement that failed.
	if err != nil && failures == 0 {
		return 0, fmt.Errorf("applying %s: %w: %s", path, err, firstLine(string(out)))
	}
	return failures, nil
}

func countErrors(output string) int {
	n := 0
	sc := bufio.NewScanner(strings.NewReader(output))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		if strings.Contains(sc.Text(), "ERROR:") {
			n++
		}
	}
	return n
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
