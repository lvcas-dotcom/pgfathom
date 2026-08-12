// Package discovery is one execution of the discover pipeline: catalog,
// naming detection, usage evidence, candidate generation, statistical
// prefilter, validation, and the assembled result.
//
// It exists as its own unit so the sequence can be run without a command
// around it — by a test, and by the benchmark harness that has to measure the
// same path a user executes rather than an imitation of it. Each stage is
// named, which is where per-stage timing goes when the benchmark needs it.
package discovery

import (
	"context"
	"time"

	"github.com/lvcas-dotcom/pgfathom/internal/catalog"
	"github.com/lvcas-dotcom/pgfathom/internal/infer"
	"github.com/lvcas-dotcom/pgfathom/internal/model"
	"github.com/lvcas-dotcom/pgfathom/internal/profile"
	"github.com/lvcas-dotcom/pgfathom/internal/sqlprobe"
	"github.com/lvcas-dotcom/pgfathom/internal/stats"
	"github.com/lvcas-dotcom/pgfathom/internal/validate"
)

// Stage names the steps of a run, in order. They are the seams a caller can
// measure at, and the vocabulary a warning uses to say where it came from.
type Stage string

// The stages, in execution order.
const (
	StageCatalog    Stage = "catalog"
	StageDetection  Stage = "naming detection"
	StageEvidence   Stage = "usage evidence"
	StageGeneration Stage = "candidate generation"
	StagePrefilter  Stage = "statistical prefilter"
	StageValidation Stage = "validation"
)

// Options is everything a run needs that is not the connection.
type Options struct {
	Profile     *profile.Profile
	Scope       *catalog.Scope
	Exclude     []string
	MinScore    float64
	ToolVersion string

	// The negatives mirror the flags: a stage is on unless it was turned off,
	// because the person who needs a stage is usually the one who does not
	// know they need it.
	NoDetect bool
	NoStats  bool
	NoProbe  bool

	Validation validate.Options

	// Warn receives the degradation notices of the stages that are allowed to
	// fail. Reporting them to the caller rather than writing to a stream is
	// what lets a benchmark count what a command displays.
	Warn func(Stage, string)

	// Progress receives a report as each stage begins, and repeatedly during
	// validation as candidates finish. Reported rather than printed for the same
	// reason as Warn: the benchmark runs this unit to measure it, and wants to
	// count rather than to see anything drawn.
	Progress func(Progress)
}

// Progress is what is happening right now.
type Progress struct {
	Stage Stage

	// Done and Total are set only where a denominator is known before the stage
	// ends — validation, which is one query per candidate. Elsewhere they are
	// zero, because a stage that cannot know how much is left must not imply it
	// does.
	Done, Total int
}

func (o Options) progress(p Progress) {
	if o.Progress != nil {
		o.Progress(p)
	}
}

func (o Options) begin(s Stage) { o.progress(Progress{Stage: s}) }

func (o Options) warn(s Stage, msg string) {
	if o.Warn != nil {
		o.Warn(s, msg)
	}
}

// Conn is the connection surface a run needs, satisfied by *db.Pool.
type Conn interface {
	catalog.Querier
	stats.Querier
	sqlprobe.Querier
	validate.Beginner
	ServerVersion() string
}

// StageTiming is how long one stage took. Measurement comes from inside
// because a clock on the outside only sees the total, and the total does not
// say where the time went.
type StageTiming struct {
	Stage    Stage         `json:"stage"`
	Duration time.Duration `json:"duration_ns"`
}

// Result is the finished model plus what the report needs beyond it.
type Result struct {
	Result *model.Result

	// Discarded fell below the threshold or was rejected by the prefilter.
	// Kept apart from the model so a caller decides whether to publish it.
	Discarded []model.Candidate

	// Detection is what the schema revealed about its own naming convention.
	Detection model.NamingDetection

	// Stages is the cost of each stage that ran, in execution order. A stage
	// turned off is absent rather than present at zero: "did not run" and "ran
	// instantly" are different facts.
	Stages []StageTiming
}

// timeline accumulates stage costs. Each mark closes the span that started
// when the previous one closed, so a stage that was skipped contributes no
// entry and its (empty) elapsed time falls to whichever stage ran next.
type timeline struct {
	stages []StageTiming
	last   time.Time
}

func newTimeline(start time.Time) *timeline { return &timeline{last: start} }

func (t *timeline) mark(s Stage) {
	now := time.Now()
	t.stages = append(t.stages, StageTiming{Stage: s, Duration: now.Sub(t.last)})
	t.last = now
}

// Run executes the pipeline. Three stages are allowed to degrade — usage
// evidence, the statistical prefilter, and anything the caller warns about
// before us — because losing a signal is not the same as getting an answer
// wrong. Everything else aborts: a catalog that cannot be read leaves nothing
// to reason about.
func Run(ctx context.Context, conn Conn, opts Options) (*Result, error) {
	started := time.Now()
	tl := newTimeline(started)

	opts.begin(StageCatalog)
	cat, err := catalog.Read(ctx, conn, catalog.Options{Scope: opts.Scope, Exclude: opts.Exclude})
	if err != nil {
		return nil, err
	}
	tl.mark(StageCatalog)

	// Detection is on by default: whoever needs it is precisely whoever does
	// not know they need it. Measured against real schemas, a missing local
	// convention cost 78 points of recall and failed silently.
	var detection model.NamingDetection
	active := opts.Profile
	if !opts.NoDetect {
		opts.begin(StageDetection)
		detection = opts.Profile.Detect(cat.Schemas)
		active = opts.Profile.WithDetected(detection)
		tl.mark(StageDetection)
	}

	// Usage evidence is what breaks the ceiling of name matching. A failure to
	// read it costs signal, never correctness.
	var evidence sqlprobe.Evidence
	if !opts.NoProbe {
		opts.begin(StageEvidence)
		probed, err := sqlprobe.Probe(ctx, conn, cat.Schemas)
		if err != nil {
			opts.warn(StageEvidence, "usage evidence skipped: "+err.Error())
		} else {
			evidence = *probed
		}
		tl.mark(StageEvidence)
	}

	opts.begin(StageGeneration)
	inferred := infer.Generate(cat.Schemas, infer.Options{
		Profile:  active,
		MinScore: opts.MinScore,
		Evidence: evidence.Joins,
	})
	tl.mark(StageGeneration)

	coverage := cat.Coverage
	coverage.CandidatesFound = len(inferred.Candidates) + len(inferred.Discarded)
	coverage.PgStatStatements = evidence.StatementsAvailable

	// The prefilter runs on the threshold survivors only: every candidate it
	// kills is an anti-join that validation will not fire against someone's
	// production server. A failure to read statistics degrades to a warning,
	// never to an opinion about candidates.
	if !opts.NoStats {
		opts.begin(StagePrefilter)
		pre, err := stats.Prefilter(ctx, conn, cat.Schemas, inferred.Candidates, stats.Options{})
		if err != nil {
			opts.warn(StagePrefilter, "statistical prefilter skipped: "+err.Error())
		} else {
			inferred.Candidates = pre.Kept
			inferred.Discarded = append(inferred.Discarded, pre.Rejected...)
			coverage.StatsPrefilter = true
			coverage.CandidatesStatsChecked = pre.Checked
			coverage.CandidatesStatsRejected = len(pre.Rejected)
			coverage.CandidatesWithoutStats = pre.NoStats
		}
		tl.mark(StagePrefilter)
	}

	opts.begin(StageValidation)

	// The total is known before the stage starts: one query per candidate. This
	// is the only stage with an honest denominator.
	total := len(inferred.Candidates)
	validation := opts.Validation
	if opts.Progress != nil && total > 0 {
		validation.OnValidated = func(done int) {
			opts.progress(Progress{Stage: StageValidation, Done: done, Total: total})
		}
	}

	validated, err := validate.Run(ctx, conn, cat.Schemas, inferred.Candidates, validation)
	if err != nil {
		return nil, err
	}
	tl.mark(StageValidation)

	inferred.Candidates = validated.Candidates
	coverage.CandidatesValidated = validated.Validated
	coverage.CandidatesTimedOut = validated.TimedOut

	result := model.NewResult(opts.ToolVersion, opts.Profile.Name, time.Now().UTC(), coverage)
	result.ServerVersion = conn.ServerVersion()
	result.Naming = detection
	result.Schemas = cat.Schemas
	result.Candidates = inferred.Candidates
	result.Findings = observations(inferred)
	result.Duration = time.Since(started)

	return &Result{
		Result:    result,
		Discarded: inferred.Discarded,
		Detection: detection,
		Stages:    tl.stages,
	}, nil
}

// observations turns what inference recognized but could not use into
// findings, so a deliberate limitation reads as an observation rather than as
// silence.
func observations(res *infer.Result) []model.Finding {
	out := make([]model.Finding, 0, len(res.Polymorphic)+len(res.Skipped))

	for _, p := range res.Polymorphic {
		out = append(out, model.Finding{
			Kind:   model.FindingPolymorphicPair,
			Object: p.Table + "." + p.ReferenceColumn,
			Detail: "only meaningful beside " + p.TypeColumn + "; polymorphic references are out of scope",
		})
	}

	for _, s := range res.Skipped {
		detail := string(s.Reason) + " (" + s.Target + ")"
		if s.Detail != "" {
			detail += ": " + s.Detail
		}
		out = append(out, model.Finding{
			Kind:   model.FindingUnsupportedTarget,
			Object: s.Child.String(),
			Detail: detail,
		})
	}

	return out
}
