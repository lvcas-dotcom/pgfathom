## What changes, and why

<!-- The why is the part that is hard to recover later. What was the problem? -->

## How it was verified

<!--
Not "tests pass" — which tests, and what would have caught it failing.
`make lint` runs under all three build tags; code behind a tag is invisible
to the default build.
-->

- [ ] `make fmt && make lint && make test`
- [ ] `make test-integration` (if it touches SQL, the catalog, or validation)

## Checks

- [ ] Nothing writes to the target database
- [ ] No row value can reach output, logs or errors
- [ ] New behaviour has a change under `openspec/`, validating with `--strict`
- [ ] A new dependency, if any, is justified with measured numbers
