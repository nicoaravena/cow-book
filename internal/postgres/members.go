package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"nicoaravena/cow-book-app/cowbook"
	"nicoaravena/cow-book-app/internal/postgres/tenantdb"
)

// The compiler checks that *MemberStore implements cowbook.MemberStore.
var _ cowbook.MemberStore = (*MemberStore)(nil)

// MemberStore keeps one tenant's members. Every method runs in WithTenant, as
// in SpaceStore.
type MemberStore struct {
	pool     *pgxpool.Pool
	tenantID int64
}

// NewMemberStore returns a MemberStore for tenant tenantID. Like
// NewSpaceStore, it holds no connection.
func NewMemberStore(pool *pgxpool.Pool, tenantID int64) *MemberStore {
	return &MemberStore{pool: pool, tenantID: tenantID}
}

// GetMember returns the member, deleted or not, or cowbook.ErrMemberNotFound
// if the tenant has no such member.
func (s *MemberStore) GetMember(ctx context.Context, id int64) (cowbook.Member, error) {
	var row tenantdb.Member
	err := WithTenant(ctx, s.pool, s.tenantID, func(q *tenantdb.Queries) error {
		var err error
		row, err = q.GetMember(ctx, id)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return cowbook.Member{}, cowbook.ErrMemberNotFound
	}
	if err != nil {
		return cowbook.Member{}, err
	}
	return memberFromRow(row)
}

// CreateMember stores m and returns it with its ID set. It returns
// cowbook.ErrEmailTaken if a member of the tenant who isn't deleted has m's
// email: the partial unique index decides, so two racing sign-ups with one
// email can't both pass.
func (s *MemberStore) CreateMember(ctx context.Context, m cowbook.Member) (cowbook.Member, error) {
	err := WithTenant(ctx, s.pool, s.tenantID, func(q *tenantdb.Queries) error {
		var err error
		m.ID, err = q.CreateMember(ctx, tenantdb.CreateMemberParams{
			Name:      m.Name(),
			Email:     m.Email(),
			CreatedAt: timestamptz(m.CreatedAt()),
			UpdatedAt: timestamptz(m.UpdatedAt()),
		})
		return err
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "members_email_key" {
		return cowbook.Member{}, cowbook.ErrEmailTaken
	}
	if err != nil {
		return cowbook.Member{}, err
	}
	return m, nil
}

// ListMembers returns every member of the tenant, deleted ones included,
// sorted by ID.
func (s *MemberStore) ListMembers(ctx context.Context) ([]cowbook.Member, error) {
	var rows []tenantdb.Member
	err := WithTenant(ctx, s.pool, s.tenantID, func(q *tenantdb.Queries) error {
		var err error
		rows, err = q.ListMembers(ctx)
		return err
	})
	if err != nil {
		return nil, err
	}
	members := make([]cowbook.Member, 0, len(rows))
	for _, row := range rows {
		m, err := memberFromRow(row)
		if err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, nil
}

// SaveDeletion stores a member that Delete has changed and cancels their
// bookings, all in one transaction:
//
//  1. It marks the member deleted, only if they aren't yet. The losing one of
//     two deletions gets cowbook.ErrMemberDeleted. The row stays locked until
//     commit, so a booking insert for the member waits, then sees the
//     deletion and refuses (see CreateBooking).
//  2. It saves each booking in cancelled, the ones MemberService.Delete
//     listed and cancelled. One that someone else cancelled meanwhile keeps
//     that cancellation, with no error.
//  3. It cancels, at m.DeletedAt(), any other booking of the member that
//     isn't cancelled and hasn't ended: one inserted after the service's list
//     was read. An insert still running holds the member's row, so step 1
//     waited for it to commit, and this statement sees its booking.
//
// It returns cowbook.ErrMemberNotFound if the tenant has no such member.
func (s *MemberStore) SaveDeletion(ctx context.Context, m cowbook.Member, cancelled []cowbook.Booking) error {
	return WithTenant(ctx, s.pool, s.tenantID, func(q *tenantdb.Queries) error {
		n, err := q.DeleteMember(ctx, tenantdb.DeleteMemberParams{
			ID:        m.ID,
			DeletedAt: timestamptz(m.DeletedAt()),
			UpdatedAt: timestamptz(m.UpdatedAt()),
		})
		if err != nil {
			return err
		}
		if n == 0 {
			// Not there or already deleted; the row says which.
			if _, err := q.GetMember(ctx, m.ID); errors.Is(err, pgx.ErrNoRows) {
				return cowbook.ErrMemberNotFound
			} else if err != nil {
				return err
			}
			return cowbook.ErrMemberDeleted
		}

		for _, b := range cancelled {
			// 0 rows means it was already cancelled, which is fine here.
			if _, err := q.CancelBooking(ctx, cancelParams(b)); err != nil {
				return err
			}
		}

		_, err = q.CancelMemberBookingsEndingAfter(ctx, tenantdb.CancelMemberBookingsEndingAfterParams{
			MemberID: m.ID,
			Now:      timestamptz(m.DeletedAt()),
		})
		return err
	})
}

// memberFromRow rebuilds a cowbook.Member from its row. A NULL deleted_at's
// Time is the zero time, which the domain reads as not deleted.
func memberFromRow(row tenantdb.Member) (cowbook.Member, error) {
	m, err := cowbook.RestoreMember(cowbook.MemberRecord{
		ID:           row.ID,
		MemberParams: cowbook.MemberParams{Name: row.Name, Email: row.Email},
		CreatedAt:    row.CreatedAt.Time,
		UpdatedAt:    row.UpdatedAt.Time,
		DeletedAt:    row.DeletedAt.Time,
	})
	if err != nil {
		return cowbook.Member{}, fmt.Errorf("member %d: %w", row.ID, err)
	}
	return m, nil
}
