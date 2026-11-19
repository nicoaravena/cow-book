//go:build integration

package postgres

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"nicoaravena/cow-book-app/internal/postgres/tenantdb"
)

// onePool opens a pool of one connection with newPool, so every call reuses
// the same connection. params is appended to DATABASE_URL, which already has
// a ? part. NewPool and pgxpool.New both fit newPool's type.
func onePool(t *testing.T, newPool func(context.Context, string) (*pgxpool.Pool, error), params string) *pgxpool.Pool {
	t.Helper()
	pool, err := newPool(t.Context(), os.Getenv("DATABASE_URL")+"&pool_max_conns=1"+params)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// createLocation adds a location named name in the tenant and returns its id.
func createLocation(t *testing.T, pool *pgxpool.Pool, tenantID int64, name string) int64 {
	t.Helper()
	var id int64
	err := WithTenant(t.Context(), pool, tenantID, func(q *tenantdb.Queries) error {
		var err error
		id, err = q.CreateLocation(t.Context(), tenantdb.CreateLocationParams{Name: name, Timezone: "Europe/Madrid"})
		return err
	})
	if err != nil {
		t.Fatalf("create location: %v", err)
	}
	return id
}

func TestWithTenantSeesOnlyItsSchema(t *testing.T) {
	pool := testPool(t)
	a := createTestTenant(t, pool)
	b := createTestTenant(t, pool)
	ctx := t.Context()

	id := createLocation(t, pool, a, "Centre")

	var got tenantdb.GetLocationRow
	err := WithTenant(ctx, pool, a, func(q *tenantdb.Queries) error {
		var err error
		got, err = q.GetLocation(ctx, id)
		return err
	})
	if err != nil {
		t.Fatalf("tenant a: %v", err)
	}
	if got.Name != "Centre" || got.Timezone != "Europe/Madrid" {
		t.Errorf("tenant a: got %+v, want Centre in Europe/Madrid", got)
	}

	// The same id in tenant b reads b's empty locations table.
	err = WithTenant(ctx, pool, b, func(q *tenantdb.Queries) error {
		_, err := q.GetLocation(ctx, id)
		return err
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("tenant b: err = %v, want pgx.ErrNoRows", err)
	}
}

// callRecovering calls f and reports whether it panicked, so a test can go on
// after a panic. Deferred functions run while a panic unwinds, and recover
// inside one stops it.
func callRecovering(f func() error) (panicked bool, err error) {
	defer func() {
		if recover() != nil {
			panicked = true
		}
	}()
	return false, f()
}

func TestWithTenantSearchPathDoesNotCarryOver(t *testing.T) {
	setup := testPool(t)
	id := createTestTenant(t, setup)
	pool := onePool(t, NewPool, "")
	ctx := t.Context()

	// What the one connection looks like outside any tenant transaction.
	var pid int32
	var path string
	if err := pool.QueryRow(ctx, "SELECT pg_backend_pid(), current_setting('search_path')").Scan(&pid, &path); err != nil {
		t.Fatalf("read connection: %v", err)
	}

	errRefused := errors.New("refused")
	tests := []struct {
		name      string
		fn        func(q *tenantdb.Queries) error
		wantErr   error // nil means WithTenant must succeed
		wantPanic bool
		wantSaved bool // whether the location fn added is still there
	}{
		{"commit", func(q *tenantdb.Queries) error {
			_, err := q.CreateLocation(ctx, tenantdb.CreateLocationParams{Name: "commit", Timezone: "UTC"})
			return err
		}, nil, false, true},
		{"error rolls back", func(q *tenantdb.Queries) error {
			if _, err := q.CreateLocation(ctx, tenantdb.CreateLocationParams{Name: "error rolls back", Timezone: "UTC"}); err != nil {
				return err
			}
			return errRefused
		}, errRefused, false, false},
		{"panic rolls back", func(q *tenantdb.Queries) error {
			if _, err := q.CreateLocation(ctx, tenantdb.CreateLocationParams{Name: "panic rolls back", Timezone: "UTC"}); err != nil {
				return err
			}
			panic("boom")
		}, nil, true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			panicked, err := callRecovering(func() error { return WithTenant(ctx, pool, id, tt.fn) })
			if panicked != tt.wantPanic {
				t.Fatalf("panicked = %v, want %v", panicked, tt.wantPanic)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}

			var gotPID int32
			var gotPath string
			if err := pool.QueryRow(ctx, "SELECT pg_backend_pid(), current_setting('search_path')").Scan(&gotPID, &gotPath); err != nil {
				t.Fatalf("read connection: %v", err)
			}
			if gotPID != pid {
				t.Fatalf("backend pid = %d, want %d: the test needs the same connection", gotPID, pid)
			}
			if gotPath != path {
				t.Errorf("search_path = %q after WithTenant, want %q", gotPath, path)
			}

			// Without a tenant transaction, tenant tables aren't visible.
			_, err = pool.Exec(ctx, "SELECT 1 FROM locations")
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) || pgErr.Code != "42P01" {
				t.Errorf("unqualified locations outside WithTenant: err = %v, want undefined_table (42P01)", err)
			}

			var saved bool
			q := "SELECT EXISTS (SELECT 1 FROM " + pgx.Identifier{SchemaName(id), "locations"}.Sanitize() + " WHERE name = $1)"
			if err := setup.QueryRow(ctx, q, tt.name).Scan(&saved); err != nil {
				t.Fatalf("look up location: %v", err)
			}
			if saved != tt.wantSaved {
				t.Errorf("location saved = %v, want %v", saved, tt.wantSaved)
			}
		})
	}
}

// TestQueryModeAcrossSchemaVersions shows why NewPool doesn't use pgx's
// default query mode. Tenant b's locations.name is varchar instead of text,
// as if b were on another migration version.
func TestQueryModeAcrossSchemaVersions(t *testing.T) {
	setup := testPool(t)
	a := createTestTenant(t, setup)
	b := createTestTenant(t, setup)
	alter := "ALTER TABLE " + pgx.Identifier{SchemaName(b), "locations"}.Sanitize() +
		" ALTER COLUMN name TYPE varchar(200)"
	if _, err := setup.Exec(t.Context(), alter); err != nil {
		t.Fatalf("alter tenant b: %v", err)
	}
	ids := map[int64]int64{a: createLocation(t, setup, a, "A"), b: createLocation(t, setup, b, "B")}

	// get runs the same GetLocation SQL in a, then in b, on pool's one connection.
	get := func(t *testing.T, pool *pgxpool.Pool) (errA, errB error) {
		t.Helper()
		errs := map[int64]error{}
		for _, tenant := range []int64{a, b} {
			errs[tenant] = WithTenant(t.Context(), pool, tenant, func(q *tenantdb.Queries) error {
				_, err := q.GetLocation(t.Context(), ids[tenant])
				return err
			})
		}
		return errs[a], errs[b]
	}

	t.Run("cache_statement", func(t *testing.T) {
		pool := onePool(t, pgxpool.New, "&default_query_exec_mode=cache_statement")
		errA, errB := get(t, pool)
		if errA != nil {
			t.Fatalf("tenant a: %v", errA)
		}
		var pgErr *pgconn.PgError
		if !errors.As(errB, &pgErr) || pgErr.Code != "0A000" {
			t.Fatalf("tenant b: err = %v, want feature_not_supported (0A000), cached plan must not change result type", errB)
		}
	})

	t.Run("describe_exec", func(t *testing.T) {
		pool := onePool(t, NewPool, "")
		if errA, errB := get(t, pool); errA != nil || errB != nil {
			t.Fatalf("tenant a: %v, tenant b: %v", errA, errB)
		}
	})
}
