package postgres

import (
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"nicoaravena/cow-book-app/cowbook"
	"nicoaravena/cow-book-app/internal/postgres/tenantdb"
)

func TestApprovalRefusal(t *testing.T) {
	deadline := time.Date(2026, time.November, 3, 9, 0, 0, 0, time.UTC)
	set := func(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
	pending := tenantdb.Booking{ID: 4, ApprovalDeadline: set(deadline)}

	tests := []struct {
		name       string
		change     func(row *tenantdb.Booking)
		approvedAt time.Time
		want       error
	}{
		{"cancelled", func(row *tenantdb.Booking) { row.CancelledAt = set(deadline) }, deadline.Add(-time.Hour), cowbook.ErrAlreadyCancelled},
		{"cancelled and lapsed", func(row *tenantdb.Booking) { row.CancelledAt = set(deadline) }, deadline, cowbook.ErrAlreadyCancelled},
		{"approved already", func(row *tenantdb.Booking) { row.ApprovedAt = set(deadline) }, deadline.Add(-time.Hour), cowbook.ErrNotPending},
		{"member's own booking", func(row *tenantdb.Booking) { row.ApprovalDeadline = pgtype.Timestamptz{} }, deadline.Add(-time.Hour), cowbook.ErrNotPending},
		{"at the deadline", func(*tenantdb.Booking) {}, deadline, cowbook.ErrApprovalExpired},
		{"after the deadline", func(*tenantdb.Booking) {}, deadline.Add(time.Minute), cowbook.ErrApprovalExpired},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := pending
			tt.change(&row)
			if err := approvalRefusal(row, tt.approvedAt); !errors.Is(err, tt.want) {
				t.Errorf("approvalRefusal() = %v, want %v", err, tt.want)
			}
		})
	}
}

// A pending booking the UPDATE should have approved means a bug, not a
// refusal the caller can show, so it must not look like one.
func TestApprovalRefusalStillPending(t *testing.T) {
	deadline := time.Date(2026, time.November, 3, 9, 0, 0, 0, time.UTC)
	row := tenantdb.Booking{ID: 4, ApprovalDeadline: pgtype.Timestamptz{Time: deadline, Valid: true}}

	err := approvalRefusal(row, deadline.Add(-time.Hour))
	for _, refusal := range []error{cowbook.ErrAlreadyCancelled, cowbook.ErrNotPending, cowbook.ErrApprovalExpired} {
		if err == nil || errors.Is(err, refusal) {
			t.Fatalf("approvalRefusal() = %v, want an error that is no refusal", err)
		}
	}
}

func TestNullableColumns(t *testing.T) {
	if got := timestamptz(time.Time{}); got.Valid {
		t.Errorf("timestamptz(zero) = %+v, want NULL", got)
	}
	now := time.Date(2026, time.November, 2, 9, 0, 0, 0, time.UTC)
	if got := timestamptz(now); !got.Valid || !got.Time.Equal(now) {
		t.Errorf("timestamptz(%v) = %+v, want that time", now, got)
	}
	if got := nullableID(0); got.Valid {
		t.Errorf("nullableID(0) = %+v, want NULL", got)
	}
	if got := nullableID(9); !got.Valid || got.Int64 != 9 {
		t.Errorf("nullableID(9) = %+v, want 9", got)
	}
}
