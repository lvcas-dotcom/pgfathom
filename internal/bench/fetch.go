//go:build benchmark

package bench

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// fetchTimeout bounds a single download. The GitLab schema is under three
// megabytes; anything slower than this is a network that is not going to
// finish.
const fetchTimeout = 5 * time.Minute

// Acquire downloads a schema into the cache and verifies it, returning whether
// it had to go to the network.
//
// Acquisition is separate from measurement on purpose. Network inside the
// measurement would contaminate two things at once: the time reported would
// include whatever the carrier did that afternoon, and an outage would read as
// a benchmark failure.
func Acquire(ctx context.Context, e Entry) (downloaded bool, err error) {
	if e.Kind == FromLocal {
		if _, err := os.Stat(e.CachePath()); err != nil {
			return false, fmt.Errorf("local schema %s is not on this machine: %w", e.Name, err)
		}
		return false, nil
	}

	path := e.CachePath()
	if sum, err := checksum(path); err == nil {
		if sum == e.SHA256 {
			return false, nil
		}
		return false, mismatch(e, sum)
	}

	if err := download(ctx, e, path); err != nil {
		return false, err
	}

	sum, err := checksum(path)
	if err != nil {
		return true, err
	}
	if sum != e.SHA256 {
		// The bad file does not stay behind pretending to be the corpus.
		_ = os.Remove(path)
		return true, mismatch(e, sum)
	}
	return true, nil
}

// mismatch is a hard stop, never a warning. A corpus that is not the corpus
// produces a number about something else.
func mismatch(e Entry, got string) error {
	return fmt.Errorf(
		"%s: checksum mismatch\n  expected %s\n  got      %s\nthe upstream file changed under a pinned commit, or the download is corrupt; nothing was measured",
		e.Name, e.SHA256, got)
}

func download(ctx context.Context, e Entry, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, e.URL, nil)
	if err != nil {
		return fmt.Errorf("%s: building request: %w", e.Name, err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s: fetching %s: %w", e.Name, e.URL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: fetching %s: %s", e.Name, e.URL, resp.Status)
	}

	// Written beside the target and renamed, so an interrupted download never
	// leaves a truncated file that the next run would checksum and reject
	// without knowing why.
	tmp, err := os.CreateTemp(filepath.Dir(path), ".partial-*")
	if err != nil {
		return fmt.Errorf("%s: creating temporary file: %w", e.Name, err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("%s: writing download: %w", e.Name, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("%s: closing download: %w", e.Name, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("%s: moving download into place: %w", e.Name, err)
	}
	return nil
}

func checksum(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Ready reports whether a schema is cached and correct, which is what the
// measurement checks before deciding it cannot run.
func Ready(e Entry) bool {
	sum, err := checksum(e.CachePath())
	if err != nil {
		return false
	}
	return e.Kind == FromLocal || sum == e.SHA256
}
