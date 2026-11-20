package cowbook

import (
	"errors"
	"fmt"
	"time"
)

// approvalWindow is how long a member has to approve a booking staff made for
// them, unless the booking starts sooner.
const approvalWindow = 24 * time.Hour

// Errors returned by NewBooking. Some come wrapped with details, so callers
// compare them with errors.Is and not with ==.
var (
	ErrBookNotAllowed      = errors.New("only the member or the operator's staff can book for this member")
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
//
// A booking staff make for a member is pending until the member approves it.
// Its state comes from which timestamps are set, with no separate status field.
type Booking struct {
	ID               int64
	spaceID          int64
	memberID         int64
	bookedBy         int64 // staff ID; 0 when the member booked for themselves
	attendees        int
	start            time.Time
	end              time.Time
	createdAt        time.Time
	updatedAt        time.Time
	approvalDeadline time.Time // zero when no approval is needed
	approvedAt       time.Time // zero until approved
	cancelledAt      time.Time // zero until cancelled
}

// The accessors below let stores outside this package read a booking without
// being able to change it.

// SpaceID returns the ID of the booked space.
func (b Booking) SpaceID() int64 { return b.spaceID }

// MemberID returns the ID of the member the booking is for.
func (b Booking) MemberID() int64 { return b.memberID }

// Attendees returns how many people the booking is for.
func (b Booking) Attendees() int { return b.attendees }

// Start returns when the booking starts.
func (b Booking) Start() time.Time { return b.start }

// End returns when the booking ends. The booking does not include End, so
// another booking may start right then.
func (b Booking) End() time.Time { return b.end }

// Cancelled reports whether the booking has been cancelled.
func (b Booking) Cancelled() bool { return !b.cancelledAt.IsZero() }

// Approved reports whether the member has approved a booking staff made for
// them. It is false for a booking that never needed approval.
func (b Booking) Approved() bool { return !b.approvedAt.IsZero() }

// ApprovalDeadline returns when a booking staff made must be approved by. It
// is zero for a booking the member made themselves.
func (b Booking) ApprovalDeadline() time.Time { return b.approvalDeadline }

// BookedBy returns the ID of the staff member who made the booking, or 0 if
// the member made it themselves.
func (b Booking) BookedBy() int64 { return b.bookedBy }

// ApprovedAt returns when the member approved the booking, or zero.
func (b Booking) ApprovedAt() time.Time { return b.approvedAt }

// CancelledAt returns when the booking was cancelled, or zero.
func (b Booking) CancelledAt() time.Time { return b.cancelledAt }

// CreatedAt returns when the booking was made.
func (b Booking) CreatedAt() time.Time { return b.createdAt }

// UpdatedAt returns when the booking last changed.
func (b Booking) UpdatedAt() time.Time { return b.updatedAt }

// BookingState is where a booking stands, for showing to people. The rules
// read the timestamps, not this.
type BookingState string

const (
	// StateConfirmed: the member made it, or approved it when staff did.
	StateConfirmed BookingState = "confirmed"
	// StatePending: staff made it and the member can still approve it.
	StatePending BookingState = "pending"
	// StateExpired: staff made it and the deadline passed unapproved. It
	// still holds its slot until a store expires it.
	StateExpired BookingState = "expired"
	// StateCancelled: cancelled or declined; its slot is free.
	StateCancelled BookingState = "cancelled"
)

// State returns where the booking stands at now, from its timestamps. A
// pending booking expires at its deadline, the same moment Approve starts
// refusing it.
func (b Booking) State(now time.Time) BookingState {
	switch {
	case !b.cancelledAt.IsZero():
		return StateCancelled
	case b.awaitingApproval() && !now.Before(b.approvalDeadline):
		return StateExpired
	case b.awaitingApproval():
		return StatePending
	}
	return StateConfirmed
}

// BookingParams holds the values needed to create a Booking.
type BookingParams struct {
	MemberID  int64
	Attendees int
	Start     time.Time
	End       time.Time
}

// NewBooking validates p against space and returns a Booking made by actor.
// The ID stays 0 until the booking is stored. It returns the first rule that p
// breaks. Permission comes first, so someone not allowed learns nothing about
// the space. loc is the timezone of the space's location and must not be nil.
// A booking made by staff waits for the member's approval.
func NewBooking(actor Actor, space Space, loc *time.Location, p BookingParams, now time.Time) (Booking, error) {
	// Sub gives real elapsed time, so a clock change can't stretch or shrink it.
	length := p.End.Sub(p.Start)

	switch {
	case !actor.actsFor(p.MemberID, space.locationID):
		return Booking{}, ErrBookNotAllowed
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

	b := Booking{
		spaceID:   space.ID,
		memberID:  p.MemberID,
		attendees: p.Attendees,
		start:     p.Start,
		end:       p.End,
		createdAt: now,
		updatedAt: now,
	}
	if actor.role != roleMember {
		b.bookedBy = actor.id
		b.approvalDeadline = now.Add(approvalWindow)
		if p.Start.Before(b.approvalDeadline) {
			b.approvalDeadline = p.Start
		}
	}
	return b, nil
}

// BookingRecord holds a stored booking's values: the ones NewBooking takes,
// plus those a store keeps. Zero times and a zero BookedBy stand for NULL.
type BookingRecord struct {
	ID      int64
	SpaceID int64
	BookingParams
	BookedBy         int64
	ApprovalDeadline time.Time
	ApprovedAt       time.Time
	CancelledAt      time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// RestoreBooking rebuilds a stored booking. Unlike RestoreSpace it can't go
// through the constructor: NewBooking needs the actor and the space, and
// refuses a start in the past, which a stored booking may well have. So it
// checks the rules that hold for a booking forever, whoever made it and
// whenever it is read.
func RestoreBooking(r BookingRecord) (Booking, error) {
	switch {
	case r.MemberID <= 0:
		return Booking{}, ErrInvalidMember
	case r.SpaceID <= 0:
		return Booking{}, ErrInvalidSpace
	case r.Attendees < 1:
		return Booking{}, ErrInvalidAttendees
	case !r.Start.Before(r.End):
		return Booking{}, ErrInvalidBookingTime
	}
	return Booking{
		ID:               r.ID,
		spaceID:          r.SpaceID,
		memberID:         r.MemberID,
		bookedBy:         r.BookedBy,
		attendees:        r.Attendees,
		start:            r.Start,
		end:              r.End,
		createdAt:        r.CreatedAt,
		updatedAt:        r.UpdatedAt,
		approvalDeadline: r.ApprovalDeadline,
		approvedAt:       r.ApprovedAt,
		cancelledAt:      r.CancelledAt,
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
	if space.ID != b.spaceID || !actor.actsFor(b.memberID, space.locationID) {
		return ErrCancelNotAllowed
	}
	return b.cancel(now)
}

// cancel holds Cancel's rules without the actor check, for when the system
// cancels and not a person, as when a member is deleted. On error the booking
// is unchanged.
func (b *Booking) cancel(now time.Time) error {
	switch {
	case !b.cancelledAt.IsZero():
		return ErrAlreadyCancelled
	case !now.Before(b.end):
		return ErrBookingEnded
	}

	b.cancelledAt = now
	b.updatedAt = now
	return nil
}

// Errors returned by Approve. Callers compare them with errors.Is.
var (
	ErrApproveNotAllowed = errors.New("only the member the booking is for can approve it")
	ErrNotPending        = errors.New("booking is not awaiting approval")
	ErrApprovalExpired   = errors.New("booking's approval deadline has passed")
)

// awaitingApproval reports whether b was made by staff and the member hasn't
// approved it yet. It doesn't look at cancellation or the deadline.
func (b Booking) awaitingApproval() bool {
	return !b.approvalDeadline.IsZero() && b.approvedAt.IsZero()
}

// Approve records the member's approval of a booking staff made for them. Only
// that member may approve, before the deadline and while the booking is not
// cancelled. Permission comes first, as in Cancel. Declining is Cancel. On
// error the booking is unchanged.
func (b *Booking) Approve(actor Actor, now time.Time) error {
	switch {
	case actor.role != roleMember || actor.id != b.memberID:
		return ErrApproveNotAllowed
	case !b.cancelledAt.IsZero():
		return ErrAlreadyCancelled
	case !b.awaitingApproval():
		return ErrNotPending
	case !now.Before(b.approvalDeadline):
		return ErrApprovalExpired
	}

	b.approvedAt = now
	b.updatedAt = now
	return nil
}
