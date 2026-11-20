package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"nicoaravena/cow-book-app/cowbook"
	"nicoaravena/cow-book-app/internal/postgres/tenantdb"
)

// BookingStore keeps one tenant's bookings. Every method runs in WithTenant,
// as in SpaceStore.
//
// It has CreateBooking, GetBooking and SaveApproval so far. SaveCancellation
// and ListMemberBookingsEndingAfter arrive in 2.6, and with them the check
// that it implements cowbook.BookingStore.
type BookingStore struct {
	pool     *pgxpool.Pool
	tenantID int64
}

// NewBookingStore returns a BookingStore for tenant tenantID. Like
// NewSpaceStore, it holds no connection.
func NewBookingStore(pool *pgxpool.Pool, tenantID int64) *BookingStore {
	return &BookingStore{pool: pool, tenantID: tenantID}
}

// CreateBooking stores b and returns it with its ID set. It returns
// cowbook.ErrSlotTaken if b overlaps a booking of the same space that isn't
// cancelled, pending ones included, and cowbook.ErrSpaceNotFound or
// cowbook.ErrMemberNotFound if the tenant has no such space or member.
//
// A pending booking whose approval deadline has passed doesn't count: in the
// same transaction, before the insert, the overlapping ones are cancelled at
// b.CreatedAt(), the time the service's clock gave NewBooking. If the insert
// then fails, those cancellations roll back with it.
func (s *BookingStore) CreateBooking(ctx context.Context, b cowbook.Booking) (cowbook.Booking, error) {
	err := WithTenant(ctx, s.pool, s.tenantID, func(q *tenantdb.Queries) error {
		_, err := q.ExpireLapsedPending(ctx, tenantdb.ExpireLapsedPendingParams{
			Now:      timestamptz(b.CreatedAt()),
			SpaceID:  b.SpaceID(),
			StartsAt: timestamptz(b.Start()),
			EndsAt:   timestamptz(b.End()),
		})
		if err != nil {
			return err
		}
		b.ID, err = q.CreateBooking(ctx, tenantdb.CreateBookingParams{
			SpaceID:          b.SpaceID(),
			MemberID:         b.MemberID(),
			BookedBy:         nullableID(b.BookedBy()),
			Attendees:        int32(b.Attendees()),
			StartsAt:         timestamptz(b.Start()),
			EndsAt:           timestamptz(b.End()),
			ApprovalDeadline: timestamptz(b.ApprovalDeadline()),
			ApprovedAt:       timestamptz(b.ApprovedAt()),
			CancelledAt:      timestamptz(b.CancelledAt()),
			CreatedAt:        timestamptz(b.CreatedAt()),
			UpdatedAt:        timestamptz(b.UpdatedAt()),
		})
		return err
	})

	// The constraint names are checked as well as the codes, so a constraint
	// added later with the same code isn't mistaken for one of these.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch {
		case pgErr.Code == "23P01" && pgErr.ConstraintName == "bookings_no_overlap":
			return cowbook.Booking{}, cowbook.ErrSlotTaken
		case pgErr.Code == "23503" && pgErr.ConstraintName == "bookings_space_id_fkey":
			return cowbook.Booking{}, cowbook.ErrSpaceNotFound
		case pgErr.Code == "23503" && pgErr.ConstraintName == "bookings_member_id_fkey":
			return cowbook.Booking{}, cowbook.ErrMemberNotFound
		}
	}
	if err != nil {
		return cowbook.Booking{}, err
	}
	return b, nil
}

// GetBooking returns the booking, or cowbook.ErrBookingNotFound if the tenant
// has no such booking. A lapsed pending booking comes back as stored, not
// cancelled; its State says it has expired.
func (s *BookingStore) GetBooking(ctx context.Context, id int64) (cowbook.Booking, error) {
	var row tenantdb.Booking
	err := WithTenant(ctx, s.pool, s.tenantID, func(q *tenantdb.Queries) error {
		var err error
		row, err = q.GetBooking(ctx, id)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return cowbook.Booking{}, cowbook.ErrBookingNotFound
	}
	if err != nil {
		return cowbook.Booking{}, err
	}
	return bookingFromRow(row)
}

// SaveApproval stores the approval of a booking that Approve has changed. It
// writes only approved_at and updated_at, and only while the stored booking
// is still pending with its deadline ahead of b.ApprovedAt(): the check and
// the write are one UPDATE, so of two racing calls only one can pass it. The
// loser gets cowbook.ErrAlreadyCancelled, cowbook.ErrNotPending or
// cowbook.ErrApprovalExpired, the errors Approve itself gives. It returns
// cowbook.ErrBookingNotFound if the tenant has no such booking.
func (s *BookingStore) SaveApproval(ctx context.Context, b cowbook.Booking) error {
	return WithTenant(ctx, s.pool, s.tenantID, func(q *tenantdb.Queries) error {
		n, err := q.ApproveBooking(ctx, tenantdb.ApproveBookingParams{
			ID:         b.ID,
			ApprovedAt: timestamptz(b.ApprovedAt()),
			UpdatedAt:  timestamptz(b.UpdatedAt()),
		})
		if err != nil || n == 1 {
			return err
		}

		// Nothing changed, so read the row to say why. In the same
		// transaction, this sees what the UPDATE saw or anything committed
		// since, which only moves a booking further from pending.
		row, err := q.GetBooking(ctx, b.ID)
		if errors.Is(err, pgx.ErrNoRows) {
			return cowbook.ErrBookingNotFound
		}
		if err != nil {
			return err
		}
		return approvalRefusal(row, b.ApprovedAt())
	})
}

// approvalRefusal says why ApproveBooking didn't approve row at approvedAt,
// checking in the order Booking.Approve does.
func approvalRefusal(row tenantdb.Booking, approvedAt time.Time) error {
	switch {
	case row.CancelledAt.Valid:
		return cowbook.ErrAlreadyCancelled
	case row.ApprovedAt.Valid || !row.ApprovalDeadline.Valid:
		return cowbook.ErrNotPending
	case !approvedAt.Before(row.ApprovalDeadline.Time):
		return cowbook.ErrApprovalExpired
	}
	// Still pending and in time: the UPDATE should have matched it.
	return fmt.Errorf("booking %d: approval at %v not saved, though the booking is pending", row.ID, approvedAt)
}

// bookingFromRow rebuilds a cowbook.Booking from its row. A NULL column's
// Time is the zero time, which the domain reads as not set.
func bookingFromRow(row tenantdb.Booking) (cowbook.Booking, error) {
	b, err := cowbook.RestoreBooking(cowbook.BookingRecord{
		ID:      row.ID,
		SpaceID: row.SpaceID,
		BookingParams: cowbook.BookingParams{
			MemberID:  row.MemberID,
			Attendees: int(row.Attendees),
			Start:     row.StartsAt.Time,
			End:       row.EndsAt.Time,
		},
		BookedBy:         row.BookedBy.Int64,
		ApprovalDeadline: row.ApprovalDeadline.Time,
		ApprovedAt:       row.ApprovedAt.Time,
		CancelledAt:      row.CancelledAt.Time,
		CreatedAt:        row.CreatedAt.Time,
		UpdatedAt:        row.UpdatedAt.Time,
	})
	if err != nil {
		return cowbook.Booking{}, fmt.Errorf("booking %d: %w", row.ID, err)
	}
	return b, nil
}

// timestamptz writes t to a nullable column: the zero time, the domain's
// "not set", becomes NULL.
func timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: !t.IsZero()}
}

// nullableID writes id to a nullable column: 0, the domain's "none", becomes
// NULL.
func nullableID(id int64) pgtype.Int8 {
	return pgtype.Int8{Int64: id, Valid: id != 0}
}
