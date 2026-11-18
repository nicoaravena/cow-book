//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"nicoaravena/cow-book-app/migrations"
)

// testPool connects to DATABASE_URL, the compose database with the platform
// migration applied (make db-up migrate-platform).
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Fatal("DATABASE_URL is not set; integration tests need the compose database")
	}
	pool, err := pgxpool.New(t.Context(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// testSlug returns a slug no other test run uses, so runs don't collide and
// leftovers are easy to spot.
func testSlug() string {
	return fmt.Sprintf("test-%d", rand.Int64N(1_000_000_000))
}

// createTestTenant creates a tenant with the real migrations and removes it
// when the test ends.
func createTestTenant(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	id, err := NewProvisioner(pool, migrations.Tenant).CreateTenant(t.Context(), testSlug(), "Test Operator")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	t.Cleanup(func() { dropTenant(t, pool, id) })
	return id
}

func dropTenant(t *testing.T, pool *pgxpool.Pool, id int64) {
	t.Helper()
	ctx := context.Background() // t.Context is already cancelled during Cleanup
	schema := pgx.Identifier{SchemaName(id)}.Sanitize()
	if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); err != nil {
		t.Errorf("drop schema: %v", err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM platform.tenants WHERE id = $1", id); err != nil {
		t.Errorf("delete tenant: %v", err)
	}
}

func schemaExists(t *testing.T, pool *pgxpool.Pool, schema string) bool {
	t.Helper()
	var exists bool
	err := pool.QueryRow(t.Context(),
		"SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = $1)", schema).Scan(&exists)
	if err != nil {
		t.Fatalf("look up schema: %v", err)
	}
	return exists
}

func tenantRowExists(t *testing.T, pool *pgxpool.Pool, slug string) bool {
	t.Helper()
	var exists bool
	err := pool.QueryRow(t.Context(),
		"SELECT EXISTS (SELECT 1 FROM platform.tenants WHERE slug = $1)", slug).Scan(&exists)
	if err != nil {
		t.Fatalf("look up tenant: %v", err)
	}
	return exists
}

// inTenant runs sql with the tenant's schema first on the search_path, in a
// transaction that is rolled back, and returns its error.
func inTenant(t *testing.T, pool *pgxpool.Pool, id int64, sql string) error {
	t.Helper()
	tx, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(context.Background())
	path := pgx.Identifier{SchemaName(id)}.Sanitize() + ", shared"
	if _, err := tx.Exec(t.Context(), "SET LOCAL search_path = "+path); err != nil {
		t.Fatalf("set search_path: %v", err)
	}
	_, err = tx.Exec(t.Context(), sql)
	return err
}

func TestCreateTenant(t *testing.T) {
	pool := testPool(t)
	id := createTestTenant(t, pool)
	schema := SchemaName(id)

	var status string
	if err := pool.QueryRow(t.Context(),
		"SELECT status FROM platform.tenants WHERE id = $1", id).Scan(&status); err != nil {
		t.Fatalf("read tenant: %v", err)
	}
	if status != "active" {
		t.Errorf("status = %q, want active", status)
	}

	// goose's version table must be in the tenant's schema, not in public or
	// shared: each tenant tracks its own version.
	for _, table := range []string{"locations", "spaces", "opening_windows", "members", "goose_db_version"} {
		var exists bool
		err := pool.QueryRow(t.Context(),
			"SELECT EXISTS (SELECT 1 FROM pg_tables WHERE schemaname = $1 AND tablename = $2)",
			schema, table).Scan(&exists)
		if err != nil {
			t.Fatalf("look up %s: %v", table, err)
		}
		if !exists {
			t.Errorf("table %s.%s is missing", schema, table)
		}
	}

	var version int64
	err := pool.QueryRow(t.Context(),
		"SELECT max(version_id) FROM "+pgx.Identifier{schema, "goose_db_version"}.Sanitize()).Scan(&version)
	if err != nil {
		t.Fatalf("read version: %v", err)
	}
	if version != 1 {
		t.Errorf("version = %d, want 1", version)
	}
}

func TestCreateTenantCleansUpAfterFailedMigration(t *testing.T) {
	pool := testPool(t)
	broken := fstest.MapFS{
		"00001_ok.sql":     {Data: []byte("-- +goose Up\nCREATE TABLE ok (id int);\n")},
		"00002_broken.sql": {Data: []byte("-- +goose Up\nCREATE TABLE ok (id int);\n")}, // already exists
	}
	slug := testSlug()

	id, err := NewProvisioner(pool, broken).CreateTenant(t.Context(), slug, "Broken Operator")
	if err == nil {
		dropTenant(t, pool, id)
		t.Fatal("CreateTenant succeeded with a broken migration")
	}
	if tenantRowExists(t, pool, slug) {
		t.Error("tenant row was left behind")
	}
	// The id isn't returned on failure; the schema name can only be checked
	// by pattern, so look for any tenant schema without a tenant row.
	var orphans int
	err = pool.QueryRow(t.Context(), `
		SELECT count(*) FROM pg_namespace n
		WHERE n.nspname LIKE 'tenant\_%'
		  AND NOT EXISTS (SELECT 1 FROM platform.tenants t WHERE 'tenant_' || t.id = n.nspname)`).Scan(&orphans)
	if err != nil {
		t.Fatalf("look for orphan schemas: %v", err)
	}
	if orphans != 0 {
		t.Errorf("%d tenant schemas have no tenant row", orphans)
	}
}

func TestCreateTenantRefusesTakenSlug(t *testing.T) {
	pool := testPool(t)
	p := NewProvisioner(pool, migrations.Tenant)
	slug := testSlug()
	id, err := p.CreateTenant(t.Context(), slug, "First")
	if err != nil {
		t.Fatalf("first CreateTenant: %v", err)
	}
	t.Cleanup(func() { dropTenant(t, pool, id) })

	if _, err := p.CreateTenant(t.Context(), slug, "Second"); err == nil {
		t.Fatal("second CreateTenant with the same slug succeeded")
	}
	if schemaExists(t, pool, SchemaName(id+1)) {
		t.Errorf("schema %s exists after a refused tenant", SchemaName(id+1))
	}
}

func TestTenantsAreSeparate(t *testing.T) {
	pool := testPool(t)
	a := createTestTenant(t, pool)
	b := createTestTenant(t, pool)

	ctx := t.Context()
	insert := "INSERT INTO " + pgx.Identifier{SchemaName(a), "members"}.Sanitize() +
		" (name, email) VALUES ('Ana', 'ana@example.com')"
	if _, err := pool.Exec(ctx, insert); err != nil {
		t.Fatalf("insert member: %v", err)
	}

	count := func(id int64) int {
		var n int
		q := "SELECT count(*) FROM " + pgx.Identifier{SchemaName(id), "members"}.Sanitize()
		if err := pool.QueryRow(ctx, q).Scan(&n); err != nil {
			t.Fatalf("count members: %v", err)
		}
		return n
	}
	if got := count(a); got != 1 {
		t.Errorf("tenant a has %d members, want 1", got)
	}
	if got := count(b); got != 0 {
		t.Errorf("tenant b has %d members, want 0", got)
	}
}

func TestTenantConstraints(t *testing.T) {
	pool := testPool(t)
	id := createTestTenant(t, pool)

	// Each case starts from one location, one space and one member.
	const setup = `
		INSERT INTO locations (name, timezone) VALUES ('Centre', 'Europe/Madrid');
		INSERT INTO spaces (location_id, name, kind, capacity, max_booking_length)
			SELECT id, 'Room A', 'meeting_room', 6, '2 hours' FROM locations;
		INSERT INTO members (name, email) VALUES ('Ana', 'ana@example.com');
	`
	tests := []struct {
		name       string
		sql        string
		constraint string // "" means the statement must succeed
	}{
		{"valid window", "INSERT INTO opening_windows SELECT id, 1, 540, 1080 FROM spaces", ""},
		{"window to end of day", "INSERT INTO opening_windows SELECT id, 1, 0, 1440 FROM spaces", ""},
		{"blank location name", "INSERT INTO locations (name, timezone) VALUES (' ', 'UTC')", "locations_name_not_blank"},
		{"blank timezone", "INSERT INTO locations (name, timezone) VALUES ('North', '')", "locations_timezone_not_blank"},
		{"unknown kind", "INSERT INTO spaces (location_id, name, kind, capacity, max_booking_length) SELECT id, 'B', 'sauna', 1, '1 hour' FROM locations", "spaces_kind_valid"},
		{"capacity 0", "INSERT INTO spaces (location_id, name, kind, capacity, max_booking_length) SELECT id, 'B', 'hot_desk', 0, '1 hour' FROM locations", "spaces_capacity_positive"},
		{"max length 0", "INSERT INTO spaces (location_id, name, kind, capacity, max_booking_length) SELECT id, 'B', 'hot_desk', 1, '0' FROM locations", "spaces_max_booking_length_valid"},
		{"max length 25h", "INSERT INTO spaces (location_id, name, kind, capacity, max_booking_length) SELECT id, 'B', 'hot_desk', 1, '25 hours' FROM locations", "spaces_max_booking_length_valid"},
		{"unknown location", "INSERT INTO spaces (location_id, name, kind, capacity, max_booking_length) VALUES (999999, 'B', 'hot_desk', 1, '1 hour')", "spaces_location_id_fkey"},
		{"weekday 7", "INSERT INTO opening_windows SELECT id, 7, 540, 1080 FROM spaces", "opening_windows_weekday_valid"},
		{"closes after 24:00", "INSERT INTO opening_windows SELECT id, 1, 540, 1441 FROM spaces", "opening_windows_closes_valid"},
		{"opens after closing", "INSERT INTO opening_windows SELECT id, 1, 1080, 540 FROM spaces", "opening_windows_opens_before_closes"},
		{"mixed-case email", "INSERT INTO members (name, email) VALUES ('Bo', 'Bo@example.com')", "members_email_normalized"},
		{"email taken", "INSERT INTO members (name, email) VALUES ('Ana Two', 'ana@example.com')", "members_email_key"},
		{"email of a deleted member", "UPDATE members SET deleted_at = now(); INSERT INTO members (name, email) VALUES ('Ana', 'ana@example.com')", ""},
		{"deleting a space deletes its windows", "INSERT INTO opening_windows SELECT id, 1, 540, 1080 FROM spaces; DELETE FROM spaces", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := inTenant(t, pool, id, setup+tt.sql)
			if tt.constraint == "" {
				if err != nil {
					t.Fatalf("refused: %v", err)
				}
				return
			}
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) {
				t.Fatalf("err = %v, want a Postgres error from %s", err, tt.constraint)
			}
			if pgErr.ConstraintName != tt.constraint {
				t.Errorf("refused by %q (%s), want %q", pgErr.ConstraintName, pgErr.Message, tt.constraint)
			}
		})
	}
}
