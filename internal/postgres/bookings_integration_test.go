//go:build integration

package postgres

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"nicoaravena/cow-book-app/migrations"
)

func TestBookingConstraints(t *testing.T) {
	pool := testPool(t)
	id := createTestTenant(t, pool)

	// Each case starts from two spaces, one member, and a confirmed booking of
	// the first space from 10:00 to 11:00. The insert helpers below keep the
	// cases to the columns they're about.
	const setup = `
		INSERT INTO locations (name, timezone) VALUES ('Centre', 'Europe/Madrid');
		INSERT INTO spaces (location_id, name, kind, capacity, max_booking_length)
			SELECT id, 'Room A', 'meeting_room', 6, '2 hours' FROM locations;
		INSERT INTO spaces (location_id, name, kind, capacity, max_booking_length)
			SELECT id, 'Room B', 'meeting_room', 6, '2 hours' FROM locations;
		INSERT INTO members (name, email) VALUES ('Ana', 'ana@example.com');
		INSERT INTO bookings (space_id, member_id, attendees, starts_at, ends_at)
			SELECT min(s.id), min(m.id), 2, '2026-12-01 10:00Z', '2026-12-01 11:00Z'
			FROM spaces s, members m;
	`
	// book inserts a member's booking of the space named room.
	book := func(room, starts, ends string) string {
		return "INSERT INTO bookings (space_id, member_id, attendees, starts_at, ends_at)" +
			" SELECT s.id, m.id, 1, '" + starts + "', '" + ends + "'" +
			" FROM spaces s, members m WHERE s.name = '" + room + "'"
	}
	const overlap = "INSERT INTO bookings (space_id, member_id, attendees, starts_at, ends_at)" +
		" SELECT min(s.id), min(m.id), 1, '2026-12-01 10:30Z', '2026-12-01 11:30Z' FROM spaces s, members m"
	// staffBooking inserts a booking staff member 7 made of Room B at 15:00,
	// with the given approval deadline.
	staffBooking := func(deadline string) string {
		return "INSERT INTO bookings (space_id, member_id, booked_by, attendees, starts_at, ends_at, approval_deadline)" +
			" SELECT s.id, m.id, 7, 1, '2026-12-01 15:00Z', '2026-12-01 16:00Z', '" + deadline + "'" +
			" FROM spaces s, members m WHERE s.name = 'Room B'; "
	}

	tests := []struct {
		name       string
		sql        string
		constraint string // "" means the statement must succeed
	}{
		{"overlapping booking", overlap, "bookings_no_overlap"},
		{"same time in another space", book("Room B", "2026-12-01 10:00Z", "2026-12-01 11:00Z"), ""},
		{"starts when the other ends", book("Room A", "2026-12-01 11:00Z", "2026-12-01 12:00Z"), ""},
		{"ends when the other starts", book("Room A", "2026-12-01 09:00Z", "2026-12-01 10:00Z"), ""},
		{"inside the other", book("Room A", "2026-12-01 10:15Z", "2026-12-01 10:45Z"), "bookings_no_overlap"},
		{"same slot in another timezone", book("Room A", "2026-12-01 11:00+01", "2026-12-01 12:00+01"), "bookings_no_overlap"},
		{"overlapping a cancelled booking", "UPDATE bookings SET cancelled_at = now(); " + overlap, ""},
		{"overlapping a pending booking",
			staffBooking("2026-11-30 15:00Z") + book("Room B", "2026-12-01 15:30Z", "2026-12-01 16:30Z"),
			"bookings_no_overlap"},
		{"overlapping a lapsed pending booking",
			staffBooking("2000-01-01 00:00Z") + book("Room B", "2026-12-01 15:30Z", "2026-12-01 16:30Z"),
			"bookings_no_overlap"},
		{"approved staff booking", staffBooking("2026-11-30 15:00Z") + "UPDATE bookings SET approved_at = now() WHERE booked_by IS NOT NULL", ""},
		{"ends before it starts", book("Room A", "2026-12-01 13:00Z", "2026-12-01 12:00Z"), "bookings_starts_before_ends"},
		{"empty booking", book("Room A", "2026-12-01 13:00Z", "2026-12-01 13:00Z"), "bookings_starts_before_ends"},
		{"no attendees", "UPDATE bookings SET attendees = 0", "bookings_attendees_positive"},
		{"staff booking without deadline", "UPDATE bookings SET booked_by = 7", "bookings_approval_iff_staff"},
		{"member booking with deadline", "UPDATE bookings SET approval_deadline = now()", "bookings_approval_iff_staff"},
		{"approving a member booking", "UPDATE bookings SET approved_at = now()", "bookings_approved_needs_deadline"},
		{"unknown space", "INSERT INTO bookings (space_id, member_id, attendees, starts_at, ends_at) SELECT 999999, id, 1, '2026-12-02 10:00Z', '2026-12-02 11:00Z' FROM members", "bookings_space_id_fkey"},
		{"unknown member", "INSERT INTO bookings (space_id, member_id, attendees, starts_at, ends_at) SELECT min(id), 999999, 1, '2026-12-02 10:00Z', '2026-12-02 11:00Z' FROM spaces", "bookings_member_id_fkey"},
		{"deleting a member with bookings", "DELETE FROM members", "bookings_member_id_fkey"},
		{"deleting a space with bookings", "DELETE FROM spaces", "bookings_space_id_fkey"},
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
			// 2.5 turns this code into ErrSlotTaken, so pin it here.
			if tt.constraint == "bookings_no_overlap" && pgErr.Code != "23P01" {
				t.Errorf("code = %s, want 23P01 (exclusion_violation)", pgErr.Code)
			}
		})
	}
}

// TestTenantMigrationsDownAndUp takes a tenant schema all the way down and up
// again, which proves every tenant Down section works. make migrate-check only
// covers the platform migrations.
func TestTenantMigrationsDownAndUp(t *testing.T) {
	pool := testPool(t)
	id := createTestTenant(t, pool)

	// The same connection setup as Provisioner.migrate.
	cfg := pool.Config().ConnConfig.Copy()
	cfg.RuntimeParams["search_path"] = pgx.Identifier{SchemaName(id)}.Sanitize() + ", shared"
	db := stdlib.OpenDB(*cfg)
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.Tenant)
	if err != nil {
		t.Fatalf("goose provider: %v", err)
	}

	if _, err := provider.DownTo(t.Context(), 0); err != nil {
		t.Fatalf("down: %v", err)
	}
	if v, err := provider.GetDBVersion(t.Context()); err != nil || v != 0 {
		t.Fatalf("version after down = %d, %v; want 0", v, err)
	}
	if _, err := provider.Up(t.Context()); err != nil {
		t.Fatalf("up again: %v", err)
	}
	if v, err := provider.GetDBVersion(t.Context()); err != nil || v != 2 {
		t.Fatalf("version after up = %d, %v; want 2", v, err)
	}
}
