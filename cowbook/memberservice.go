package cowbook

import (
	"context"
	"time"
)

// MemberService carries out member requests: it loads what the domain rules
// need, applies them and stores the result.
type MemberService struct {
	members  MemberStore
	bookings BookingStore
	now      func() time.Time
}

// NewMemberService returns a MemberService. now is the clock, passed in so
// that tests can choose the time. Production code passes time.Now.
func NewMemberService(members MemberStore, bookings BookingStore, now func() time.Time) *MemberService {
	return &MemberService{members: members, bookings: bookings, now: now}
}

// Delete marks the member deleted, cancels every booking of theirs that hasn't
// ended, and returns the member as stored. The system cancels, not a person,
// so no actor rules apply to the bookings. Who may delete a member is not
// checked yet. Errors from the stores come back as they are.
func (s *MemberService) Delete(ctx context.Context, memberID int64) (Member, error) {
	now := s.now()

	m, err := s.members.GetMember(ctx, memberID)
	if err != nil {
		return Member{}, err
	}
	if err := m.Delete(now); err != nil {
		return Member{}, err
	}

	bs, err := s.bookings.ListMemberBookingsEndingAfter(ctx, memberID, now)
	if err != nil {
		return Member{}, err
	}

	if err := s.members.SaveDeletion(ctx, m, cancelBookings(bs, now)); err != nil {
		return Member{}, err
	}
	return m, nil
}
