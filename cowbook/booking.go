package cowbook

import (
	"errors"
	"fmt"
	"time"
)

// Errors returned by NewBooking. Some come wrapped with details, so callers
// compare them with errors.Is and not with ==.
var (
	ErrInvalidMember       = errors.New("booking needs a valid member")
	ErrInvalidSpace        = errors.New("booking needs a stored space")
	ErrSpaceUnavailable    = errors.New("space is not available for booking")
	ErrInvalidAttendees    = errors.New("booking needs at least 1 attendee")
	ErrOverCapacity        = errors.New("booking exceeds the space's capacity")
	ErrInvalidBookingTime  = errors.New("booking must end after it starts")
	ErrBookingInPast       = errors.New("booking must not start in the past")
	ErrBookingTooLong      = errors.New("booking exceeds the space's max booking length")
	ErrOutsideOpeningHours = errors.New("booking must fit inside one opening window")
)

// Booking is a member's reservation of a space for a period of time. Overlaps
// with other bookings are not checked here: the database decides those.
type Booking struct {
	ID          int64
	spaceID     int64
	memberID    int64
	attendees   int
	start       time.Time
	end         time.Time
	createdAt   time.Time
	updatedAt   time.Time
	cancelledAt time.Time // zero until cancelled
}

// BookingParams holds the values needed to create a Booking.
type BookingParams struct {
	MemberID  int64
	Attendees int
	Start     time.Time
	End       time.Time
}

// NewBooking validates p against space and returns a Booking. The ID stays 0
// until the booking is stored. It returns the first rule that p breaks. loc is
// the timezone of the space's location and must not be nil.
func NewBooking(space Space, loc *time.Location, p BookingParams, now time.Time) (Booking, error) {
	// Sub gives real elapsed time, so a clock change can't stretch or shrink it.
	length := p.End.Sub(p.Start)

	switch {
	case p.MemberID <= 0:
		return Booking{}, ErrInvalidMember
	case space.ID <= 0:
		return Booking{}, ErrInvalidSpace
	case !space.available:
		return Booking{}, ErrSpaceUnavailable
	case p.Attendees < 1:
		return Booking{}, ErrInvalidAttendees
	case p.Attendees > space.capacity:
		return Booking{}, fmt.Errorf("%w: %d attendees, capacity %d", ErrOverCapacity, p.Attendees, space.capacity)
	case !p.Start.Before(p.End):
		return Booking{}, ErrInvalidBookingTime
	case p.Start.Before(now):
		return Booking{}, ErrBookingInPast
	case length > space.maxBookingLength:
		return Booking{}, fmt.Errorf("%w: %v, limit %v", ErrBookingTooLong, length, space.maxBookingLength)
	case !space.hours.Contains(p.Start, p.End, loc):
		return Booking{}, ErrOutsideOpeningHours
	}

	return Booking{
		spaceID:   space.ID,
		memberID:  p.MemberID,
		attendees: p.Attendees,
		start:     p.Start,
		end:       p.End,
		createdAt: now,
		updatedAt: now,
	}, nil
}

// Errors returned by Cancel. Callers compare them with errors.Is.
var (
	ErrCancelNotAllowed = errors.New("only the booker or the operator's staff can cancel this booking")
	ErrAlreadyCancelled = errors.New("booking is already cancelled")
	ErrBookingEnded     = errors.New("booking has already ended")
)

// Cancel marks the booking as cancelled, which frees its slot. space must be
// the booking's own space: its location decides what front desk staff may do,
// and any other space is refused. Permission comes first, so someone not
// allowed learns nothing about the booking. An ended booking stays as it is,
// to keep the history true. It has a pointer receiver because it changes the
// booking. On error the booking is unchanged.
func (b *Booking) Cancel(actor Actor, space Space, now time.Time) error {
	switch {
	case space.ID != b.spaceID || !actor.canCancel(*b, space.locationID):
		return ErrCancelNotAllowed
	case !b.cancelledAt.IsZero():
		return ErrAlreadyCancelled
	case !now.Before(b.end):
		return ErrBookingEnded
	}

	b.cancelledAt = now
	b.updatedAt = now
	return nil
}
