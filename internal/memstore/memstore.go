// Package memstore keeps spaces, members and bookings in memory. It implements
// the cowbook store interfaces for tests and local runs, and is safe for use by
// many goroutines at once.
package memstore

import (
	"context"
	"sync"
	"time"

	"nicoaravena/cow-book-app/cowbook"
)

// The compiler checks that *Store implements the three interfaces, so a
// missing or mistyped method fails the build here and not at some caller.
var (
	_ cowbook.BookingStore = (*Store)(nil)
	_ cowbook.SpaceStore   = (*Store)(nil)
	_ cowbook.MemberStore  = (*Store)(nil)
)

// storedSpace is a space with the timezone of its location.
type storedSpace struct {
	space cowbook.Space
	loc   *time.Location
}

// Store is an in-memory store. Every method holds mu for its whole run, so
// each read, check and write happens as one step and two calls can't both see
// the same slot free. The maps hold values, so a booking or member handed to a
// caller is a copy and changing it leaves the store as it was.
//
// Saves replace the whole booking, where Postgres will update only the
// columns that changed. So if a cancel read a booking before an approval of
// it was saved, saving the cancel drops the approval time. The booking ends
// cancelled either way, which is what matters for the slot.
//
// A Store must not be copied after first use, because copying a Mutex copies
// its locked state. go vet reports such copies.
type Store struct {
	mu       sync.Mutex
	spaces   map[int64]storedSpace
	members  map[int64]cowbook.Member
	bookings map[int64]cowbook.Booking
	lastID   int64 // the last booking ID handed out
}

// New returns an empty Store.
func New() *Store {
	return &Store{
		spaces:   make(map[int64]storedSpace),
		members:  make(map[int64]cowbook.Member),
		bookings: make(map[int64]cowbook.Booking),
	}
}

// AddSpace stores sp under sp.ID, with loc as its location's timezone. Spaces
// have no service yet, so the caller picks the ID.
func (s *Store) AddSpace(sp cowbook.Space, loc *time.Location) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spaces[sp.ID] = storedSpace{space: sp, loc: loc}
}

// AddMember stores m under m.ID. Members have no create service yet, so the
// caller picks the ID.
func (s *Store) AddMember(m cowbook.Member) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.members[m.ID] = m
}

// overlaps reports whether a and b share any moment. A booking does not
// include its end, so one ending at 10:00 and one starting at 10:00 don't
// overlap, the same as a Postgres tstzrange.
func overlaps(a, b cowbook.Booking) bool {
	return a.Start().Before(b.End()) && b.Start().Before(a.End())
}

// CreateBooking stores b with the next ID. A booking that isn't cancelled
// holds its slot, pending ones included.
func (s *Store) CreateBooking(ctx context.Context, b cowbook.Booking) (cowbook.Booking, error) {
	if err := ctx.Err(); err != nil {
		return cowbook.Booking{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, other := range s.bookings {
		if other.SpaceID() == b.SpaceID() && !other.Cancelled() && overlaps(b, other) {
			return cowbook.Booking{}, cowbook.ErrSlotTaken
		}
	}

	s.lastID++
	b.ID = s.lastID
	s.bookings[b.ID] = b
	return b, nil
}

func (s *Store) GetBooking(ctx context.Context, id int64) (cowbook.Booking, error) {
	if err := ctx.Err(); err != nil {
		return cowbook.Booking{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	b, ok := s.bookings[id]
	if !ok {
		return cowbook.Booking{}, cowbook.ErrBookingNotFound
	}
	return b, nil
}

// SaveCancellation stores b unless the stored booking is already cancelled:
// of two cancels that both passed Booking.Cancel, the first one here wins.
func (s *Store) SaveCancellation(ctx context.Context, b cowbook.Booking) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	stored, ok := s.bookings[b.ID]
	switch {
	case !ok:
		return cowbook.ErrBookingNotFound
	case stored.Cancelled():
		return cowbook.ErrAlreadyCancelled
	}
	s.bookings[b.ID] = b
	return nil
}

// SaveApproval stores b unless the stored booking was cancelled or approved
// since b was read.
func (s *Store) SaveApproval(ctx context.Context, b cowbook.Booking) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	stored, ok := s.bookings[b.ID]
	switch {
	case !ok:
		return cowbook.ErrBookingNotFound
	case stored.Cancelled():
		return cowbook.ErrAlreadyCancelled
	case stored.Approved():
		return cowbook.ErrNotPending
	}
	s.bookings[b.ID] = b
	return nil
}

func (s *Store) ListMemberBookingsEndingAfter(ctx context.Context, memberID int64, t time.Time) ([]cowbook.Booking, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	var bs []cowbook.Booking
	for _, b := range s.bookings {
		if b.MemberID() == memberID && !b.Cancelled() && b.End().After(t) {
			bs = append(bs, b)
		}
	}
	return bs, nil
}

func (s *Store) GetSpace(ctx context.Context, id int64) (cowbook.Space, *time.Location, error) {
	if err := ctx.Err(); err != nil {
		return cowbook.Space{}, nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	sp, ok := s.spaces[id]
	if !ok {
		return cowbook.Space{}, nil, cowbook.ErrSpaceNotFound
	}
	return sp.space, sp.loc, nil
}

func (s *Store) GetMember(ctx context.Context, id int64) (cowbook.Member, error) {
	if err := ctx.Err(); err != nil {
		return cowbook.Member{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	m, ok := s.members[id]
	if !ok {
		return cowbook.Member{}, cowbook.ErrMemberNotFound
	}
	return m, nil
}

// SaveDeletion stores m and the cancelled bookings under one lock, so no
// other call sees the member deleted with their bookings still active. A
// booking cancelled by someone else since it was read keeps that cancellation.
func (s *Store) SaveDeletion(ctx context.Context, m cowbook.Member, cancelled []cowbook.Booking) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	stored, ok := s.members[m.ID]
	switch {
	case !ok:
		return cowbook.ErrMemberNotFound
	case stored.Deleted():
		return cowbook.ErrMemberDeleted
	}

	s.members[m.ID] = m
	for _, b := range cancelled {
		if !s.bookings[b.ID].Cancelled() {
			s.bookings[b.ID] = b
		}
	}
	return nil
}
