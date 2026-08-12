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
