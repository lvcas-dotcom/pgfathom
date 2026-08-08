package sqlprobe

import (
	"testing"

	"github.com/lvcas-dotcom/pgfathom/internal/testutil"
)

func TestQueriesTouchOnlyTheCatalog(t *testing.T) {
	allowed := map[string]bool{
		"pg_views": true, "pg_proc": true, "pg_namespace": true, "pg_language": true,
		"pg_extension": true, "pg_stat_statements": true,
	}

	testutil.AssertCatalogOnly(t, "probe.go", allowed,
		"mining SQL is pure catalog work, and the day it stops being that has to fail loudly")
}
