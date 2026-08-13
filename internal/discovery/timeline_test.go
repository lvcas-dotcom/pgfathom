package discovery

import (
	"testing"
	"time"
)

// TestTimelinePartitionsTheRun covers the property the cost report rests on:
// the stages are consecutive spans of one run, so they come out in execution
// order and add up to the elapsed time rather than overlapping it.
func TestTimelinePartitionsTheRun(t *testing.T) {
	start := time.Now()
	tl := newTimeline(start)

	for _, stage := range []Stage{StageCatalog, StageGeneration, StageValidation} {
		time.Sleep(time.Millisecond)
		tl.mark(stage)
	}
	elapsed := time.Since(start)

	want := []Stage{StageCatalog, StageGeneration, StageValidation}
	if len(tl.stages) != len(want) {
		t.Fatalf("got %d stages, want %d", len(tl.stages), len(want))
	}

	var sum time.Duration
	for i, s := range tl.stages {
		if s.Stage != want[i] {
			t.Errorf("stage %d is %q, want %q: the order must be the order they ran", i, s.Stage, want[i])
		}
		if s.Duration <= 0 {
			t.Errorf("stage %q took %s; a span that ran has a duration", s.Stage, s.Duration)
		}
		sum += s.Duration
	}

	if sum > elapsed {
		t.Errorf("stages sum to %s over an elapsed %s: consecutive spans cannot overlap", sum, elapsed)
	}
}

// TestSkippedStageIsAbsentNotZero pins the distinction the report depends on.
// A stage present at zero would read as "ran instantly", and a reader would
// conclude the tool does that work for free.
func TestSkippedStageIsAbsentNotZero(t *testing.T) {
	tl := newTimeline(time.Now())
	tl.mark(StageCatalog)
	tl.mark(StageGeneration)

	for _, s := range tl.stages {
		if s.Stage == StageDetection || s.Stage == StageEvidence {
			t.Errorf("stage %q never ran and must not appear", s.Stage)
		}
	}
}

// TestProgressCoversTheStagesInOrder pins what the drawn line depends on: every
// stage that runs announces itself, and validation is the only one that claims
// a denominator — the others cannot know one before they finish, and a stage
// that implies it does is lying with two decimal places.
func TestProgressCoversTheStagesInOrder(t *testing.T) {
	var seen []Progress
	opts := Options{Progress: func(p Progress) { seen = append(seen, p) }}

	// The stages are announced by Run, so this exercises the reporting contract
	// directly rather than the pipeline: no database is involved.
	for _, s := range []Stage{StageCatalog, StageDetection, StageEvidence, StageGeneration, StagePrefilter} {
		opts.begin(s)
	}
	opts.begin(StageValidation)
	opts.progress(Progress{Stage: StageValidation, Done: 1, Total: 4})

	if len(seen) != 7 {
		t.Fatalf("got %d reports, want 7", len(seen))
	}
	for i, want := range []Stage{
		StageCatalog, StageDetection, StageEvidence,
		StageGeneration, StagePrefilter, StageValidation, StageValidation,
	} {
		if seen[i].Stage != want {
			t.Errorf("report %d is %q, want %q", i, seen[i].Stage, want)
		}
	}
	for _, p := range seen[:6] {
		if p.Total != 0 {
			t.Errorf("stage %q claims a denominator of %d before it can know one", p.Stage, p.Total)
		}
	}
	if seen[6].Done != 1 || seen[6].Total != 4 {
		t.Errorf("validation reported %d/%d, want 1/4", seen[6].Done, seen[6].Total)
	}
}

// TestNoProgressFunctionWritesNothing is the property the benchmark depends on:
// it runs the same unit to measure it, and wants nothing drawn.
func TestNoProgressFunctionWritesNothing(t *testing.T) {
	opts := Options{}
	opts.begin(StageCatalog)
	opts.progress(Progress{Stage: StageValidation, Done: 1, Total: 2})
}
