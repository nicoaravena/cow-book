package cowbook

import (
	"context"
	"errors"
	"time"
)

// Errors returned by the stores and passed on by BookingService. Callers
// compare them with errors.Is.
var (
	ErrSlotTaken       = errors.New("space is already booked for that time")
	ErrBookingNotFound = errors.New("booking not found")
	ErrSpaceNotFound   = errors.New("space not found")
	ErrMemberNotFound  = errors.New("member not found")
)

// BookingStore keeps bookings. Each store method takes the caller's context, so
// a request that goes away also stops its database work.
type BookingStore interface {
	// CreateBooking stores b and returns it with its ID set. It returns
	// ErrSlotTaken if b overlaps an active booking of the same space.
	CreateBooking(ctx context.Context, b Booking) (Booking, error)
	// GetBooking returns ErrBookingNotFound if there is no such booking.
	GetBooking(ctx context.Context, id int64) (Booking, error)
	// SaveCancellation stores a booking that Cancel has changed. Two requests
	// can both pass Cancel, so the store decides the winner: the loser gets
	// ErrAlreadyCancelled. It returns ErrBookingNotFound if there is no such
	// booking.
	SaveCancellation(ctx context.Context, b Booking) error
	// SaveApproval stores a booking that Approve has changed. An approve can
	// race a cancel or another approve, so the store decides the winner: the
	// loser gets ErrAlreadyCancelled or ErrNotPending. It returns
	// ErrBookingNotFound if there is no such booking.
	SaveApproval(ctx context.Context, b Booking) error
}

// SpaceStore keeps spaces.
type SpaceStore interface {
	// GetSpace returns the space and the timezone of its location, which is
	// never nil. It returns ErrSpaceNotFound if there is no such space.
	GetSpace(ctx context.Context, id int64) (Space, *time.Location, error)
}

// MemberStore keeps members.
type MemberStore interface {
	// GetMember returns the member even if deleted, so the caller can tell the
	// two cases apart. It returns ErrMemberNotFound if there is no such member.
	GetMember(ctx context.Context, id int64) (Member, error)
}

// BookingService carries out booking requests: it loads what the domain rules
// need, applies them and stores the result.
type BookingService struct {
	bookings BookingStore
	spaces   SpaceStore
	members  MemberStore
	now      func() time.Time
}

// NewBookingService returns a BookingService. now is the clock, passed in so
// that tests can choose the time. Production code passes time.Now.
func NewBookingService(bookings BookingStore, spaces SpaceStore, members MemberStore, now func() time.Time) *BookingService {
	return &BookingService{bookings: bookings, spaces: spaces, members: members, now: now}
}

// Book creates a booking of the space for p.MemberID on behalf of actor. The
// rules, including who may book for whom, are in NewBooking, which runs before
// the member is loaded so a refused actor learns nothing about the member. A
// deleted member can't be booked for. Errors from the stores and NewBooking come
// back as they are, including ErrSlotTaken when another booking won the slot.
func (s *BookingService) Book(ctx context.Context, actor Actor, spaceID int64, p BookingParams) (Booking, error) {
	space, loc, err := s.spaces.GetSpace(ctx, spaceID)
	if err != nil {
		return Booking{}, err
	}

	b, err := NewBooking(actor, space, loc, p, s.now())
	if err != nil {
		return Booking{}, err
	}

	member, err := s.members.GetMember(ctx, p.MemberID)
	if err != nil {
		return Booking{}, err
	}
	if !member.deletedAt.IsZero() {
		return Booking{}, ErrMemberDeleted
	}

	return s.bookings.CreateBooking(ctx, b)
}

// Cancel cancels the booking on behalf of actor and returns it as stored. The
// rules are in Booking.Cancel. Errors from the stores come back as they are.
func (s *BookingService) Cancel(ctx context.Context, actor Actor, bookingID int64) (Booking, error) {
	b, err := s.bookings.GetBooking(ctx, bookingID)
	if err != nil {
		return Booking{}, err
	}

	space, _, err := s.spaces.GetSpace(ctx, b.spaceID)
	if err != nil {
		return Booking{}, err
	}

	if err := b.Cancel(actor, space, s.now()); err != nil {
		return Booking{}, err
	}
	if err := s.bookings.SaveCancellation(ctx, b); err != nil {
		return Booking{}, err
	}
	return b, nil
}

// Approve records actor's approval of a pending booking and returns it as
// stored. The rules are in Booking.Approve. Errors from the stores come back as
// they are.
func (s *BookingService) Approve(ctx context.Context, actor Actor, bookingID int64) (Booking, error) {
	b, err := s.bookings.GetBooking(ctx, bookingID)
	if err != nil {
		return Booking{}, err
	}

	if err := b.Approve(actor, s.now()); err != nil {
		return Booking{}, err
	}
	if err := s.bookings.SaveApproval(ctx, b); err != nil {
		return Booking{}, err
	}
	return b, nil
}
