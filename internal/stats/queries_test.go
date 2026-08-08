package stats

import (
	"testing"

	"github.com/lvcas-dotcom/pgfathom/internal/testutil"
)

func TestQueriesTouchOnlyStatisticsViews(t *testing.T) {
	allowed := map[string]bool{
		"pg_stats": true,
		"unnest":   true,
	}

	testutil.AssertCatalogOnly(t, "read.go", allowed,
		"the prefilter must touch only planner statistics, because reading data belongs to validation")
}
