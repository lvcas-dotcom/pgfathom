# Data model

← [Guide](../README.md) › [Architecture](README.md)

Sketch of the central structures in `internal/model` (pure types, no I/O — see
[Layers](layers.md)). Keeps **read**, **evidenced**, and **inferred** apart at the type
level.

```go
type Table struct {
    Schema      string
    Name        string
    Columns     []Column
    PrimaryKey  []string
    ForeignKeys []ForeignKey    // declared only
    Stats       TableStats
}

type ForeignKey struct {
    Name       string
    Columns    []string
    RefTable   string
    RefColumns []string
    Validated  bool             // pg_constraint.convalidated — false = NOT VALID
    HasIndex   bool
}
```

`Validated` is the field that reveals `audit`'s most treacherous finding — see
[The problem](../01-overview/the-problem.md).

## Candidates and signals

```go
type Signal struct {
    Kind   SignalKind
    Weight float64           // can be negative
    Detail string            // object name only, NEVER a data value
}

type Candidate struct {
    Child      ColumnRef
    Parent     ColumnRef
    Signals    []Signal
    MetaScore  float64        // 0..1, before touching data
    Validation *Validation    // nil if not validated
    Verdict    Verdict
}
```

Every score must be traceable to the signals that produced it — no scoring path is
allowed to bypass `Signals`. (`openspec/specs/candidate-scoring/spec.md`)

## Validation and verdict

```go
type Validation struct {
    Method              string        // "full" or "sampled"
    NotNullRows         int64
    DistinctVals        int64
    OrphanRows          int64
    OrphanVals          int64
    ContainmentRows     float64       // (NotNullRows - OrphanRows) / NotNullRows
    ContainmentVals     float64       // (DistinctVals - OrphanVals) / DistinctVals
}

type Verdict string

const (
    VerdictConfirmed   Verdict = "confirmed"
    VerdictBroken      Verdict = "broken"
    VerdictWeak        Verdict = "weak"
    VerdictRejected    Verdict = "rejected"
    VerdictUnvalidated Verdict = "unvalidated"
)
```

## Coverage — mandatory on every output

```go
type Coverage struct {
    TablesTotal         int
    TablesAnalyzed      int
    TablesNoPrivilege   []string
    CandidatesTimedOut  int
    StatsResetAt        *time.Time
}
```

`Coverage` is the direct materialization of "silence is never reported as a clean bill of
health" — see [Safety guarantees](../03-features-and-safety/safety-guarantees.md).

## Full detail

The complete struct set — `Column`, `ColumnStats` (and why two of its fields are
deliberately unexported), every `SignalKind`, `Finding` — is in
[`docs/PGFATHOM.md` § Modelo interno](../../PGFATHOM.md#modelo-interno) (Portuguese).
