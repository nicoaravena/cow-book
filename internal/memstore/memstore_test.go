package memstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"nicoaravena/cow-book-app/cowbook"
	"nicoaravena/cow-book-app/internal/memstore"
)

// now is the clock used by these tests: Monday 2 November 2026, 09:00 UTC.
var now = time.Date(2026, time.November, 2, 9, 0, 0, 0, time.UTC)

// at returns hour:00 UTC on Monday 9 November 2026, the day every test
// booking is on.
func at(hour int) time.Time {
	return time.Date(2026, time.November, 9, hour, 0, 0, 0, time.UTC)
}

// newSpace returns a space at location 1, stored under id: capacity 8,
// bookings up to 4 hours, open every day 08:00 to 20:00.
func newSpace(t *testing.T, id int64) cowbook.Space {
	t.Helper()
	open, err := cowbook.NewClockTime(8, 0)
	if err != nil {
		t.Fatalf("NewClockTime() error = %v, want nil", err)
	}
	closing, err := cowbook.NewClockTime(20, 0)
	if err != nil {
		t.Fatalf("NewClockTime() error = %v, want nil", err)
	}
	var windows []cowbook.OpeningWindow
	for day := time.Sunday; day <= time.Saturday; day++ {
		windows = append(windows, cowbook.OpeningWindow{Day: day, Open: open, Close: closing})
	}
	hours, err := cowbook.NewWeeklyHours(windows)
	if err != nil {
		t.Fatalf("NewWeeklyHours() error = %v, want nil", err)
	}

	s, err := cowbook.NewSpace(cowbook.SpaceParams{
		LocationID:       1,
		Name:             "Meeting room",
		Kind:             cowbook.KindMeetingRoom,
		Capacity:         8,
		MaxBookingLength: 4 * time.Hour,
		Hours:            hours,
	}, now)
	if err != nil {
		t.Fatalf("NewSpace() error = %v, want nil", err)
	}
	s.ID = id
	return s
}

// newStore returns a store holding spaces 7 and 8, both in UTC, and members 1
// to members.
func newStore(t *testing.T, members int) *memstore.Store {
	t.Helper()
	store := memstore.New()
	store.AddSpace(newSpace(t, 7), time.UTC)
	store.AddSpace(newSpace(t, 8), time.UTC)

	for id := int64(1); id <= int64(members); id++ {
		m, err := cowbook.NewMember(cowbook.MemberParams{Name: "Member", Email: "member@example.com"}, now)
		if err != nil {
			t.Fatalf("NewMember() error = %v, want nil", err)
		}
		m.ID = id
		store.AddMember(m)
	}
	return store
}

// newBooking returns a booking by actor for memberID of space 7 or 8, from
// startHour to endHour on Monday 9 November, not yet stored.
func newBooking(t *testing.T, actor cowbook.Actor, spaceID, memberID int64, startHour, endHour int) cowbook.Booking {
	t.Helper()
	b, err := cowbook.NewBooking(actor, newSpace(t, spaceID), time.UTC, cowbook.BookingParams{
		MemberID:  memberID,
		Attendees: 1,
		Start:     at(startHour),
		End:       at(endHour),
	}, now)
	if err != nil {
		t.Fatalf("NewBooking() error = %v, want nil", err)
	}
	return b
}

// create stores b and fails the test if the store refuses it.
func create(t *testing.T, store *memstore.Store, b cowbook.Booking) cowbook.Booking {
	t.Helper()
	b, err := store.CreateBooking(context.Background(), b)
	if err != nil {
		t.Fatalf("CreateBooking() error = %v, want nil", err)
	}
	return b
}

func TestCreateBookingAssignsIDs(t *testing.T) {
	store := newStore(t, 1)
	ctx := context.Background()

	first := create(t, store, newBooking(t, cowbook.MemberActor(1), 7, 1, 10, 11))
	second := create(t, store, newBooking(t, cowbook.MemberActor(1), 7, 1, 12, 13))

	if first.ID != 1 || second.ID != 2 {
		t.Errorf("IDs = %d, %d, want 1, 2", first.ID, second.ID)
	}
	got, err := store.GetBooking(ctx, second.ID)
	if err != nil {
		t.Fatalf("GetBooking() error = %v, want nil", err)
	}
	if got != second {
		t.Errorf("GetBooking() = %+v, want %+v", got, second)
	}
}

func TestCreateBookingRefusesOverlaps(t *testing.T) {
	tests := []struct {
		name      string
		spaceID   int64
		startHour int
		endHour   int
		wantErr   error
	}{
		{"same time", 7, 10, 12, cowbook.ErrSlotTaken},
		{"starts inside", 7, 11, 13, cowbook.ErrSlotTaken},
		{"ends inside", 7, 9, 11, cowbook.ErrSlotTaken},
		{"covers it", 7, 9, 13, cowbook.ErrSlotTaken},
		{"inside it", 7, 10, 11, cowbook.ErrSlotTaken},
		{"starts at its end", 7, 12, 13, nil},
		{"ends at its start", 7, 8, 10, nil},
		{"other space", 8, 10, 12, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newStore(t, 2)
			create(t, store, newBooking(t, cowbook.MemberActor(1), 7, 1, 10, 12))

			_, err := store.CreateBooking(context.Background(), newBooking(t, cowbook.MemberActor(2), tt.spaceID, 2, tt.startHour, tt.endHour))
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("CreateBooking() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestCancelledBookingFreesItsSlot(t *testing.T) {
	store := newStore(t, 2)
	b := create(t, store, newBooking(t, cowbook.MemberActor(1), 7, 1, 10, 12))
	if err := b.Cancel(cowbook.MemberActor(1), newSpace(t, 7), now); err != nil {
		t.Fatalf("Cancel() error = %v, want nil", err)
	}
	if err := store.SaveCancellation(context.Background(), b); err != nil {
		t.Fatalf("SaveCancellation() error = %v, want nil", err)
	}

	create(t, store, newBooking(t, cowbook.MemberActor(2), 7, 2, 10, 12))
}

func TestPendingBookingHoldsItsSlot(t *testing.T) {
	store := newStore(t, 2)
	pending := create(t, store, newBooking(t, cowbook.AdminActor(9), 7, 1, 10, 12))
	if pending.Approved() {
		t.Fatal("staff booking is approved, want pending")
	}

	_, err := store.CreateBooking(context.Background(), newBooking(t, cowbook.MemberActor(2), 7, 2, 10, 12))
	if !errors.Is(err, cowbook.ErrSlotTaken) {
		t.Errorf("CreateBooking() error = %v, want %v", err, cowbook.ErrSlotTaken)
	}
}

// Two requests can both read a booking, both pass the domain rule and both
// try to save. The store must let only the first through.
func TestSaveCancellationLoser(t *testing.T) {
	store := newStore(t, 1)
	stored := create(t, store, newBooking(t, cowbook.MemberActor(1), 7, 1, 10, 12))

	first, second := stored, stored
	for _, b := range []*cowbook.Booking{&first, &second} {
		if err := b.Cancel(cowbook.MemberActor(1), newSpace(t, 7), now); err != nil {
			t.Fatalf("Cancel() error = %v, want nil", err)
		}
	}

	ctx := context.Background()
	if err := store.SaveCancellation(ctx, first); err != nil {
		t.Fatalf("first SaveCancellation() error = %v, want nil", err)
	}
	if err := store.SaveCancellation(ctx, second); !errors.Is(err, cowbook.ErrAlreadyCancelled) {
		t.Errorf("second SaveCancellation() error = %v, want %v", err, cowbook.ErrAlreadyCancelled)
	}
}

func TestSaveApprovalLosers(t *testing.T) {
	tests := []struct {
		name    string
		before  func(b *cowbook.Booking) error // the winner's change, saved first
		save    func(store *memstore.Store, b cowbook.Booking) error
		wantErr error
	}{
		{
			name:   "approved twice",
			before: func(b *cowbook.Booking) error { return b.Approve(cowbook.MemberActor(1), now) },
			save: func(store *memstore.Store, b cowbook.Booking) error {
				return store.SaveApproval(context.Background(), b)
			},
			wantErr: cowbook.ErrNotPending,
		},
		{
			name: "cancelled first",
			before: func(b *cowbook.Booking) error {
				return b.Cancel(cowbook.MemberActor(1), newSpace(t, 7), now)
			},
			save: func(store *memstore.Store, b cowbook.Booking) error {
				return store.SaveCancellation(context.Background(), b)
			},
			wantErr: cowbook.ErrAlreadyCancelled,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newStore(t, 1)
			stored := create(t, store, newBooking(t, cowbook.AdminActor(9), 7, 1, 10, 12))

			winner, loser := stored, stored
			if err := tt.before(&winner); err != nil {
				t.Fatalf("winner's change error = %v, want nil", err)
			}
			if err := tt.save(store, winner); err != nil {
				t.Fatalf("winner's save error = %v, want nil", err)
			}

			if err := loser.Approve(cowbook.MemberActor(1), now); err != nil {
				t.Fatalf("Approve() error = %v, want nil", err)
			}
			if err := store.SaveApproval(context.Background(), loser); !errors.Is(err, tt.wantErr) {
				t.Errorf("SaveApproval() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNotFound(t *testing.T) {
	store := newStore(t, 1)
	ctx := context.Background()
	unknown := newBooking(t, cowbook.MemberActor(1), 7, 1, 10, 12)
	unknown.ID = 999
	ghost, err := cowbook.NewMember(cowbook.MemberParams{Name: "Ghost", Email: "ghost@example.com"}, now)
	if err != nil {
		t.Fatalf("NewMember() error = %v, want nil", err)
	}
	ghost.ID = 999

	tests := []struct {
		name    string
		call    func() error
		wantErr error
	}{
		{"GetBooking", func() error { _, err := store.GetBooking(ctx, 999); return err }, cowbook.ErrBookingNotFound},
		{"SaveCancellation", func() error { return store.SaveCancellation(ctx, unknown) }, cowbook.ErrBookingNotFound},
		{"SaveApproval", func() error { return store.SaveApproval(ctx, unknown) }, cowbook.ErrBookingNotFound},
		{"GetSpace", func() error { _, _, err := store.GetSpace(ctx, 999); return err }, cowbook.ErrSpaceNotFound},
		{"GetMember", func() error { _, err := store.GetMember(ctx, 999); return err }, cowbook.ErrMemberNotFound},
		{"SaveDeletion", func() error { return store.SaveDeletion(ctx, ghost, nil) }, cowbook.ErrMemberNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestListMemberBookingsEndingAfter(t *testing.T) {
	store := newStore(t, 2)
	create(t, store, newBooking(t, cowbook.MemberActor(1), 7, 1, 10, 11)) // ends at t
	want := create(t, store, newBooking(t, cowbook.MemberActor(1), 7, 1, 12, 13))
	create(t, store, newBooking(t, cowbook.MemberActor(2), 7, 2, 14, 15)) // another member

	cancelled := create(t, store, newBooking(t, cowbook.MemberActor(1), 8, 1, 14, 15))
	if err := cancelled.Cancel(cowbook.MemberActor(1), newSpace(t, 8), now); err != nil {
		t.Fatalf("Cancel() error = %v, want nil", err)
	}
	if err := store.SaveCancellation(context.Background(), cancelled); err != nil {
		t.Fatalf("SaveCancellation() error = %v, want nil", err)
	}

	got, err := store.ListMemberBookingsEndingAfter(context.Background(), 1, at(11))
	if err != nil {
		t.Fatalf("ListMemberBookingsEndingAfter() error = %v, want nil", err)
	}
	if len(got) != 1 || got[0] != want {
		t.Errorf("ListMemberBookingsEndingAfter() = %+v, want only booking %d", got, want.ID)
	}
}

func TestSaveDeletion(t *testing.T) {
	store := newStore(t, 1)
	ctx := context.Background()
	stored := create(t, store, newBooking(t, cowbook.MemberActor(1), 7, 1, 10, 12))

	// The member cancels the booking after the deletion read it, so the
	// deletion's copy is stale and must not replace the member's cancellation.
	byMember := stored
	if err := byMember.Cancel(cowbook.MemberActor(1), newSpace(t, 7), now); err != nil {
		t.Fatalf("Cancel() error = %v, want nil", err)
	}
	if err := store.SaveCancellation(ctx, byMember); err != nil {
		t.Fatalf("SaveCancellation() error = %v, want nil", err)
	}
	byDeletion := stored
	if err := byDeletion.Cancel(cowbook.AdminActor(9), newSpace(t, 7), now.Add(time.Hour)); err != nil {
		t.Fatalf("Cancel() error = %v, want nil", err)
	}

	m, err := store.GetMember(ctx, 1)
	if err != nil {
		t.Fatalf("GetMember() error = %v, want nil", err)
	}
	first, second := m, m
	if err := first.Delete(now); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}
	if err := second.Delete(now); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}

	if err := store.SaveDeletion(ctx, first, []cowbook.Booking{byDeletion}); err != nil {
		t.Fatalf("SaveDeletion() error = %v, want nil", err)
	}
	if err := store.SaveDeletion(ctx, second, nil); !errors.Is(err, cowbook.ErrMemberDeleted) {
		t.Errorf("second SaveDeletion() error = %v, want %v", err, cowbook.ErrMemberDeleted)
	}

	if got, _ := store.GetMember(ctx, 1); !got.Deleted() {
		t.Error("stored member is not deleted")
	}
	if got, _ := store.GetBooking(ctx, stored.ID); got != byMember {
		t.Errorf("stored booking = %+v, want the member's cancellation %+v", got, byMember)
	}
}

func TestDoneContextIsRefused(t *testing.T) {
	store := newStore(t, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b := newBooking(t, cowbook.MemberActor(1), 7, 1, 10, 12)
	m, _ := store.GetMember(context.Background(), 1)

	tests := []struct {
		name string
		call func() error
	}{
		{"CreateBooking", func() error { _, err := store.CreateBooking(ctx, b); return err }},
		{"GetBooking", func() error { _, err := store.GetBooking(ctx, 1); return err }},
		{"SaveCancellation", func() error { return store.SaveCancellation(ctx, b) }},
		{"SaveApproval", func() error { return store.SaveApproval(ctx, b) }},
		{"ListMemberBookingsEndingAfter", func() error { _, err := store.ListMemberBookingsEndingAfter(ctx, 1, now); return err }},
		{"GetSpace", func() error { _, _, err := store.GetSpace(ctx, 7); return err }},
		{"GetMember", func() error { _, err := store.GetMember(ctx, 1); return err }},
		{"SaveDeletion", func() error { return store.SaveDeletion(ctx, m, nil) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); !errors.Is(err, context.Canceled) {
				t.Errorf("error = %v, want %v", err, context.Canceled)
			}
		})
	}
}
