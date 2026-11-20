//go:build integration

package repl_test

import (
	"context"
	"fmt"
	"math/rand/v2"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nicoaravena/cow-book-app/cowbook"
	"nicoaravena/cow-book-app/internal/postgres"
	"nicoaravena/cow-book-app/internal/repl"
	"nicoaravena/cow-book-app/migrations"
)

// pgTenant creates a tenant in DATABASE_URL and removes it when the test
// ends. internal/postgres has the same helpers, but in its own test files,
// which other packages can't import.
func pgTenant(t *testing.T) (*pgxpool.Pool, int64) {
	t.Helper()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		t.Fatal("DATABASE_URL is not set; integration tests need the compose database")
	}
	pool, err := postgres.NewPool(t.Context(), url)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	slug := fmt.Sprintf("test-%d", rand.Int64N(1_000_000_000))
	id, err := postgres.NewProvisioner(pool, migrations.Tenant).CreateTenant(t.Context(), slug, "Test Operator")
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background() // t.Context is already cancelled during Cleanup
		schema := pgx.Identifier{postgres.SchemaName(id)}.Sanitize()
		if _, err := pool.Exec(ctx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE"); err != nil {
			t.Errorf("drop schema: %v", err)
		}
		if _, err := pool.Exec(ctx, "DELETE FROM platform.tenants WHERE id = $1", id); err != nil {
			t.Errorf("delete tenant: %v", err)
		}
	})
	return pool, id
}

// runPostgres runs script in a new session over the tenant's Postgres stores,
// built the way cmd/cowbook builds them, and returns all it printed.
func runPostgres(t *testing.T, pool *pgxpool.Pool, tenantID int64, script ...string) string {
	t.Helper()
	clock := repl.NewClock(start)
	stores := postgres.NewStores(pool, tenantID)
	s := repl.NewSession(stores,
		cowbook.NewBookingService(stores, stores, stores, clock.Now),
		cowbook.NewMemberService(stores, stores, clock.Now),
		clock)

	var out strings.Builder
	in := strings.NewReader(strings.Join(script, "\n") + "\n")
	if err := s.Run(context.Background(), in, &out, true); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	return out.String()
}

// What one session stores, a second session over the same tenant sees: the
// data outlives the session, as it outlives a restart of cowbook repl.
func TestSessionOverPostgresKeepsData(t *testing.T) {
	pool, tenant := pgTenant(t)

	first := runPostgres(t, pool, tenant,
		"space add Room-A meeting_room 6 2h",
		"space add Desk-1 hot_desk 1 4h mon-fri/09:00-13:00 Europe/Madrid",
		"member add Ana ana@example.com",
		"member add Bo bo@example.com",
		"member add Ann ANA@example.com",
		"as admin 9",
		"book 2 2026-11-17 09:00-10:00 for 2",
		"as member 1",
		"book 1 2026-11-16 09:00-10:00",
		"book 1 2026-11-16 09:30-10:30",
		"member delete 2",
	)
	for _, want := range []string{
		"ok space #1", "ok space #2", "ok member #1", "ok member #2",
		"error: email is already taken by another member",
		"ok booking #1: space 2, Tue 2026-11-17 09:00-10:00 CET, pending: member 2 must approve by Tue 2026-11-17 09:00 CET",
		"ok booking #2: space 1, Mon 2026-11-16 09:00-10:00 UTC",
		"error: space is already booked for that time",
		"ok member #2 deleted",
	} {
		if !strings.Contains(first, want+"\n") {
			t.Errorf("first session lacks %q:\n%s", want, first)
		}
	}

	second := runPostgres(t, pool, tenant, "space list", "member list", "booking list")
	want := `[nobody]> space list
ID  NAME    KIND          CAPACITY  MAX-LENGTH  TIMEZONE
1   Room-A  meeting_room  6         2h0m0s      UTC
2   Desk-1  hot_desk      1         4h0m0s      Europe/Madrid
[nobody]> member list
ID  NAME  EMAIL            STATE
1   Ana   ana@example.com  active
2   Bo    bo@example.com   deleted
[nobody]> booking list
ID  SPACE  MEMBER  WHEN                            ATTENDEES  STATE
1   2      2       Tue 2026-11-17 09:00-10:00 CET  1          cancelled
2   1      1       Mon 2026-11-16 09:00-10:00 UTC  1          confirmed
`
	if second != want {
		t.Errorf("second session:\n%s\nwant:\n%s", second, want)
	}
}
