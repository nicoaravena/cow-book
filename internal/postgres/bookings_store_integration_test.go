//go:build integration

package postgres

import (
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"nicoaravena/cow-book-app/cowbook"
)

// These tests are internal/memstore's booking tests ported to Postgres, with
// the same names after the BookingStore prefix, plus the lazy expiry the
// in-memory store doesn't have.

// bookingNow is the clock used by these tests: Monday 2 November 2026, 09:00
// UTC. Every test booking is a week later, so a staff booking's approval
// deadline is 24 hours after this.
var bookingNow = time.Date(2026, time.November, 2, 9, 0, 0, 0, time.UTC)

// lapse is when a staff booking made at bookingNow stops waiting for approval.
var lapse = bookingNow.Add(24 * time.Hour)

// bookingFixture is a test tenant with what a booking needs: two spaces at
// one location, open 09:00 to 18:00 on weekdays with bookings up to 4 hours
// as in memstore's tests, and some members.
type bookingFixture struct {
	pool    *pgxpool.Pool
	tenant  int64
	store   *BookingStore
	mstore  *MemberStore
	spaces  [2]cowbook.Space
	loc     *time.Location
	members []int64
}

// newBookingFixture creates a tenant holding two spaces and n members.
func newBookingFixture(t *testing.T, n int) *bookingFixture {
	t.Helper()
	pool := testPool(t)
	f := &bookingFixture{pool: pool, tenant: createTestTenant(t, pool)}
	f.store = NewBookingStore(pool, f.tenant)
	f.mstore = NewMemberStore(pool, f.tenant)

	spaces := NewSpaceStore(pool, f.tenant)
	locationID := createLocation(t, pool, f.tenant, "Centre")
	for i, name := range []string{"Room A", "Room B"} {
		p := spaceParams(t, locationID, name, weekdays(t, 9, 18))
		p.MaxBookingLength = 4 * time.Hour
		sp := createSpace(t, spaces, p, bookingNow)
		// Read back, so the fixture holds the location's timezone too.
		var err error
		if f.spaces[i], f.loc, err = spaces.GetSpace(t.Context(), sp.ID); err != nil {
			t.Fatalf("GetSpace: %v", err)
		}
	}

	// MemberStore can't create members until 2.7, so plain SQL. generate_series gives one row
	// per number, and the emails differ because the table wants them unique.
	rows, err := pool.Query(t.Context(),
		"INSERT INTO "+f.schema()+".members (name, email)"+
			" SELECT 'Member', 'member' || g || '@example.com' FROM generate_series(1, $1) g"+
			" RETURNING id", n)
	if err != nil {
		t.Fatalf("insert members: %v", err)
	}
	if f.members, err = pgx.CollectRows(rows, pgx.RowTo[int64]); err != nil {
		t.Fatalf("insert members: %v", err)
	}
	return f
}

// schema returns the tenant's schema name, quoted for use in SQL.
func (f *bookingFixture) schema() string {
	return pgx.Identifier{SchemaName(f.tenant)}.Sanitize()
}

// at returns hour:00 on Monday 9 November 2026 at the fixture's location.
func (f *bookingFixture) at(hour int) time.Time {
	return time.Date(2026, time.November, 9, hour, 0, 0, 0, f.loc)
}

// newBooking returns a booking by actor for memberID of f.spaces[space], from
// startHour to endHour on Monday 9 November, made at now and not yet stored.
func (f *bookingFixture) newBooking(t *testing.T, actor cowbook.Actor, space int, memberID int64, startHour, endHour int, now time.Time) cowbook.Booking {
	t.Helper()
	b, err := cowbook.NewBooking(actor, f.spaces[space], f.loc, cowbook.BookingParams{
		MemberID:  memberID,
		Attendees: 1,
		Start:     f.at(startHour),
		End:       f.at(endHour),
	}, now)
	if err != nil {
		t.Fatalf("NewBooking: %v", err)
	}
	return b
}

// create stores b and fails the test if the store refuses it.
func (f *bookingFixture) create(t *testing.T, b cowbook.Booking) cowbook.Booking {
	t.Helper()
	b, err := f.store.CreateBooking(t.Context(), b)
	if err != nil {
		t.Fatalf("CreateBooking: %v", err)
	}
	return b
}

// cancel cancels b on behalf of actor at now with f.store.SaveCancellation
// and fails the test if either refuses. It returns the booking as cancelled.
func (f *bookingFixture) cancel(t *testing.T, b cowbook.Booking, actor cowbook.Actor, now time.Time) cowbook.Booking {
	t.Helper()
	space := f.spaces[0]
	if b.SpaceID() == f.spaces[1].ID {
		space = f.spaces[1]
	}
	if err := b.Cancel(actor, space, now); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if err := f.store.SaveCancellation(t.Context(), b); err != nil {
		t.Fatalf("SaveCancellation: %v", err)
	}
	return b
}

// deleteMember marks member id deleted at now with plain SQL, leaving their
// bookings as they are.
func (f *bookingFixture) deleteMember(t *testing.T, id int64, now time.Time) {
	t.Helper()
	_, err := f.pool.Exec(t.Context(),
		"UPDATE "+f.schema()+".members SET deleted_at = $1, updated_at = $1 WHERE id = $2", now, id)
	if err != nil {
		t.Fatalf("delete member: %v", err)
	}
}

// get reads booking id back and fails the test if it can't.
func (f *bookingFixture) get(t *testing.T, id int64) cowbook.Booking {
	t.Helper()
	b, err := f.store.GetBooking(t.Context(), id)
	if err != nil {
		t.Fatalf("GetBooking(%d): %v", id, err)
	}
	return b
}

// checkSameBooking fails the test unless got and want hold the same values.
// Bookings can't be compared with ==: a stored time comes back cut to the
// microsecond and in time.Local, so each time is compared with Equal after
// cutting want's the same way.
func checkSameBooking(t *testing.T, got, want cowbook.Booking) {
	t.Helper()
	if got.ID != want.ID || got.SpaceID() != want.SpaceID() || got.MemberID() != want.MemberID() ||
		got.BookedBy() != want.BookedBy() || got.Attendees() != want.Attendees() {
		t.Errorf("ID, SpaceID, MemberID, BookedBy, Attendees = %d, %d, %d, %d, %d; want %d, %d, %d, %d, %d",
			got.ID, got.SpaceID(), got.MemberID(), got.BookedBy(), got.Attendees(),
			want.ID, want.SpaceID(), want.MemberID(), want.BookedBy(), want.Attendees())
	}
	times := []struct {
		name      string
		got, want time.Time
	}{
		{"Start", got.Start(), want.Start()},
		{"End", got.End(), want.End()},
		{"ApprovalDeadline", got.ApprovalDeadline(), want.ApprovalDeadline()},
		{"ApprovedAt", got.ApprovedAt(), want.ApprovedAt()},
		{"CancelledAt", got.CancelledAt(), want.CancelledAt()},
		{"CreatedAt", got.CreatedAt(), want.CreatedAt()},
		{"UpdatedAt", got.UpdatedAt(), want.UpdatedAt()},
	}
	for _, tt := range times {
		if w := tt.want.Truncate(time.Microsecond); !tt.got.Equal(w) {
			t.Errorf("%s() = %v, want %v", tt.name, tt.got, w)
		}
	}
}

func TestBookingStoreCreateAndGet(t *testing.T) {
	f := newBookingFixture(t, 1)
	// Nanoseconds on purpose: Postgres keeps microseconds.
	now := bookingNow.Add(123456789 * time.Nanosecond)

	own := f.create(t, f.newBooking(t, cowbook.MemberActor(f.members[0]), 0, f.members[0], 10, 11, now))
	staff := f.create(t, f.newBooking(t, cowbook.AdminActor(9), 0, f.members[0], 12, 13, now))

	if own.ID <= 0 || staff.ID <= own.ID {
		t.Fatalf("IDs = %d, %d; want positive and increasing", own.ID, staff.ID)
	}
	checkSameBooking(t, f.get(t, own.ID), own)
	checkSameBooking(t, f.get(t, staff.ID), staff)
	if got := f.get(t, staff.ID); got.BookedBy() != 9 || got.State(now) != cowbook.StatePending {
		t.Errorf("staff booking: BookedBy, State = %d, %q; want 9, pending", got.BookedBy(), got.State(now))
	}
}

func TestBookingStoreRefusesOverlaps(t *testing.T) {
	tests := []struct {
		name      string
		space     int
		startHour int
		endHour   int
		wantErr   error
	}{
		{"same time", 0, 10, 12, cowbook.ErrSlotTaken},
		{"starts inside", 0, 11, 13, cowbook.ErrSlotTaken},
		{"ends inside", 0, 9, 11, cowbook.ErrSlotTaken},
		{"covers it", 0, 9, 13, cowbook.ErrSlotTaken},
		{"inside it", 0, 10, 11, cowbook.ErrSlotTaken},
		{"starts at its end", 0, 12, 13, nil},
		{"ends at its start", 0, 9, 10, nil},
		{"other space", 1, 10, 12, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newBookingFixture(t, 2)
			first, second := f.members[0], f.members[1]
			f.create(t, f.newBooking(t, cowbook.MemberActor(first), 0, first, 10, 12, bookingNow))

			_, err := f.store.CreateBooking(t.Context(),
				f.newBooking(t, cowbook.MemberActor(second), tt.space, second, tt.startHour, tt.endHour, bookingNow))
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("CreateBooking error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestBookingStorePendingBookingHoldsItsSlot(t *testing.T) {
	f := newBookingFixture(t, 2)
	f.create(t, f.newBooking(t, cowbook.AdminActor(9), 0, f.members[0], 10, 12, bookingNow))

	_, err := f.store.CreateBooking(t.Context(),
		f.newBooking(t, cowbook.MemberActor(f.members[1]), 0, f.members[1], 10, 12, bookingNow))
	if !errors.Is(err, cowbook.ErrSlotTaken) {
		t.Errorf("CreateBooking error = %v, want %v", err, cowbook.ErrSlotTaken)
	}
}

// Two requests can both read a pending booking, both pass Approve and both
// try to save. Only the first may get through, and a booking cancelled in
// between can't be approved.
func TestBookingStoreSaveApprovalLosers(t *testing.T) {
	tests := []struct {
		name    string
		winner  func(t *testing.T, f *bookingFixture, b cowbook.Booking) // saved first
		wantErr error
	}{
		{
			name: "approved twice",
			winner: func(t *testing.T, f *bookingFixture, b cowbook.Booking) {
				if err := b.Approve(cowbook.MemberActor(b.MemberID()), bookingNow); err != nil {
					t.Fatalf("Approve: %v", err)
				}
				if err := f.store.SaveApproval(t.Context(), b); err != nil {
					t.Fatalf("winner's SaveApproval: %v", err)
				}
			},
			wantErr: cowbook.ErrNotPending,
		},
		{
			name: "cancelled first",
			winner: func(t *testing.T, f *bookingFixture, b cowbook.Booking) {
				f.cancel(t, b, cowbook.MemberActor(b.MemberID()), bookingNow)
			},
			wantErr: cowbook.ErrAlreadyCancelled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newBookingFixture(t, 1)
			stored := f.create(t, f.newBooking(t, cowbook.AdminActor(9), 0, f.members[0], 10, 12, bookingNow))
			tt.winner(t, f, stored)

			loser := stored
			if err := loser.Approve(cowbook.MemberActor(f.members[0]), bookingNow.Add(time.Hour)); err != nil {
				t.Fatalf("Approve: %v", err)
			}
			if err := f.store.SaveApproval(t.Context(), loser); !errors.Is(err, tt.wantErr) {
				t.Errorf("SaveApproval error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// Approve refuses this already, so SaveApproval's own check is reached only by
// a booking that skipped it, built here with RestoreBooking.
func TestBookingStoreSaveApprovalAfterDeadline(t *testing.T) {
	f := newBookingFixture(t, 1)
	stored := f.create(t, f.newBooking(t, cowbook.AdminActor(9), 0, f.members[0], 10, 12, bookingNow))

	late, err := cowbook.RestoreBooking(cowbook.BookingRecord{
		ID:      stored.ID,
		SpaceID: stored.SpaceID(),
		BookingParams: cowbook.BookingParams{
			MemberID: stored.MemberID(), Attendees: stored.Attendees(), Start: stored.Start(), End: stored.End(),
		},
		BookedBy:         stored.BookedBy(),
		ApprovalDeadline: stored.ApprovalDeadline(),
		ApprovedAt:       lapse,
		CreatedAt:        stored.CreatedAt(),
		UpdatedAt:        lapse,
	})
	if err != nil {
		t.Fatalf("RestoreBooking: %v", err)
	}

	if err := f.store.SaveApproval(t.Context(), late); !errors.Is(err, cowbook.ErrApprovalExpired) {
		t.Errorf("SaveApproval error = %v, want %v", err, cowbook.ErrApprovalExpired)
	}
	checkSameBooking(t, f.get(t, stored.ID), stored)
}

func TestBookingStoreNotFound(t *testing.T) {
	f := newBookingFixture(t, 1)
	unknown := f.newBooking(t, cowbook.AdminActor(9), 0, f.members[0], 10, 12, bookingNow)
	unknown.ID = 999999
	if err := unknown.Approve(cowbook.MemberActor(f.members[0]), bookingNow); err != nil {
		t.Fatalf("Approve: %v", err)
	}

	tests := []struct {
		name string
		call func() error
	}{
		{"GetBooking", func() error { _, err := f.store.GetBooking(t.Context(), 999999); return err }},
		{"SaveCancellation", func() error { return f.store.SaveCancellation(t.Context(), unknown) }},
		{"SaveApproval", func() error { return f.store.SaveApproval(t.Context(), unknown) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); !errors.Is(err, cowbook.ErrBookingNotFound) {
				t.Errorf("error = %v, want %v", err, cowbook.ErrBookingNotFound)
			}
		})
	}
}

// The service loads the space and member first, so these come back only when
// one is removed in between. An unknown member is found by the member lock
// before the insert, an unknown space by the foreign key.
func TestBookingStoreUnknownSpaceOrMember(t *testing.T) {
	f := newBookingFixture(t, 1)

	ghostMember := f.newBooking(t, cowbook.AdminActor(9), 0, 999999, 10, 12, bookingNow)
	if _, err := f.store.CreateBooking(t.Context(), ghostMember); !errors.Is(err, cowbook.ErrMemberNotFound) {
		t.Errorf("unknown member: CreateBooking error = %v, want %v", err, cowbook.ErrMemberNotFound)
	}

	f.spaces[1].ID = 999999
	ghostSpace := f.newBooking(t, cowbook.MemberActor(f.members[0]), 1, f.members[0], 10, 12, bookingNow)
	if _, err := f.store.CreateBooking(t.Context(), ghostSpace); !errors.Is(err, cowbook.ErrSpaceNotFound) {
		t.Errorf("unknown space: CreateBooking error = %v, want %v", err, cowbook.ErrSpaceNotFound)
	}
}

func TestBookingStoreOtherTenant(t *testing.T) {
	f := newBookingFixture(t, 1)
	b := f.create(t, f.newBooking(t, cowbook.MemberActor(f.members[0]), 0, f.members[0], 10, 12, bookingNow))

	other := NewBookingStore(f.pool, createTestTenant(t, f.pool))
	if _, err := other.GetBooking(t.Context(), b.ID); !errors.Is(err, cowbook.ErrBookingNotFound) {
		t.Errorf("other tenant: GetBooking error = %v, want %v", err, cowbook.ErrBookingNotFound)
	}
}

// A staff booking made at bookingNow for 10:00 to 12:00 lapses at lapse, 24
// hours later. Each case then tries a member booking, made at the case's
// clock, and checks what became of the staff one.
func TestBookingStoreLazyExpiry(t *testing.T) {
	tests := []struct {
		name string
		// before runs after the staff booking is stored; it may change it or
		// add more bookings.
		before        func(t *testing.T, f *bookingFixture, pending cowbook.Booking)
		now           time.Time
		space         int
		startHour     int
		endHour       int
		wantErr       error
		wantCancelled bool // whether the staff booking ends up cancelled
	}{
		{name: "before the deadline", now: lapse.Add(-time.Minute),
			startHour: 10, endHour: 12, wantErr: cowbook.ErrSlotTaken},
		{name: "at the deadline", now: lapse,
			startHour: 10, endHour: 12, wantCancelled: true},
		{name: "overlapping part of it", now: lapse.Add(time.Hour),
			startHour: 11, endHour: 13, wantCancelled: true},
		{name: "starting at its end", now: lapse,
			startHour: 12, endHour: 13},
		{name: "in another space", now: lapse,
			space: 1, startHour: 10, endHour: 12},
		{
			name: "approved in time",
			before: func(t *testing.T, f *bookingFixture, pending cowbook.Booking) {
				if err := pending.Approve(cowbook.MemberActor(pending.MemberID()), lapse.Add(-time.Hour)); err != nil {
					t.Fatalf("Approve: %v", err)
				}
				if err := f.store.SaveApproval(t.Context(), pending); err != nil {
					t.Fatalf("SaveApproval: %v", err)
				}
			},
			now: lapse.Add(time.Hour), startHour: 10, endHour: 12, wantErr: cowbook.ErrSlotTaken,
		},
		{
			// The new booking also overlaps a confirmed one, so its insert
			// fails and the expiry in the same transaction rolls back.
			name: "insert refused",
			before: func(t *testing.T, f *bookingFixture, _ cowbook.Booking) {
				f.create(t, f.newBooking(t, cowbook.MemberActor(f.members[0]), 0, f.members[0], 12, 13, bookingNow))
			},
			now: lapse, startHour: 11, endHour: 13, wantErr: cowbook.ErrSlotTaken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newBookingFixture(t, 2)
			pending := f.create(t, f.newBooking(t, cowbook.AdminActor(9), 0, f.members[0], 10, 12, bookingNow))
			if tt.before != nil {
				tt.before(t, f, pending)
			}

			second := f.members[1]
			_, err := f.store.CreateBooking(t.Context(),
				f.newBooking(t, cowbook.MemberActor(second), tt.space, second, tt.startHour, tt.endHour, tt.now))
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("CreateBooking error = %v, want %v", err, tt.wantErr)
			}

			got := f.get(t, pending.ID)
			if got.Cancelled() != tt.wantCancelled {
				t.Fatalf("staff booking Cancelled() = %t, want %t", got.Cancelled(), tt.wantCancelled)
			}
			// Cancelled by the store's clock, which is the new booking's.
			if tt.wantCancelled && (!got.CancelledAt().Equal(tt.now) || !got.UpdatedAt().Equal(tt.now)) {
				t.Errorf("CancelledAt, UpdatedAt = %v, %v; want %v", got.CancelledAt(), got.UpdatedAt(), tt.now)
			}
		})
	}
}

func TestBookingStoreCancelledBookingFreesItsSlot(t *testing.T) {
	f := newBookingFixture(t, 2)
	b := f.create(t, f.newBooking(t, cowbook.MemberActor(f.members[0]), 0, f.members[0], 10, 12, bookingNow))
	cancelled := f.cancel(t, b, cowbook.MemberActor(f.members[0]), bookingNow)
	checkSameBooking(t, f.get(t, b.ID), cancelled)

	f.create(t, f.newBooking(t, cowbook.MemberActor(f.members[1]), 0, f.members[1], 10, 12, bookingNow))
}

// Two requests can both read a booking, both pass Cancel and both try to
// save. The store must let only the first through.
func TestBookingStoreSaveCancellationLoser(t *testing.T) {
	f := newBookingFixture(t, 1)
	actor := cowbook.MemberActor(f.members[0])
	stored := f.create(t, f.newBooking(t, actor, 0, f.members[0], 10, 12, bookingNow))

	first := f.cancel(t, stored, actor, bookingNow)
	second := stored
	if err := second.Cancel(actor, f.spaces[0], bookingNow.Add(time.Hour)); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if err := f.store.SaveCancellation(t.Context(), second); !errors.Is(err, cowbook.ErrAlreadyCancelled) {
		t.Errorf("second SaveCancellation error = %v, want %v", err, cowbook.ErrAlreadyCancelled)
	}
	checkSameBooking(t, f.get(t, stored.ID), first)
}

// A cancel read the booking before an approval of it was saved. The store
// writes only the cancel columns, so the approval time stays, unlike in
// internal/memstore, which replaces the whole booking.
func TestBookingStoreCancelKeepsApproval(t *testing.T) {
	f := newBookingFixture(t, 1)
	member := cowbook.MemberActor(f.members[0])
	stored := f.create(t, f.newBooking(t, cowbook.AdminActor(9), 0, f.members[0], 10, 12, bookingNow))

	approved := stored
	if err := approved.Approve(member, bookingNow.Add(time.Hour)); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if err := f.store.SaveApproval(t.Context(), approved); err != nil {
		t.Fatalf("SaveApproval: %v", err)
	}
	f.cancel(t, stored, member, bookingNow.Add(2*time.Hour))

	got := f.get(t, stored.ID)
	if !got.Cancelled() || !got.ApprovedAt().Equal(approved.ApprovedAt()) {
		t.Errorf("Cancelled(), ApprovedAt() = %v, %v; want true, %v", got.Cancelled(), got.ApprovedAt(), approved.ApprovedAt())
	}
}

func TestBookingStoreListMemberBookingsEndingAfter(t *testing.T) {
	f := newBookingFixture(t, 2)
	m1, m2 := f.members[0], f.members[1]
	f.create(t, f.newBooking(t, cowbook.MemberActor(m1), 0, m1, 10, 11, bookingNow)) // ends at the cut-off
	later := f.create(t, f.newBooking(t, cowbook.MemberActor(m1), 1, m1, 13, 14, bookingNow))
	earlier := f.create(t, f.newBooking(t, cowbook.MemberActor(m1), 0, m1, 12, 13, bookingNow))
	f.create(t, f.newBooking(t, cowbook.MemberActor(m2), 0, m2, 14, 15, bookingNow)) // another member
	cancelled := f.create(t, f.newBooking(t, cowbook.MemberActor(m1), 0, m1, 15, 16, bookingNow))
	f.cancel(t, cancelled, cowbook.MemberActor(m1), bookingNow)

	got, err := f.store.ListMemberBookingsEndingAfter(t.Context(), m1, f.at(11))
	if err != nil {
		t.Fatalf("ListMemberBookingsEndingAfter: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("ListMemberBookingsEndingAfter returned %d bookings, want 2: %+v", len(got), got)
	}
	// Ordered by start, not by ID.
	checkSameBooking(t, got[0], earlier)
	checkSameBooking(t, got[1], later)
}

// The service refuses a deleted member before it calls the store. This is the
// store's own check, for a member deleted after that.
func TestBookingStoreRefusesDeletedMember(t *testing.T) {
	f := newBookingFixture(t, 1)
	b := f.newBooking(t, cowbook.AdminActor(9), 0, f.members[0], 10, 12, bookingNow)
	f.deleteMember(t, f.members[0], bookingNow)

	if _, err := f.store.CreateBooking(t.Context(), b); !errors.Is(err, cowbook.ErrMemberDeleted) {
		t.Errorf("CreateBooking error = %v, want %v", err, cowbook.ErrMemberDeleted)
	}
}
