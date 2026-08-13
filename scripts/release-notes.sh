#!/usr/bin/env bash
#
# Extracts one version's section from the CHANGELOG, for goreleaser to publish
# as the release notes.
#
# It exists because notes generated from commits describe what was touched, not
# what changed for whoever uses this — and because publishing is irreversible.
# If the section is missing this fails and stops the whole release: better to
# abort before publishing than to go out with empty notes that cannot be fixed.
#
# Usage: scripts/release-notes.sh v0.1.1 [changelog-path]
set -euo pipefail

tag="${1:?usage: release-notes.sh <tag> [changelog]}"
changelog="${2:-CHANGELOG.md}"
version="${tag#v}"

if [ ! -f "$changelog" ]; then
	echo "release-notes: $changelog does not exist" >&2
	exit 1
fi

# The section runs from the version heading to the next heading of the same level.
notes=$(awk -v version="$version" '
	$0 ~ "^## \\[" version "\\]" { collecting = 1; next }
	collecting && /^## / { exit }
	# Link definitions sit at the foot of the file, after the last section, with
	# no heading to separate them. Without this the oldest version inherits them.
	collecting && /^\[[^]]+\]: / { next }
	collecting { print }
' "$changelog")

# Trimmed at both ends, so the notes do not open with a gap.
notes=$(printf '%s\n' "$notes" | sed -e '/./,$!d' -e :a -e '/^\n*$/{$d;N;ba' -e '}')

if [ -z "$notes" ]; then
	echo "release-notes: $changelog has no section for $version" >&2
	echo "  add '## [$version] - <date>' before tagging" >&2
	exit 1
fi

printf '%s\n' "$notes"
