package cowbook

import (
	"errors"
	"testing"
	"time"
)

// The booking from cancellableBooking is for member 3, in space 7 at location 1.
func TestCancelPermissions(t *testing.T) {
	tests := []struct {
		name    string
		actor   Actor
		wantErr error
	}{
		{"the booker", MemberActor(3), nil},
		{"another member", MemberActor(4), ErrCancelNotAllowed},
		{"admin", AdminActor(1), nil},
		{"front desk at the location", FrontDeskActor(2, []int64{9, 1}), nil},
		{"front desk at another location", FrontDeskActor(2, []int64{9}), ErrCancelNotAllowed},
		{"front desk with no locations", FrontDeskActor(2, nil), ErrCancelNotAllowed},
		// Members and staff are separate tables, so staff 3 and member 3 are
		// different people.
		{"front desk with the booker's ID", FrontDeskActor(3, []int64{9}), ErrCancelNotAllowed},
		{"zero actor", Actor{}, ErrCancelNotAllowed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := cancellableBooking(t)

			err := b.Cancel(tt.actor, storedSpace(t), bookingNow)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Cancel() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestFrontDeskActorKeepsOwnLocations(t *testing.T) {
	locations := []int64{1}
	actor := FrontDeskActor(2, locations)

	locations[0] = 9

	b := cancellableBooking(t)
	if err := b.Cancel(actor, storedSpace(t), bookingNow); err != nil {
		t.Errorf("Cancel() error = %v, want nil: changing the caller's slice must not change the actor", err)
	}
}

func TestCancelRefusesEveryoneOnWrongSpace(t *testing.T) {
	other := storedSpace(t)
	other.ID = 8
	b := cancellableBooking(t)

	err := b.Cancel(AdminActor(1), other, bookingNow)
	if !errors.Is(err, ErrCancelNotAllowed) {
		t.Errorf("Cancel() error = %v, want %v", err, ErrCancelNotAllowed)
	}
}

// Someone who may not cancel must not learn whether the booking was cancelled.
func TestCancelChecksPermissionFirst(t *testing.T) {
	b := cancellableBooking(t)
	if err := b.Cancel(MemberActor(3), storedSpace(t), bookingNow); err != nil {
		t.Fatalf("first Cancel() error = %v, want nil", err)
	}

	err := b.Cancel(MemberActor(4), storedSpace(t), bookingNow)
	if !errors.Is(err, ErrCancelNotAllowed) {
		t.Errorf("Cancel() error = %v, want %v", err, ErrCancelNotAllowed)
	}

	late := utc(time.November, 9, 12, 0)
	fresh := cancellableBooking(t)
	err = fresh.Cancel(MemberActor(4), storedSpace(t), late)
	if !errors.Is(err, ErrCancelNotAllowed) {
		t.Errorf("Cancel() after the end error = %v, want %v", err, ErrCancelNotAllowed)
	}
}
