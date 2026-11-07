package cowbook

import (
	"errors"
	"testing"
	"time"
)

// memberNow is the clock used by these tests.
var memberNow = utc(time.November, 2, 9, 0)

func validMemberParams() MemberParams {
	return MemberParams{Name: "Ana Rojas", Email: "ana@example.com"}
}

func TestNewMemberSetsFields(t *testing.T) {
	m, err := NewMember(MemberParams{Name: "  Ana Rojas ", Email: " Ana@Example.COM "}, memberNow)
	if err != nil {
		t.Fatalf("NewMember() error = %v, want nil", err)
	}

	if m.ID != 0 {
		t.Errorf("ID = %d, want 0 until the member is stored", m.ID)
	}
	if m.name != "Ana Rojas" {
		t.Errorf("name = %q, want it trimmed", m.name)
	}
	if m.email != "ana@example.com" {
		t.Errorf("email = %q, want it trimmed and lowercased", m.email)
	}
	if !m.createdAt.Equal(memberNow) {
		t.Errorf("createdAt = %v, want %v", m.createdAt, memberNow)
	}
	if !m.updatedAt.Equal(memberNow) {
		t.Errorf("updatedAt = %v, want %v", m.updatedAt, memberNow)
	}
	if !m.deletedAt.IsZero() {
		t.Errorf("deletedAt = %v, want zero on a new member", m.deletedAt)
	}
}

func TestNewMemberValidation(t *testing.T) {
	tests := []struct {
		name    string
		change  func(p *MemberParams)
		wantErr error
	}{
		{"valid params", func(p *MemberParams) {}, nil},

		{"empty name", func(p *MemberParams) { p.Name = "" }, ErrMemberNameRequired},
		{"blank name", func(p *MemberParams) { p.Name = "   " }, ErrMemberNameRequired},

		{"empty email", func(p *MemberParams) { p.Email = "" }, ErrInvalidMemberEmail},
		{"blank email", func(p *MemberParams) { p.Email = "   " }, ErrInvalidMemberEmail},
		{"no at sign", func(p *MemberParams) { p.Email = "ana.example.com" }, ErrInvalidMemberEmail},
		{"no domain", func(p *MemberParams) { p.Email = "ana@" }, ErrInvalidMemberEmail},
		{"display name", func(p *MemberParams) { p.Email = "Ana <ana@example.com>" }, ErrInvalidMemberEmail},
		{"two addresses", func(p *MemberParams) { p.Email = "ana@example.com, bo@example.com" }, ErrInvalidMemberEmail},

		{"name before email", func(p *MemberParams) {
			p.Name = ""
			p.Email = ""
		}, ErrMemberNameRequired},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validMemberParams()
			tt.change(&p)

			_, err := NewMember(p, memberNow)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("NewMember() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestMemberDelete(t *testing.T) {
	m, err := NewMember(validMemberParams(), memberNow)
	if err != nil {
		t.Fatalf("NewMember() error = %v, want nil", err)
	}
	later := memberNow.Add(time.Hour)

	if err := m.Delete(later); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}

	if !m.deletedAt.Equal(later) {
		t.Errorf("deletedAt = %v, want %v", m.deletedAt, later)
	}
	if !m.updatedAt.Equal(later) {
		t.Errorf("updatedAt = %v, want %v", m.updatedAt, later)
	}
	if !m.createdAt.Equal(memberNow) {
		t.Errorf("createdAt = %v, want it unchanged at %v", m.createdAt, memberNow)
	}
}

func TestMemberDeleteTwice(t *testing.T) {
	m, err := NewMember(validMemberParams(), memberNow)
	if err != nil {
		t.Fatalf("NewMember() error = %v, want nil", err)
	}
	if err := m.Delete(memberNow.Add(time.Hour)); err != nil {
		t.Fatalf("first Delete() error = %v, want nil", err)
	}
	before := m

	err = m.Delete(memberNow.Add(2 * time.Hour))

	if !errors.Is(err, ErrMemberDeleted) {
		t.Errorf("second Delete() error = %v, want %v", err, ErrMemberDeleted)
	}
	if m != before {
		t.Errorf("member = %+v, want it unchanged: %+v", m, before)
	}
}

func TestCancelBookingsOnDeletion(t *testing.T) {
	end := validBookingParams().End
	now := end.Add(-30 * time.Minute) // the 10:00-11:00 bookings are in progress

	cancelled := cancellableBooking(t)
	if err := cancelled.Cancel(MemberActor(3), storedSpace(t), bookingNow); err != nil {
		t.Fatalf("Cancel() error = %v, want nil", err)
	}
	ended := cancellableBooking(t)
	ended.end = now

	bs := []Booking{cancellableBooking(t), pendingBooking(t), cancelled, ended}
	bs[0].ID, bs[1].ID, bs[2].ID, bs[3].ID = 21, 22, 23, 24

	got := cancelBookings(bs, now)

	if len(got) != 2 || got[0].ID != 21 || got[1].ID != 22 {
		t.Fatalf("cancelBookings() = %+v, want bookings 21 and 22", got)
	}
	for _, b := range got {
		if !b.cancelledAt.Equal(now) {
			t.Errorf("booking %d cancelledAt = %v, want %v", b.ID, b.cancelledAt, now)
		}
	}
	// The input elements change in place, so the loop must go by index: a
	// range value is a copy of the element.
	if !bs[0].cancelledAt.Equal(now) || !bs[1].cancelledAt.Equal(now) {
		t.Errorf("input bookings 21 and 22 not cancelled in place: %+v", bs[:2])
	}
	if !bs[2].cancelledAt.Equal(bookingNow) {
		t.Errorf("already cancelled booking cancelledAt = %v, want it unchanged at %v", bs[2].cancelledAt, bookingNow)
	}
	if !bs[3].cancelledAt.IsZero() {
		t.Errorf("ended booking cancelledAt = %v, want zero", bs[3].cancelledAt)
	}
}
