//go:build integration

package postgres

import (
	"errors"
	"sync"
	"testing"
	"time"

	"nicoaravena/cow-book-app/cowbook"
)

// racers is how many goroutines each race test starts at once. The pool has
// fewer connections, so some wait for one, but those that have one run their
// transactions in Postgres at the same time.
const racers = 50

// The helpers below are internal/memstore/race_test.go's. Test files can't
// import another package's _test.go files, so each package keeps its copy;
// here they are shared by the race tests in this file.

// race runs call(0) to call(n-1), each in its own goroutine, and returns their
// errors by index. The goroutines wait on start, so they begin together.
func race(n int, call func(i int) error) []error {
	errs := make([]error, n)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			<-start
			errs[i] = call(i)
		})
	}
	close(start)
	wg.Wait()
	return errs
}

// count returns how many errors in errs match target. A nil target counts the
// calls that succeeded.
func count(errs []error, target error) int {
	n := 0
	for _, err := range errs {
		if errors.Is(err, target) {
			n++
		}
	}
	return n
}

// checkCounts fails the test unless errs holds exactly wantOK successes and
// every other call failed with wantErr.
func checkCounts(t *testing.T, errs []error, wantOK int, wantErr error) {
	t.Helper()
	ok, lost := count(errs, nil), count(errs, wantErr)
	if ok != wantOK || ok+lost != len(errs) {
		t.Errorf("%d succeeded and %d got %v out of %d, want %d and %d; errors: %v",
			ok, lost, wantErr, len(errs), wantOK, len(errs)-wantOK, errs)
	}
}

// racingBookings returns one booking per racer, each by members[i] for
// themselves, of the first space from 10:00 to 12:00, made at now. They are
// built before the race because newBooking may call t.Fatalf, which only the
// test's own goroutine may do.
func (f *bookingFixture) racingBookings(t *testing.T, now time.Time) []cowbook.Booking {
	t.Helper()
	bookings := make([]cowbook.Booking, racers)
	for i := range bookings {
		id := f.members[i]
		bookings[i] = f.newBooking(t, cowbook.MemberActor(id), 0, id, 10, 12, now)
	}
	return bookings
}

// The README's rule: many members try to book the same slot at the same
// moment, and only one of them succeeds. Here the exclusion constraint alone
// decides.
func TestBookingStoreRaceForOneSlot(t *testing.T) {
	f := newBookingFixture(t, racers)
	bookings := f.racingBookings(t, bookingNow)

	errs := race(racers, func(i int) error {
		_, err := f.store.CreateBooking(t.Context(), bookings[i])
		return err
	})

	checkCounts(t, errs, 1, cowbook.ErrSlotTaken)
}

// The slot is held by a lapsed staff booking, so every racer first tries to
// expire it. Only one can, and only one booking takes the slot.
func TestBookingStoreRaceForALapsedSlot(t *testing.T) {
	f := newBookingFixture(t, racers+1)
	pending := f.create(t, f.newBooking(t, cowbook.AdminActor(9), 0, f.members[racers], 10, 12, bookingNow))

	bookings := f.racingBookings(t, lapse)

	errs := race(racers, func(i int) error {
		_, err := f.store.CreateBooking(t.Context(), bookings[i])
		return err
	})

	checkCounts(t, errs, 1, cowbook.ErrSlotTaken)
	if got := f.get(t, pending.ID); !got.Cancelled() {
		t.Error("lapsed staff booking is not cancelled")
	}
}

func TestBookingStoreRaceToApprove(t *testing.T) {
	f := newBookingFixture(t, 1)
	pending := f.create(t, f.newBooking(t, cowbook.AdminActor(9), 0, f.members[0], 10, 12, bookingNow))

	// Each racer read the booking before anyone saved, so each passes
	// Approve; the store refuses all but one.
	errs := race(racers, func(int) error {
		b := pending
		if err := b.Approve(cowbook.MemberActor(f.members[0]), bookingNow.Add(time.Hour)); err != nil {
			return err
		}
		return f.store.SaveApproval(t.Context(), b)
	})

	checkCounts(t, errs, 1, cowbook.ErrNotPending)
}
