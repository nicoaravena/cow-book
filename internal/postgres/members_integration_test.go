//go:build integration

package postgres

import (
	"errors"
	"testing"
	"time"

	"nicoaravena/cow-book-app/cowbook"
)

// These tests use bookingFixture: deleting a member is mostly about their
// bookings.

// getMember reads member id back and fails the test if it can't.
func (f *bookingFixture) getMember(t *testing.T, id int64) cowbook.Member {
	t.Helper()
	m, err := f.mstore.GetMember(t.Context(), id)
	if err != nil {
		t.Fatalf("GetMember(%d): %v", id, err)
	}
	return m
}

// deleted returns member id as read from the store and then deleted at now,
// not yet saved.
func (f *bookingFixture) deleted(t *testing.T, id int64, now time.Time) cowbook.Member {
	t.Helper()
	m := f.getMember(t, id)
	if err := m.Delete(now); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	return m
}

func TestMemberStoreGet(t *testing.T) {
	f := newBookingFixture(t, 1)
	id := f.members[0]

	m := f.getMember(t, id)
	if m.ID != id || m.Name() != "Member" || m.Email() != "member1@example.com" || m.Deleted() {
		t.Errorf("GetMember = %d, %q, %q, deleted %v; want %d, Member, member1@example.com, not deleted",
			m.ID, m.Name(), m.Email(), m.Deleted(), id)
	}
	if m.CreatedAt().IsZero() || m.UpdatedAt().IsZero() {
		t.Errorf("CreatedAt(), UpdatedAt() = %v, %v; want the column defaults", m.CreatedAt(), m.UpdatedAt())
	}

	// A deleted member still comes back, so the caller can tell the cases apart.
	f.deleteMember(t, id, bookingNow)
	if m := f.getMember(t, id); !m.Deleted() || !m.DeletedAt().Equal(bookingNow) {
		t.Errorf("Deleted(), DeletedAt() = %v, %v; want true, %v", m.Deleted(), m.DeletedAt(), bookingNow)
	}

	if _, err := f.mstore.GetMember(t.Context(), 999999); !errors.Is(err, cowbook.ErrMemberNotFound) {
		t.Errorf("unknown member: GetMember error = %v, want %v", err, cowbook.ErrMemberNotFound)
	}
	other := NewMemberStore(f.pool, createTestTenant(t, f.pool))
	if _, err := other.GetMember(t.Context(), id); !errors.Is(err, cowbook.ErrMemberNotFound) {
		t.Errorf("other tenant: GetMember error = %v, want %v", err, cowbook.ErrMemberNotFound)
	}
}

// internal/memstore's TestSaveDeletion, ported: of two deletions only the
// first is saved, and a booking someone else cancelled meanwhile keeps that
// cancellation.
func TestMemberStoreSaveDeletion(t *testing.T) {
	f := newBookingFixture(t, 1)
	id := f.members[0]
	stored := f.create(t, f.newBooking(t, cowbook.MemberActor(id), 0, id, 10, 12, bookingNow))

	// The member cancels the booking after the deletion read it, so the
	// deletion's copy is stale and must not replace the member's cancellation.
	byMember := f.cancel(t, stored, cowbook.MemberActor(id), bookingNow)
	byDeletion := stored
	if err := byDeletion.Cancel(cowbook.AdminActor(9), f.spaces[0], bookingNow.Add(time.Hour)); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	first, second := f.deleted(t, id, bookingNow), f.deleted(t, id, bookingNow)
	if err := f.mstore.SaveDeletion(t.Context(), first, []cowbook.Booking{byDeletion}); err != nil {
		t.Fatalf("SaveDeletion: %v", err)
	}
	if err := f.mstore.SaveDeletion(t.Context(), second, nil); !errors.Is(err, cowbook.ErrMemberDeleted) {
		t.Errorf("second SaveDeletion error = %v, want %v", err, cowbook.ErrMemberDeleted)
	}

	if m := f.getMember(t, id); !m.DeletedAt().Equal(bookingNow) {
		t.Errorf("DeletedAt() = %v, want %v", m.DeletedAt(), bookingNow)
	}
	checkSameBooking(t, f.get(t, stored.ID), byMember)

	ghost := first
	ghost.ID = 999999
	if err := f.mstore.SaveDeletion(t.Context(), ghost, nil); !errors.Is(err, cowbook.ErrMemberNotFound) {
		t.Errorf("unknown member: SaveDeletion error = %v, want %v", err, cowbook.ErrMemberNotFound)
	}
}

// The service lists the bookings, then saves. A booking made in between isn't
// in the list, and SaveDeletion cancels it anyway.
func TestMemberStoreSaveDeletionCancelsStragglers(t *testing.T) {
	f := newBookingFixture(t, 1)
	id := f.members[0]
	listed := f.create(t, f.newBooking(t, cowbook.MemberActor(id), 0, id, 10, 11, bookingNow))

	bs, err := f.store.ListMemberBookingsEndingAfter(t.Context(), id, bookingNow)
	if err != nil {
		t.Fatalf("ListMemberBookingsEndingAfter: %v", err)
	}
	straggler := f.create(t, f.newBooking(t, cowbook.MemberActor(id), 1, id, 14, 15, bookingNow))

	// What MemberService.Delete does with the list, with Cancel in place of
	// the unexported cancelBookings.
	deletedAt := bookingNow.Add(time.Minute)
	for i := range bs {
		if err := bs[i].Cancel(cowbook.AdminActor(9), f.spaces[0], deletedAt); err != nil {
			t.Fatalf("Cancel: %v", err)
		}
	}
	if err := f.mstore.SaveDeletion(t.Context(), f.deleted(t, id, deletedAt), bs); err != nil {
		t.Fatalf("SaveDeletion: %v", err)
	}

	for _, b := range []cowbook.Booking{listed, straggler} {
		if got := f.get(t, b.ID); !got.CancelledAt().Equal(deletedAt) {
			t.Errorf("booking %d: CancelledAt() = %v, want %v", b.ID, got.CancelledAt(), deletedAt)
		}
	}
}

// MemberService.Delete over the Postgres stores, end to end: at 12:00 it
// cancels the member's booking in progress and their lapsed pending one, and
// leaves their ended booking and another member's booking as they are.
func TestMemberServiceDeleteOverPostgres(t *testing.T) {
	f := newBookingFixture(t, 2)
	m1, m2 := f.members[0], f.members[1]
	ended := f.create(t, f.newBooking(t, cowbook.MemberActor(m1), 0, m1, 9, 10, bookingNow))
	inProgress := f.create(t, f.newBooking(t, cowbook.MemberActor(m1), 0, m1, 11, 13, bookingNow))
	pending := f.create(t, f.newBooking(t, cowbook.AdminActor(9), 1, m1, 14, 15, bookingNow))
	others := f.create(t, f.newBooking(t, cowbook.MemberActor(m2), 0, m2, 14, 15, bookingNow))

	now := f.at(12)
	svc := cowbook.NewMemberService(f.mstore, f.store, func() time.Time { return now })
	m, err := svc.Delete(t.Context(), m1)
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got := f.getMember(t, m1); !got.Deleted() || !got.DeletedAt().Equal(m.DeletedAt()) {
		t.Errorf("stored member: Deleted(), DeletedAt() = %v, %v; want true, %v", got.Deleted(), got.DeletedAt(), m.DeletedAt())
	}

	tests := []struct {
		name          string
		b             cowbook.Booking
		wantCancelled bool
	}{
		{"ended", ended, false},
		{"in progress", inProgress, true},
		{"lapsed pending", pending, true},
		{"another member's", others, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := f.get(t, tt.b.ID)
			if got.Cancelled() != tt.wantCancelled {
				t.Errorf("Cancelled() = %v, want %v", got.Cancelled(), tt.wantCancelled)
			}
			if tt.wantCancelled && !got.CancelledAt().Equal(now) {
				t.Errorf("CancelledAt() = %v, want %v", got.CancelledAt(), now)
			}
		})
	}
}

// internal/memstore's TestCreateMember, ported, plus the list: emails are
// unique among members who aren't deleted.
func TestMemberStoreCreateAndList(t *testing.T) {
	pool := testPool(t)
	store := NewMemberStore(pool, createTestTenant(t, pool))
	newMember := func(email string) cowbook.Member {
		m, err := cowbook.NewMember(cowbook.MemberParams{Name: "Ana", Email: email}, bookingNow)
		if err != nil {
			t.Fatalf("NewMember: %v", err)
		}
		return m
	}

	first, err := store.CreateMember(t.Context(), newMember("ana@example.com"))
	if err != nil || first.ID == 0 {
		t.Fatalf("CreateMember = member %d, %v; want an ID, nil", first.ID, err)
	}
	got, err := store.GetMember(t.Context(), first.ID)
	if err != nil || got.Name() != "Ana" || got.Email() != "ana@example.com" || !got.CreatedAt().Equal(bookingNow) {
		t.Errorf("GetMember = %q, %q, created %v, %v; want Ana, ana@example.com, created %v, nil",
			got.Name(), got.Email(), got.CreatedAt(), err, bookingNow)
	}
	if _, err := store.CreateMember(t.Context(), newMember("ANA@example.com")); !errors.Is(err, cowbook.ErrEmailTaken) {
		t.Errorf("taken email: CreateMember error = %v, want %v", err, cowbook.ErrEmailTaken)
	}

	deleted := first
	if err := deleted.Delete(bookingNow.Add(time.Hour)); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := store.SaveDeletion(t.Context(), deleted, nil); err != nil {
		t.Fatalf("SaveDeletion: %v", err)
	}
	second, err := store.CreateMember(t.Context(), newMember("ana@example.com"))
	if err != nil || second.ID <= first.ID {
		t.Fatalf("deleted member's email: CreateMember = member %d, %v; want a later ID, nil", second.ID, err)
	}

	members, err := store.ListMembers(t.Context())
	if err != nil || len(members) != 2 || members[0].ID != first.ID || members[1].ID != second.ID {
		t.Fatalf("ListMembers = %+v, %v; want members %d and %d", members, err, first.ID, second.ID)
	}
	if !members[0].Deleted() || members[1].Deleted() {
		t.Errorf("ListMembers deleted = %v, %v; want true, false", members[0].Deleted(), members[1].Deleted())
	}
}
