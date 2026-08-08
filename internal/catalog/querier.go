package catalog

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Querier is the slice of the connection pool this layer needs. Declaring it
// here rather than taking the pool keeps the dependency pointing at what is
// used, and lets the error paths — a restricted role, a malformed row — be
// exercised without a server.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}
