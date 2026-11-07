package cowbook

import (
	"context"
	"errors"
	"testing"
	"time"
)

// newMemberTestService returns a service over store whose clock always reads
// now.
func newMemberTestService(store *fakeStore, now time.Time) *MemberService {
	return NewMemberService(store, store, func() time.Time { return now })
}

func TestMemberServiceDeleteCancelsBookings(t *testing.T) {
	store := newFakeStore(t)
	pending := pendingBooking(t)
	pending.ID = 12
	store.bookings[12] = pending
	ended := cancellableBooking(t)
	ended.ID = 13
	ended.end = bookingNow
	store.bookings[13] = ended
	other := cancellableBooking(t)
	other.ID = 14
	other.memberID = 4
	store.bookings[14] = other

	later := bookingNow.Add(time.Hour)
	svc := newMemberTestService(store, later)

	m, err := svc.Delete(context.Background(), 3)
	if err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}

	if !m.deletedAt.Equal(later) {
		t.Errorf("returned deletedAt = %v, want the clock's %v", m.deletedAt, later)
	}
	if len(store.deleted) != 1 || !store.deleted[0].deletedAt.Equal(later) {
		t.Fatalf("store deleted %+v, want member 3 deleted at %v", store.deleted, later)
	}
	if got := len(store.deletedFor[0]); got != 2 {
		t.Errorf("SaveDeletion got %d cancelled bookings, want 2", got)
	}
	for _, id := range []int64{11, 12} {
		if b := store.bookings[id]; !b.cancelledAt.Equal(later) {
			t.Errorf("booking %d cancelledAt = %v, want the clock's %v", id, b.cancelledAt, later)
		}
	}
	for _, id := range []int64{13, 14} {
		if b := store.bookings[id]; !b.cancelledAt.IsZero() {
			t.Errorf("booking %d cancelledAt = %v, want zero", id, b.cancelledAt)
		}
	}
}

func TestMemberServiceDeleteWithNoBookings(t *testing.T) {
	store := newFakeStore(t)
	delete(store.bookings, 11)
	svc := newMemberTestService(store, bookingNow)

	if _, err := svc.Delete(context.Background(), 3); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}
	if len(store.deleted) != 1 {
		t.Errorf("store deleted %d members, want 1", len(store.deleted))
	}
}

func TestMemberServiceDeleteRefusalsSaveNothing(t *testing.T) {
	tests := []struct {
		name     string
		memberID int64
		change   func(t *testing.T, store *fakeStore)
		wantErr  error
	}{
		{"unknown member", 999, nil, ErrMemberNotFound},
		{"already deleted", 3, deleteMember3, ErrMemberDeleted},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeStore(t)
			if tt.change != nil {
				tt.change(t, store)
			}
			svc := newMemberTestService(store, bookingNow)

			_, err := svc.Delete(context.Background(), tt.memberID)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Delete() error = %v, want %v", err, tt.wantErr)
			}
			if len(store.deleted) != 0 {
				t.Errorf("store deleted %d members, want 0", len(store.deleted))
			}
		})
	}
}

// Two deletions can both pass Member.Delete. The store decides the winner, and
// the loser must see the store's error.
func TestMemberServiceDeleteReturnsLostRaceFromTheStore(t *testing.T) {
	store := newFakeStore(t)
	store.deleteErr = ErrMemberDeleted
	svc := newMemberTestService(store, bookingNow)

	_, err := svc.Delete(context.Background(), 3)
	if !errors.Is(err, ErrMemberDeleted) {
		t.Errorf("Delete() error = %v, want %v", err, ErrMemberDeleted)
	}
}

func TestMemberServicePassesContextToEveryStoreCall(t *testing.T) {
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "from the caller")
	store := newFakeStore(t)
	svc := newMemberTestService(store, bookingNow)

	if _, err := svc.Delete(ctx, 3); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}

	// GetMember, ListMemberBookingsEndingAfter, SaveDeletion
	if len(store.contexts) != 3 {
		t.Fatalf("store got %d calls, want 3", len(store.contexts))
	}
	for i, got := range store.contexts {
		if got.Value(key{}) == nil {
			t.Errorf("store call %d did not get the caller's context", i)
		}
	}
}
