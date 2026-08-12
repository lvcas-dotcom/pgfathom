# Security policy

pgfathom is pointed at production databases owned by someone who is trusting the
person running it. That shapes what counts as a vulnerability here.

## Reporting a vulnerability

Report privately through
[GitHub's advisory form](https://github.com/lvcas-dotcom/pgfathom/security/advisories/new).
It reaches the maintainers without becoming public first.

Please do not open a public issue for anything in the list below.

You should get a first response within **72 hours**, and an assessment — whether
it is a vulnerability, and what the fix looks like — within **7 days**. If you
have not heard back in a week, assume the message was lost rather than ignored
and open a public issue saying only that you are waiting on a private report.

A fix ships in a patch release, credited to you unless you ask otherwise, with
an advisory naming the affected versions.

## What counts as a vulnerability

The tool makes three promises that its whole design serves. **Any way to break
one of them is a vulnerability, even without a classical exploit:**

- **Read-only, absolutely.** Any path that gets a write to the target database —
  DDL, DML, a session setting that outlives the connection, a lock that blocks
  another transaction beyond the statement.
- **Your data never leaves.** Any path that puts a value from a row into a
  report, a log line, an error message, a JSON document or a generated `.sql`
  file. Counts, proportions and object names are what the tool is allowed to
  emit; a primary key that leaked into an error string is a bug of this class.
- **Nothing is executed for you.** The generated SQL is for a human to read and
  run. Any path that runs it, or that composes SQL a reviewer would read as
  doing one thing while it does another, belongs here.

Also in scope, in the ordinary sense:

- SQL injection through a schema, table or column name — object names come from
  the target database and are not trusted input.
- Credentials reaching a place they should not: a log, a process argument, a
  crash dump, a generated artifact.
- A connection silently downgrading to plaintext when TLS was asked for.
- A published artifact that does not match the source it claims — the release,
  the container image, the packages.

## What does not count

- Findings that need an already-compromised machine, or a hostile
  `PGFATHOM_DSN`. Whoever controls those already controls the run.
- The tool being slow, or heavy, on a database somebody pointed it at without
  reading `--help`. Cost control is a feature request; open an issue.
- A false positive in a discovered relationship. That is a correctness bug and
  belongs in a public issue, with the evidence block the report printed.

## Supported versions

While the major version is `0`, only the latest release gets fixes. There is no
backporting to earlier `0.x` versions.

| Version | Supported |
| --- | --- |
| 0.1.x (latest) | yes |
| anything older | no |
