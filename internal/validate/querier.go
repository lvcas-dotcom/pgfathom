package validate

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Beginner is the slice of the connection pool this layer needs. Validation is
// the one layer that requires a transaction rather than a bare query: the
// per-candidate time ceiling is applied with SET LOCAL, which has to die with
// the transaction that set it.
type Beginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}
