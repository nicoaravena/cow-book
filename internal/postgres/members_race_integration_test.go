//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"nicoaravena/cow-book-app/cowbook"
)

// The first two tests stop a transaction half-way on purpose, so the order of
// events is fixed rather than left to a race: one connection takes a lock and
// holds it, the store's call waits on it, and only then does the first
// transaction commit.

// holdTx begins a transaction on a connection of its own and returns it with
// the server process ID behind that connection. The transaction is rolled
// back when the test ends unless it was committed first. Its SQL names the
// tenant's schema rather than setting the search_path.
func (f *bookingFixture) holdTx(t *testing.T) (pgx.Tx, uint32) {
	t.Helper()
	tx, err := f.pool.Begin(t.Context())
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	// t.Context is done by the time cleanups run.
	t.Cleanup(func() { tx.Rollback(context.Background()) })
	return tx, tx.Conn().PgConn().PID()
}

// waitUntilBlockedBy waits until some other connection is waiting for a lock
// that process pid holds, and fails the test after five seconds.
// pg_blocking_pids lists the processes a process waits for.
func (f *bookingFixture) waitUntilBlockedBy(t *testing.T, pid uint32) {
	t.Helper()
	const query = "SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE $1::int = ANY (pg_blocking_pids(pid)))"
	deadline := time.Now().Add(5 * time.Second)
	for {
		var blocked bool
		if err := f.pool.QueryRow(t.Context(), query, int32(pid)).Scan(&blocked); err != nil {
			t.Fatalf("pg_blocking_pids: %v", err)
		}
		if blocked {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("nothing waited for the held transaction")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// A deletion has marked the member deleted but not committed. CreateBooking
// waits on the member's row, then sees the deletion and refuses. Without the
// wait it would read the member as not deleted and insert.
func TestBookingWaitsForADeletionInFlight(t *testing.T) {
	f := newBookingFixture(t, 1)
	id := f.members[0]
	b := f.newBooking(t, cowbook.MemberActor(id), 0, id, 10, 12, bookingNow)

	tx, pid := f.holdTx(t)
	_, err := tx.Exec(t.Context(),
		"UPDATE "+f.schema()+".members SET deleted_at = $1, updated_at = $1 WHERE id = $2", bookingNow, id)
	if err != nil {
		t.Fatalf("delete member: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		_, err := f.store.CreateBooking(t.Context(), b)
		done <- err
	}()
	f.waitUntilBlockedBy(t, pid)
	if err := tx.Commit(t.Context()); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if err := <-done; !errors.Is(err, cowbook.ErrMemberDeleted) {
		t.Errorf("CreateBooking error = %v, want %v", err, cowbook.ErrMemberDeleted)
	}
	left, err := f.store.ListMemberBookingsEndingAfter(t.Context(), id, bookingNow)
	if err != nil {
		t.Fatalf("ListMemberBookingsEndingAfter: %v", err)
	}
	if len(left) != 0 {
		t.Errorf("member has %d active bookings, want 0", len(left))
	}
}

// The mirror case: a booking insert holds the member's row and hasn't
// committed when the deletion is saved, with a list of bookings read before
// that insert. SaveDeletion waits on the row, then its last statement sees the
// new booking and cancels it.
func TestDeletionWaitsForABookingInFlight(t *testing.T) {
	f := newBookingFixture(t, 1)
	id := f.members[0]

	tx, pid := f.holdTx(t)
	if _, err := tx.Exec(t.Context(), "SELECT 1 FROM "+f.schema()+".members WHERE id = $1 FOR SHARE", id); err != nil {
		t.Fatalf("lock member: %v", err)
	}
	var bookingID int64
	err := tx.QueryRow(t.Context(),
		"INSERT INTO "+f.schema()+".bookings (space_id, member_id, attendees, starts_at, ends_at)"+
			" VALUES ($1, $2, 1, $3, $4) RETURNING id",
		f.spaces[0].ID, id, f.at(10), f.at(12)).Scan(&bookingID)
	if err != nil {
		t.Fatalf("insert booking: %v", err)
	}

	m := f.deleted(t, id, bookingNow)
	done := make(chan error, 1)
	go func() { done <- f.mstore.SaveDeletion(t.Context(), m, nil) }()
	f.waitUntilBlockedBy(t, pid)
	if err := tx.Commit(t.Context()); err != nil {
		t.Fatalf("commit: %v", err)
	}

	if err := <-done; err != nil {
		t.Fatalf("SaveDeletion: %v", err)
	}
	if got := f.get(t, bookingID); !got.CancelledAt().Equal(bookingNow) {
		t.Errorf("CancelledAt() = %v, want %v", got.CancelledAt(), bookingNow)
	}
}

func TestBookingStoreRaceToCancel(t *testing.T) {
	f := newBookingFixture(t, 1)
	actor := cowbook.MemberActor(f.members[0])
	stored := f.create(t, f.newBooking(t, actor, 0, f.members[0], 10, 12, bookingNow))

	// Each racer read the booking before anyone saved, so each passes Cancel;
	// the store refuses all but one.
	errs := race(racers, func(int) error {
		b := stored
		if err := b.Cancel(actor, f.spaces[0], bookingNow); err != nil {
			return err
		}
		return f.store.SaveCancellation(t.Context(), b)
	})

	checkCounts(t, errs, 1, cowbook.ErrAlreadyCancelled)
}

// services returns the booking and member services over the fixture's three
// Postgres stores, with the clock stopped at bookingNow.
func (f *bookingFixture) services() (*cowbook.BookingService, *cowbook.MemberService) {
	clock := func() time.Time { return bookingNow }
	spaces := NewSpaceStore(f.pool, f.tenant)
	return cowbook.NewBookingService(f.store, spaces, f.mstore, clock),
		cowbook.NewMemberService(f.mstore, f.store, clock)
}

// internal/memstore's TestRaceToDeleteMember, ported.
func TestMemberStoreRaceToDeleteMember(t *testing.T) {
	f := newBookingFixture(t, 2)
	m1, m2 := f.members[0], f.members[1]
	bookings, members := f.services()
	slot := cowbook.BookingParams{MemberID: m1, Attendees: 1, Start: f.at(10), End: f.at(12)}
	for _, b := range []struct {
		actor cowbook.Actor
		space int
		p     cowbook.BookingParams
	}{
		{cowbook.MemberActor(m1), 0, slot},
		{cowbook.MemberActor(m1), 1, slot},
		{cowbook.AdminActor(9), 0, cowbook.BookingParams{MemberID: m1, Attendees: 1, Start: f.at(12), End: f.at(13)}},
	} {
		if _, err := bookings.Book(t.Context(), b.actor, f.spaces[b.space].ID, b.p); err != nil {
			t.Fatalf("Book: %v", err)
		}
	}

	errs := race(racers, func(int) error {
		_, err := members.Delete(t.Context(), m1)
		return err
	})

	checkCounts(t, errs, 1, cowbook.ErrMemberDeleted)
	if m := f.getMember(t, m1); !m.Deleted() {
		t.Error("member is not deleted")
	}
	left, err := f.store.ListMemberBookingsEndingAfter(t.Context(), m1, bookingNow)
	if err != nil {
		t.Fatalf("ListMemberBookingsEndingAfter: %v", err)
	}
	if len(left) != 0 {
		t.Errorf("member has %d active bookings, want 0", len(left))
	}
	slot.MemberID = m2
	if _, err := bookings.Book(t.Context(), cowbook.MemberActor(m2), f.spaces[0].ID, slot); err != nil {
		t.Errorf("Book of the freed slot: %v", err)
	}
}

// Racers book one-hour slots for a member while one racer deletes them. Each
// booking either is refused because the member is deleted or ends up
// cancelled; none is left active.
func TestRaceToBookWhileDeleting(t *testing.T) {
	f := newBookingFixture(t, 1)
	id := f.members[0]
	bookings, members := f.services()

	// Two spaces, 09:00 to 17:00: 16 distinct slots, plus the deletion.
	const bookers = 16
	booked := make([]cowbook.Booking, bookers+1)
	errs := race(bookers+1, func(i int) error {
		if i == bookers {
			_, err := members.Delete(t.Context(), id)
			return err
		}
		p := cowbook.BookingParams{MemberID: id, Attendees: 1, Start: f.at(9 + i/2), End: f.at(10 + i/2)}
		var err error
		booked[i], err = bookings.Book(t.Context(), cowbook.MemberActor(id), f.spaces[i%2].ID, p)
		return err
	})

	if errs[bookers] != nil {
		t.Fatalf("Delete: %v", errs[bookers])
	}
	for i, err := range errs[:bookers] {
		switch {
		case errors.Is(err, cowbook.ErrMemberDeleted):
		case err != nil:
			t.Errorf("Book %d: %v, want success or %v", i, err, cowbook.ErrMemberDeleted)
		case !f.get(t, booked[i].ID).Cancelled():
			t.Errorf("Book %d: booking %d is active after the deletion", i, booked[i].ID)
		}
	}
}
