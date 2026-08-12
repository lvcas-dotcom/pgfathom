//go:build benchmark

// Package bench measures recovery rate against a corpus of real schemas.
//
// It exists behind a build tag because it is not part of what ships: it needs
// Docker, it writes to the databases it measures, and nothing in a released
// binary should be able to reach it. Everything here runs `go test
// -tags=benchmark`, which is a small impropriety of vocabulary bought in
// exchange for container lifecycle, output and filtering that already work.
//
// The measurement calls the discovery pipeline in process. A number published
// about this tool only means something if it measured the path a user runs.
package bench

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Acquisition says where a schema comes from. The field exists with a single
// value in use so that a schema requiring an application boot — Mastodon,
// Redmine and Odoo publish their schema only as a DSL or through the ORM —
// arrives later as a new line rather than as a rewrite.
type Acquisition string

const (
	// FromSQL is a plain SQL file downloaded from a pinned commit.
	FromSQL Acquisition = "sql"

	// FromLocal is a dump that lives on this machine only, typically because
	// it is real data that cannot be published. Absent is not an error.
	FromLocal Acquisition = "local"
)

// Entry is one corpus entry. It is not the same thing as the PostgreSQL schema
// it measures — that one is the Schema field — and calling both "schema" would
// force every reader to guess which is meant.
type Entry struct {
	Name string      `toml:"name"`
	Kind Acquisition `toml:"kind"`

	URL    string `toml:"url"`
	Commit string `toml:"commit"`
	SHA256 string `toml:"sha256"`

	// Path is where a local entry lives, relative to the repository root.
	Path string `toml:"path"`

	// Postgres is the server image. The corpus runs above the floor the
	// integration suite pins: GitLab's schema uses syntax older servers reject,
	// and a benchmark that cannot load its corpus measures nothing.
	Postgres string `toml:"postgres"`

	// Profile is the shipped naming profile this schema runs with, published
	// alongside its numbers. Naming it is a documented flag a user of that
	// schema would pass; inventing an affix so the corpus matches would be
	// tuning, and would not fit in a column of the published table.
	Profile string `toml:"profile"`

	// Schema is the PostgreSQL schema measured inside the loaded database. It
	// defaults to public, which is the tool's own default and therefore the run
	// a user makes. A dump of one schema of a multi-schema database — the shape
	// of every municipal system measured so far — names it here.
	Schema string `toml:"schema"`

	Note string `toml:"note"`
}

// DefaultSchema is what an entry measures when it says nothing: the tool's own
// default, so the number describes the run a user makes.
const DefaultSchema = "public"

// Manifest is the versioned corpus recipe.
type Manifest struct {
	Schemas []Entry `toml:"schema"`
}

// LoadManifest reads bench/corpus.toml and checks that every entry carries
// what its acquisition kind requires.
func LoadManifest() (*Manifest, error) {
	path := filepath.Join(repoRoot(), "bench", "corpus.toml")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading corpus manifest: %w", err)
	}

	var m Manifest
	if err := toml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if len(m.Schemas) == 0 {
		return nil, fmt.Errorf("%s declares no schema", path)
	}

	for i := range m.Schemas {
		if m.Schemas[i].Schema == "" {
			m.Schemas[i].Schema = DefaultSchema
		}
		if err := m.Schemas[i].validate(); err != nil {
			return nil, fmt.Errorf("%s: entry %d: %w", path, i+1, err)
		}
	}
	return &m, nil
}

func (e Entry) validate() error {
	switch {
	case e.Name == "":
		return fmt.Errorf("name is required")
	case e.Postgres == "":
		return fmt.Errorf("%s: postgres image is required", e.Name)
	case e.Profile == "":
		return fmt.Errorf("%s: profile is required, and is published with the numbers", e.Name)
	}

	switch e.Kind {
	case FromSQL:
		if e.URL == "" || e.Commit == "" || e.SHA256 == "" {
			return fmt.Errorf("%s: a downloaded schema needs url, commit and sha256", e.Name)
		}
		if !strings.Contains(e.URL, e.Commit) {
			return fmt.Errorf("%s: the url must pin the declared commit, or the checksum guards a moving target", e.Name)
		}
	case FromLocal:
		if e.Path == "" {
			return fmt.Errorf("%s: a local schema needs a path", e.Name)
		}
	default:
		return fmt.Errorf("%s: unknown acquisition kind %q", e.Name, e.Kind)
	}
	return nil
}

// CachePath is where a schema's SQL lives once acquired.
func (e Entry) CachePath() string {
	if e.Kind == FromLocal {
		return filepath.Join(repoRoot(), e.Path)
	}
	return filepath.Join(repoRoot(), "bench", "cache", e.Name+".sql")
}

// repoRoot locates the repository from this file's own path, so the harness
// works whatever directory it is invoked from.
func repoRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}
