package cowbook

import (
	"errors"
	"net/mail"
	"strings"
	"time"
)

// Errors returned by NewMember, Delete, MemberService.Delete and the
// BookingService methods. Callers compare them with errors.Is.
var (
	ErrMemberNameRequired = errors.New("member name is required")
	ErrInvalidMemberEmail = errors.New("member email is not valid")
	ErrMemberDeleted      = errors.New("member is deleted")
)

// Member is a person who books spaces at one operator.
type Member struct {
	ID        int64
	name      string
	email     string
	createdAt time.Time
	updatedAt time.Time
	deletedAt time.Time // zero until deleted
}

// MemberParams holds the values needed to create a Member.
type MemberParams struct {
	Name  string
	Email string
}

// NewMember validates p and returns a Member. The ID stays 0 until the member
// is stored. It returns the first rule that p breaks. The email is lowercased
// because it must be unique per operator, so Ana@x.com and ana@x.com have to
// be the same member.
func NewMember(p MemberParams, now time.Time) (Member, error) {
	name := strings.TrimSpace(p.Name)
	email := strings.ToLower(strings.TrimSpace(p.Email))

	switch {
	case name == "":
		return Member{}, ErrMemberNameRequired
	case !validEmail(email):
		return Member{}, ErrInvalidMemberEmail
	}

	return Member{
		name:      name,
		email:     email,
		createdAt: now,
		updatedAt: now,
	}, nil
}

// validEmail reports whether email is a bare address. ParseAddress alone also
// accepts forms like "Ana <ana@x.com>", so the result must match the input.
func validEmail(email string) bool {
	addr, err := mail.ParseAddress(email)
	return err == nil && addr.Address == email
}

// Delete marks the member as deleted and keeps the record, so bookings and
// history still point at a real member. It has a pointer receiver because it
// changes the member. On error the member is unchanged.
func (m *Member) Delete(now time.Time) error {
	if !m.deletedAt.IsZero() {
		return ErrMemberDeleted
	}

	m.deletedAt = now
	m.updatedAt = now
	return nil
}

// cancelBookings cancels a deleted member's bookings: every one in bs that
// hasn't ended or been cancelled, pending ones and ones in progress included.
// It changes the elements of bs in place and returns copies of those it
// cancelled. The loop goes by index because a range value is a copy, and
// cancelling the copy would leave bs as it was.
func cancelBookings(bs []Booking, now time.Time) []Booking {
	var cancelled []Booking
	for i := range bs {
		if bs[i].cancel(now) == nil {
			cancelled = append(cancelled, bs[i])
		}
	}
	return cancelled
}
