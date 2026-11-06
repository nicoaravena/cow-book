package cowbook

import (
	"context"
	"errors"
	"testing"
	"time"
)

// fakeStore stands in for all three stores. It keeps what it is given in maps
// and records every call's context. Set createErr or saveErr to make that call
// fail, the way a real store would on a lost race.
type fakeStore struct {
	spaces    map[int64]Space
	loc       *time.Location
	members   map[int64]Member
	bookings  map[int64]Booking
	createErr error
	saveErr   error
	created   []Booking
	saved     []Booking
	contexts  []context.Context
}

func (f *fakeStore) CreateBooking(ctx context.Context, b Booking) (Booking, error) {
	f.contexts = append(f.contexts, ctx)
	if f.createErr != nil {
		return Booking{}, f.createErr
	}
	b.ID = 100
	f.bookings[b.ID] = b
	f.created = append(f.created, b)
	return b, nil
}

func (f *fakeStore) GetBooking(ctx context.Context, id int64) (Booking, error) {
	f.contexts = append(f.contexts, ctx)
	b, ok := f.bookings[id]
	if !ok {
		return Booking{}, ErrBookingNotFound
	}
	return b, nil
}

func (f *fakeStore) SaveCancellation(ctx context.Context, b Booking) error {
	f.contexts = append(f.contexts, ctx)
	if f.saveErr != nil {
		return f.saveErr
	}
	f.bookings[b.ID] = b
	f.saved = append(f.saved, b)
	return nil
}

func (f *fakeStore) GetSpace(ctx context.Context, id int64) (Space, *time.Location, error) {
	f.contexts = append(f.contexts, ctx)
	s, ok := f.spaces[id]
	if !ok {
		return Space{}, nil, ErrSpaceNotFound
	}
	return s, f.loc, nil
}

func (f *fakeStore) GetMember(ctx context.Context, id int64) (Member, error) {
	f.contexts = append(f.contexts, ctx)
	m, ok := f.members[id]
	if !ok {
		return Member{}, ErrMemberNotFound
	}
	return m, nil
}

// newFakeStore returns a store holding storedSpace, member 3 and
// cancellableBooking, with the location in UTC.
func newFakeStore(t *testing.T) *fakeStore {
	t.Helper()
	m, err := NewMember(validMemberParams(), bookingNow)
	if err != nil {
		t.Fatalf("NewMember() error = %v, want nil", err)
	}
	m.ID = 3

	s := storedSpace(t)
	b := cancellableBooking(t)
	return &fakeStore{
		spaces:   map[int64]Space{s.ID: s},
		loc:      time.UTC,
		members:  map[int64]Member{m.ID: m},
		bookings: map[int64]Booking{b.ID: b},
	}
}

// newTestService returns a service over store whose clock always reads now.
func newTestService(store *fakeStore, now time.Time) *BookingService {
	return NewBookingService(store, store, store, func() time.Time { return now })
}

func TestBookStoresTheBooking(t *testing.T) {
	store := newFakeStore(t)
	svc := newTestService(store, bookingNow)

	b, err := svc.Book(context.Background(), 7, validBookingParams())
	if err != nil {
		t.Fatalf("Book() error = %v, want nil", err)
	}

	if b.ID != 100 {
		t.Errorf("ID = %d, want 100 from the store", b.ID)
	}
	if b.spaceID != 7 {
		t.Errorf("spaceID = %d, want 7", b.spaceID)
	}
	if !b.createdAt.Equal(bookingNow) {
		t.Errorf("createdAt = %v, want the clock's %v", b.createdAt, bookingNow)
	}
	if len(store.created) != 1 {
		t.Errorf("store created %d bookings, want 1", len(store.created))
	}
}

func TestBookUsesTheInjectedClock(t *testing.T) {
	p := validBookingParams()

	tests := []struct {
		name    string
		now     time.Time
		wantErr error
	}{
		{"before the start", p.Start.Add(-time.Minute), nil},
		{"after the start", p.Start.Add(time.Minute), ErrBookingInPast},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := newTestService(newFakeStore(t), tt.now)

			_, err := svc.Book(context.Background(), 7, p)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Book() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestBookUsesTheLocationFromTheStore(t *testing.T) {
	// Monday 15:00 UTC is Monday 10:00 in New York and Tuesday 00:00 in Tokyo.
	p := validBookingParams()
	p.Start = utc(time.November, 9, 15, 0)
	p.End = utc(time.November, 9, 16, 0)

	tests := []struct {
		zone    string
		wantErr error
	}{
		{"America/New_York", nil},
		{"Asia/Tokyo", ErrOutsideOpeningHours},
	}

	for _, tt := range tests {
		t.Run(tt.zone, func(t *testing.T) {
			store := newFakeStore(t)
			store.loc = loadLocation(t, tt.zone)
			svc := newTestService(store, bookingNow)

			_, err := svc.Book(context.Background(), 7, p)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Book() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestBookRefusalsStoreNothing(t *testing.T) {
	tests := []struct {
		name    string
		spaceID int64
		change  func(t *testing.T, store *fakeStore, p *BookingParams)
		wantErr error
	}{
		{
			name:    "unknown space",
			spaceID: 999,
			wantErr: ErrSpaceNotFound,
		},
		{
			name:    "unknown member",
			spaceID: 7,
			change: func(t *testing.T, store *fakeStore, p *BookingParams) {
				p.MemberID = 999
			},
			wantErr: ErrMemberNotFound,
		},
		{
			name:    "deleted member",
			spaceID: 7,
			change: func(t *testing.T, store *fakeStore, p *BookingParams) {
				m := store.members[3]
				if err := m.Delete(bookingNow); err != nil {
					t.Fatalf("Delete() error = %v, want nil", err)
				}
				store.members[3] = m
			},
			wantErr: ErrMemberDeleted,
		},
		{
			name:    "domain rule",
			spaceID: 7,
			change: func(t *testing.T, store *fakeStore, p *BookingParams) {
				p.Attendees = 99
			},
			wantErr: ErrOverCapacity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeStore(t)
			p := validBookingParams()
			if tt.change != nil {
				tt.change(t, store, &p)
			}
			svc := newTestService(store, bookingNow)

			_, err := svc.Book(context.Background(), tt.spaceID, p)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Book() error = %v, want %v", err, tt.wantErr)
			}
			if len(store.created) != 0 {
				t.Errorf("store created %d bookings, want 0", len(store.created))
			}
		})
	}
}

func TestBookReturnsErrSlotTakenFromTheStore(t *testing.T) {
	store := newFakeStore(t)
	store.createErr = ErrSlotTaken
	svc := newTestService(store, bookingNow)

	_, err := svc.Book(context.Background(), 7, validBookingParams())
	if !errors.Is(err, ErrSlotTaken) {
		t.Errorf("Book() error = %v, want %v", err, ErrSlotTaken)
	}
}

func TestServiceCancelSavesTheBooking(t *testing.T) {
	store := newFakeStore(t)
	later := bookingNow.Add(time.Hour)
	svc := newTestService(store, later)

	b, err := svc.Cancel(context.Background(), MemberActor(3), 11)
	if err != nil {
		t.Fatalf("Cancel() error = %v, want nil", err)
	}

	if !b.cancelledAt.Equal(later) {
		t.Errorf("returned cancelledAt = %v, want the clock's %v", b.cancelledAt, later)
	}
	if len(store.saved) != 1 {
		t.Fatalf("store saved %d bookings, want 1", len(store.saved))
	}
	if !store.saved[0].cancelledAt.Equal(later) {
		t.Errorf("saved cancelledAt = %v, want the clock's %v", store.saved[0].cancelledAt, later)
	}
}

func TestServiceCancelRefusalsSaveNothing(t *testing.T) {
	afterTheEnd := validBookingParams().End.Add(time.Minute)

	tests := []struct {
		name      string
		actor     Actor
		bookingID int64
		now       time.Time
		change    func(store *fakeStore)
		wantErr   error
	}{
		{
			name:      "someone else's booking",
			actor:     MemberActor(4),
			bookingID: 11,
			now:       bookingNow,
			wantErr:   ErrCancelNotAllowed,
		},
		{
			name:      "ended booking",
			actor:     MemberActor(3),
			bookingID: 11,
			now:       afterTheEnd,
			wantErr:   ErrBookingEnded,
		},
		{
			name:      "unknown booking",
			actor:     MemberActor(3),
			bookingID: 999,
			now:       bookingNow,
			wantErr:   ErrBookingNotFound,
		},
		{
			name:      "space no longer there",
			actor:     MemberActor(3),
			bookingID: 11,
			now:       bookingNow,
			change:    func(store *fakeStore) { delete(store.spaces, 7) },
			wantErr:   ErrSpaceNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeStore(t)
			if tt.change != nil {
				tt.change(store)
			}
			svc := newTestService(store, tt.now)

			_, err := svc.Cancel(context.Background(), tt.actor, tt.bookingID)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Cancel() error = %v, want %v", err, tt.wantErr)
			}
			if len(store.saved) != 0 {
				t.Errorf("store saved %d bookings, want 0", len(store.saved))
			}
		})
	}
}

// Two cancels can both pass Booking.Cancel. The store decides the winner, and
// the loser must see the store's error.
func TestServiceCancelReturnsLostRaceFromTheStore(t *testing.T) {
	store := newFakeStore(t)
	store.saveErr = ErrAlreadyCancelled
	svc := newTestService(store, bookingNow)

	_, err := svc.Cancel(context.Background(), MemberActor(3), 11)
	if !errors.Is(err, ErrAlreadyCancelled) {
		t.Errorf("Cancel() error = %v, want %v", err, ErrAlreadyCancelled)
	}
}

func TestServicePassesContextToEveryStoreCall(t *testing.T) {
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "from the caller")

	tests := []struct {
		name      string
		call      func(svc *BookingService) error
		wantCalls int
	}{
		{
			name: "Book",
			call: func(svc *BookingService) error {
				_, err := svc.Book(ctx, 7, validBookingParams())
				return err
			},
			wantCalls: 3, // GetSpace, GetMember, CreateBooking
		},
		{
			name: "Cancel",
			call: func(svc *BookingService) error {
				_, err := svc.Cancel(ctx, MemberActor(3), 11)
				return err
			},
			wantCalls: 3, // GetBooking, GetSpace, SaveCancellation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newFakeStore(t)
			svc := newTestService(store, bookingNow)

			if err := tt.call(svc); err != nil {
				t.Fatalf("error = %v, want nil", err)
			}

			if len(store.contexts) != tt.wantCalls {
				t.Fatalf("store got %d calls, want %d", len(store.contexts), tt.wantCalls)
			}
			for i, got := range store.contexts {
				if got.Value(key{}) == nil {
					t.Errorf("store call %d did not get the caller's context", i)
				}
			}
		})
	}
}
