package memstore_test

import (
	"context"
	"errors"
	"fmt"
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

// newSpace returns a space at location 1 with its ID set to id: capacity 8,
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

// newMember returns a member with email, not yet stored.
func newMember(t *testing.T, email string) cowbook.Member {
	t.Helper()
	m, err := cowbook.NewMember(cowbook.MemberParams{Name: "Member", Email: email}, now)
	if err != nil {
		t.Fatalf("NewMember() error = %v, want nil", err)
	}
	return m
}

// newStore returns a store holding location 1 in UTC, spaces 1 and 2 there,
// and members 1 to members. The store hands out the IDs, in that order.
func newStore(t *testing.T, members int) *memstore.Store {
	t.Helper()
	ctx := context.Background()
	store := memstore.New()
	if _, err := store.LocationFor(ctx, time.UTC); err != nil {
		t.Fatalf("LocationFor() error = %v, want nil", err)
	}
	for range 2 {
		if _, err := store.CreateSpace(ctx, newSpace(t, 0)); err != nil {
			t.Fatalf("CreateSpace() error = %v, want nil", err)
		}
	}
	for i := range members {
		m := newMember(t, fmt.Sprintf("member%d@example.com", i+1))
		if _, err := store.CreateMember(ctx, m); err != nil {
			t.Fatalf("CreateMember() error = %v, want nil", err)
		}
	}
	return store
}

// newBooking returns a booking by actor for memberID of space 1 or 2, from
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

	first := create(t, store, newBooking(t, cowbook.MemberActor(1), 1, 1, 10, 11))
	second := create(t, store, newBooking(t, cowbook.MemberActor(1), 1, 1, 12, 13))

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
		{"same time", 1, 10, 12, cowbook.ErrSlotTaken},
		{"starts inside", 1, 11, 13, cowbook.ErrSlotTaken},
		{"ends inside", 1, 9, 11, cowbook.ErrSlotTaken},
		{"covers it", 1, 9, 13, cowbook.ErrSlotTaken},
		{"inside it", 1, 10, 11, cowbook.ErrSlotTaken},
		{"starts at its end", 1, 12, 13, nil},
		{"ends at its start", 1, 8, 10, nil},
		{"other space", 2, 10, 12, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newStore(t, 2)
			create(t, store, newBooking(t, cowbook.MemberActor(1), 1, 1, 10, 12))

			_, err := store.CreateBooking(context.Background(), newBooking(t, cowbook.MemberActor(2), tt.spaceID, 2, tt.startHour, tt.endHour))
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("CreateBooking() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestCancelledBookingFreesItsSlot(t *testing.T) {
	store := newStore(t, 2)
	b := create(t, store, newBooking(t, cowbook.MemberActor(1), 1, 1, 10, 12))
	if err := b.Cancel(cowbook.MemberActor(1), newSpace(t, 1), now); err != nil {
		t.Fatalf("Cancel() error = %v, want nil", err)
	}
	if err := store.SaveCancellation(context.Background(), b); err != nil {
		t.Fatalf("SaveCancellation() error = %v, want nil", err)
	}

	create(t, store, newBooking(t, cowbook.MemberActor(2), 1, 2, 10, 12))
}

func TestPendingBookingHoldsItsSlot(t *testing.T) {
	store := newStore(t, 2)
	pending := create(t, store, newBooking(t, cowbook.AdminActor(9), 1, 1, 10, 12))
	if pending.Approved() {
		t.Fatal("staff booking is approved, want pending")
	}

	_, err := store.CreateBooking(context.Background(), newBooking(t, cowbook.MemberActor(2), 1, 2, 10, 12))
	if !errors.Is(err, cowbook.ErrSlotTaken) {
		t.Errorf("CreateBooking() error = %v, want %v", err, cowbook.ErrSlotTaken)
	}
}

// Two requests can both read a booking, both pass the domain rule and both
// try to save. The store must let only the first through.
func TestSaveCancellationLoser(t *testing.T) {
	store := newStore(t, 1)
	stored := create(t, store, newBooking(t, cowbook.MemberActor(1), 1, 1, 10, 12))

	first, second := stored, stored
	for _, b := range []*cowbook.Booking{&first, &second} {
		if err := b.Cancel(cowbook.MemberActor(1), newSpace(t, 1), now); err != nil {
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
				return b.Cancel(cowbook.MemberActor(1), newSpace(t, 1), now)
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
			stored := create(t, store, newBooking(t, cowbook.AdminActor(9), 1, 1, 10, 12))

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
	unknown := newBooking(t, cowbook.MemberActor(1), 1, 1, 10, 12)
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
	create(t, store, newBooking(t, cowbook.MemberActor(1), 1, 1, 10, 11)) // ends at t
	want := create(t, store, newBooking(t, cowbook.MemberActor(1), 1, 1, 12, 13))
	create(t, store, newBooking(t, cowbook.MemberActor(2), 1, 2, 14, 15)) // another member

	cancelled := create(t, store, newBooking(t, cowbook.MemberActor(1), 2, 1, 14, 15))
	if err := cancelled.Cancel(cowbook.MemberActor(1), newSpace(t, 2), now); err != nil {
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
	stored := create(t, store, newBooking(t, cowbook.MemberActor(1), 1, 1, 10, 12))

	// The member cancels the booking after the deletion read it, so the
	// deletion's copy is stale and must not replace the member's cancellation.
	byMember := stored
	if err := byMember.Cancel(cowbook.MemberActor(1), newSpace(t, 1), now); err != nil {
		t.Fatalf("Cancel() error = %v, want nil", err)
	}
	if err := store.SaveCancellation(ctx, byMember); err != nil {
		t.Fatalf("SaveCancellation() error = %v, want nil", err)
	}
	byDeletion := stored
	if err := byDeletion.Cancel(cowbook.AdminActor(9), newSpace(t, 1), now.Add(time.Hour)); err != nil {
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

// The lists hold everything stored, cancelled bookings and deleted members
// included, sorted by ID whatever order the map gives.
func TestLists(t *testing.T) {
	store := newStore(t, 3)
	ctx := context.Background()
	for _, hour := range []int{14, 10, 12} {
		create(t, store, newBooking(t, cowbook.MemberActor(1), 2, 1, hour, hour+1))
	}
	cancelled, _ := store.GetBooking(ctx, 2)
	if err := cancelled.Cancel(cowbook.MemberActor(1), newSpace(t, 2), now); err != nil {
		t.Fatalf("Cancel() error = %v, want nil", err)
	}
	if err := store.SaveCancellation(ctx, cancelled); err != nil {
		t.Fatalf("SaveCancellation() error = %v, want nil", err)
	}
	deleted, _ := store.GetMember(ctx, 2)
	if err := deleted.Delete(now); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}
	if err := store.SaveDeletion(ctx, deleted, nil); err != nil {
		t.Fatalf("SaveDeletion() error = %v, want nil", err)
	}

	spaces, err := store.ListSpaces(ctx)
	if err != nil || len(spaces) != 2 || spaces[0].ID != 1 || spaces[1].ID != 2 {
		t.Errorf("ListSpaces() = %+v, %v, want spaces 1, 2", spaces, err)
	}
	members, err := store.ListMembers(ctx)
	if err != nil || len(members) != 3 || members[0].ID != 1 || members[2].ID != 3 || !members[1].Deleted() {
		t.Errorf("ListMembers() = %+v, %v, want members 1-3 with 2 deleted", members, err)
	}
	bookings, err := store.ListBookings(ctx)
	if err != nil || len(bookings) != 3 || bookings[0].ID != 1 || bookings[2].ID != 3 || !bookings[1].Cancelled() {
		t.Errorf("ListBookings() = %+v, %v, want bookings 1-3 with 2 cancelled", bookings, err)
	}
}

func TestDoneContextIsRefused(t *testing.T) {
	store := newStore(t, 1)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b := newBooking(t, cowbook.MemberActor(1), 1, 1, 10, 12)
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
		{"GetSpace", func() error { _, _, err := store.GetSpace(ctx, 1); return err }},
		{"GetMember", func() error { _, err := store.GetMember(ctx, 1); return err }},
		{"SaveDeletion", func() error { return store.SaveDeletion(ctx, m, nil) }},
		{"ListSpaces", func() error { _, err := store.ListSpaces(ctx); return err }},
		{"ListMembers", func() error { _, err := store.ListMembers(ctx); return err }},
		{"ListBookings", func() error { _, err := store.ListBookings(ctx); return err }},
		{"LocationFor", func() error { _, err := store.LocationFor(ctx, time.UTC); return err }},
		{"CreateSpace", func() error { _, err := store.CreateSpace(ctx, newSpace(t, 0)); return err }},
		{"CreateMember", func() error { _, err := store.CreateMember(ctx, newMember(t, "new@example.com")); return err }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.call(); !errors.Is(err, context.Canceled) {
				t.Errorf("error = %v, want %v", err, context.Canceled)
			}
		})
	}
}

// LocationFor gives each timezone one location: the same zone again gets the
// same ID, a new zone the next one.
func TestLocationFor(t *testing.T) {
	store := memstore.New()
	ctx := context.Background()
	madrid, err := time.LoadLocation("Europe/Madrid")
	if err != nil {
		t.Fatalf("LoadLocation() error = %v, want nil", err)
	}
	// A second *time.Location for the same zone, to show zones compare by name.
	madridAgain, err := time.LoadLocation("Europe/Madrid")
	if err != nil {
		t.Fatalf("LoadLocation() error = %v, want nil", err)
	}

	for _, tt := range []struct {
		tz   *time.Location
		want int64
	}{{time.UTC, 1}, {madrid, 2}, {time.UTC, 1}, {madridAgain, 2}} {
		if got, err := store.LocationFor(ctx, tt.tz); got != tt.want || err != nil {
			t.Errorf("LocationFor(%s) = %d, %v, want %d, nil", tt.tz, got, err, tt.want)
		}
	}
}

// A space gets its ID from the store, and the timezone of its location.
func TestCreateSpace(t *testing.T) {
	store := memstore.New()
	ctx := context.Background()
	madrid, err := time.LoadLocation("Europe/Madrid")
	if err != nil {
		t.Fatalf("LoadLocation() error = %v, want nil", err)
	}
	if _, err := store.LocationFor(ctx, madrid); err != nil {
		t.Fatalf("LocationFor() error = %v, want nil", err)
	}

	for _, want := range []int64{1, 2} {
		sp, err := store.CreateSpace(ctx, newSpace(t, 0))
		if err != nil || sp.ID != want {
			t.Fatalf("CreateSpace() = space %d, %v, want space %d, nil", sp.ID, err, want)
		}
		if _, loc, err := store.GetSpace(ctx, sp.ID); loc != madrid || err != nil {
			t.Errorf("GetSpace(%d) timezone = %v, %v, want %v, nil", sp.ID, loc, err, madrid)
		}
	}

	// newSpace is at location 1; a new store has none.
	if _, err := memstore.New().CreateSpace(ctx, newSpace(t, 0)); !errors.Is(err, cowbook.ErrInvalidLocation) {
		t.Errorf("unknown location: CreateSpace() error = %v, want %v", err, cowbook.ErrInvalidLocation)
	}
}

// Emails are unique among members who aren't deleted, so a deleted member's
// email can register again.
func TestCreateMember(t *testing.T) {
	store := newStore(t, 1)
	ctx := context.Background()

	second, err := store.CreateMember(ctx, newMember(t, "ana@example.com"))
	if err != nil || second.ID != 2 {
		t.Fatalf("CreateMember() = member %d, %v, want member 2, nil", second.ID, err)
	}
	if _, err := store.CreateMember(ctx, newMember(t, "ana@example.com")); !errors.Is(err, cowbook.ErrEmailTaken) {
		t.Errorf("taken email: CreateMember() error = %v, want %v", err, cowbook.ErrEmailTaken)
	}

	if err := second.Delete(now); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}
	if err := store.SaveDeletion(ctx, second, nil); err != nil {
		t.Fatalf("SaveDeletion() error = %v, want nil", err)
	}
	if again, err := store.CreateMember(ctx, newMember(t, "ana@example.com")); err != nil || again.ID != 3 {
		t.Errorf("deleted member's email: CreateMember() = member %d, %v, want member 3, nil", again.ID, err)
	}
}
