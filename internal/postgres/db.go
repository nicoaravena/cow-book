package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nicoaravena/cow-book-app/internal/postgres/tenantdb"
)

// NewPool connects to url and returns a pool of connections, which the pool
// hands out and takes back, so one connection serves many tenants in turn.
//
// It sets pgx's query mode to describe_exec. pgx's default, cache_statement,
// prepares each SQL text once per connection and reuses it. The same text in
// another tenant's schema is planned again by Postgres, and if that schema is
// on another migration version with different result columns, the query
// fails with "cached plan must not change result type". Tenants are on
// different versions while a migration rolls out (5.5). describe_exec asks
// Postgres for the types on every query, which costs one more round trip.
// exec would save it, but guesses parameter types from the Go values instead.
// TestQueryModeAcrossSchemaVersions shows the failure.
func NewPool(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeDescribeExec
	return pgxpool.NewWithConfig(ctx, cfg)
}

// WithTenant runs fn in a transaction in tenant tenantID's schema and commits
// if fn returns nil. If fn returns an error or panics, the transaction is
// rolled back; the error comes back wrapped, the panic carries on up.
//
// The search_path is set with is_local, so it ends with the transaction and
// the connection goes back to the pool as it came. Outside WithTenant no
// tenant table is visible, and a tenant without a schema sees none either:
// a mistake fails closed. 5.1 adds SET LOCAL ROLE here.
func WithTenant(ctx context.Context, pool *pgxpool.Pool, tenantID int64, fn func(q *tenantdb.Queries) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	// Runs however this function ends, a panic in fn included. After Commit,
	// Rollback does nothing and returns pgx.ErrTxClosed, so its error is
	// ignored. WithoutCancel lets it run when ctx is what failed.
	defer tx.Rollback(context.WithoutCancel(ctx))

	// set_config takes the path as a parameter, so no SQL is built from it.
	path := pgx.Identifier{SchemaName(tenantID)}.Sanitize() + ", shared"
	if _, err := tx.Exec(ctx, "SELECT set_config('search_path', $1, true)", path); err != nil {
		return fmt.Errorf("set search_path: %w", err)
	}

	if err := fn(tenantdb.New(tx)); err != nil {
		return fmt.Errorf("tenant %d: %w", tenantID, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}
