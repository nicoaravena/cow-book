package memstore_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"nicoaravena/cow-book-app/cowbook"
	"nicoaravena/cow-book-app/internal/memstore"
)

// racers is how many goroutines each test starts at once.
const racers = 50

// race runs call(0) to call(n-1), each in its own goroutine, and returns their
// errors by index. The goroutines wait on start, so they begin together
// instead of one by one as they are created. Each writes only its own element
// of errs, so they share no variable and need no lock; wg.Wait makes their
// writes visible here.
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
	close(start) // a closed channel releases every receiver at once
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

// services returns the booking and member services over store, with the
// clock fixed at now.
func services(store *memstore.Store) (*cowbook.BookingService, *cowbook.MemberService) {
	clock := func() time.Time { return now }
	return cowbook.NewBookingService(store, store, store, clock), cowbook.NewMemberService(store, store, clock)
}

// slot returns booking params for memberID on Monday 9 November, 10:00 to
// 11:00 UTC.
func slot(memberID int64) cowbook.BookingParams {
	return cowbook.BookingParams{MemberID: memberID, Attendees: 1, Start: at(10), End: at(11)}
}

func TestRaceForOneSlot(t *testing.T) {
	store := newStore(t, racers)
	bookings, _ := services(store)

	errs := race(racers, func(i int) error {
		id := int64(i + 1)
		_, err := bookings.Book(context.Background(), cowbook.MemberActor(id), 1, slot(id))
		return err
	})

	checkCounts(t, errs, 1, cowbook.ErrSlotTaken)
}

func TestRaceForAPendingBookingsSlot(t *testing.T) {
	store := newStore(t, racers)
	bookings, _ := services(store)
	ctx := context.Background()
	if _, err := bookings.Book(ctx, cowbook.FrontDeskActor(9, []int64{1}), 1, slot(1)); err != nil {
		t.Fatalf("staff Book() error = %v, want nil", err)
	}

	errs := race(racers, func(i int) error {
		id := int64(i + 1)
		_, err := bookings.Book(ctx, cowbook.MemberActor(id), 1, slot(id))
		return err
	})

	checkCounts(t, errs, 0, cowbook.ErrSlotTaken)
}

func TestRaceToApprove(t *testing.T) {
	store := newStore(t, 1)
	bookings, _ := services(store)
	ctx := context.Background()
	pending, err := bookings.Book(ctx, cowbook.AdminActor(9), 1, slot(1))
	if err != nil {
		t.Fatalf("staff Book() error = %v, want nil", err)
	}

	// A loser that reads the booking after the winner saved is refused by
	// Booking.Approve; one that read it before is refused by the store. Both
	// give ErrNotPending.
	errs := race(racers, func(int) error {
		_, err := bookings.Approve(ctx, cowbook.MemberActor(1), pending.ID)
		return err
	})

	checkCounts(t, errs, 1, cowbook.ErrNotPending)
}

// Cancelling an approved booking is allowed, so the cancel always succeeds.
// The approve succeeds only if it saves first. Which one goes first changes
// from run to run, so the test runs the race many times.
func TestApproveRacingCancel(t *testing.T) {
	for range 20 {
		store := newStore(t, 1)
		bookings, _ := services(store)
		ctx := context.Background()
		pending, err := bookings.Book(ctx, cowbook.AdminActor(9), 1, slot(1))
		if err != nil {
			t.Fatalf("staff Book() error = %v, want nil", err)
		}

		errs := race(2, func(i int) error {
			if i == 0 {
				_, err := bookings.Approve(ctx, cowbook.MemberActor(1), pending.ID)
				return err
			}
			_, err := bookings.Cancel(ctx, cowbook.AdminActor(9), pending.ID)
			return err
		})

		if errs[0] != nil && !errors.Is(errs[0], cowbook.ErrAlreadyCancelled) {
			t.Errorf("Approve() error = %v, want nil or %v", errs[0], cowbook.ErrAlreadyCancelled)
		}
		if errs[1] != nil {
			t.Errorf("Cancel() error = %v, want nil", errs[1])
		}
		if got, _ := store.GetBooking(ctx, pending.ID); !got.Cancelled() {
			t.Errorf("stored booking is not cancelled, want cancelled")
		}
	}
}

func TestRaceToDeleteMember(t *testing.T) {
	store := newStore(t, 2)
	bookings, members := services(store)
	ctx := context.Background()
	for _, b := range []struct {
		actor   cowbook.Actor
		spaceID int64
		p       cowbook.BookingParams
	}{
		{cowbook.MemberActor(1), 1, slot(1)},
		{cowbook.MemberActor(1), 2, slot(1)},
		{cowbook.AdminActor(9), 1, cowbook.BookingParams{MemberID: 1, Attendees: 1, Start: at(12), End: at(13)}},
	} {
		if _, err := bookings.Book(ctx, b.actor, b.spaceID, b.p); err != nil {
			t.Fatalf("Book() error = %v, want nil", err)
		}
	}

	errs := race(racers, func(int) error {
		_, err := members.Delete(ctx, 1)
		return err
	})

	checkCounts(t, errs, 1, cowbook.ErrMemberDeleted)
	if m, _ := store.GetMember(ctx, 1); !m.Deleted() {
		t.Error("member 1 is not deleted")
	}
	left, err := store.ListMemberBookingsEndingAfter(ctx, 1, now)
	if err != nil {
		t.Fatalf("ListMemberBookingsEndingAfter() error = %v, want nil", err)
	}
	if len(left) != 0 {
		t.Errorf("member 1 has %d active bookings, want 0", len(left))
	}
	if _, err := bookings.Book(ctx, cowbook.MemberActor(2), 1, slot(2)); err != nil {
		t.Errorf("Book() of the freed slot error = %v, want nil", err)
	}
}
