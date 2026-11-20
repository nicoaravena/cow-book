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

// The compiler checks that *SpaceStore implements cowbook.SpaceStore.
var _ cowbook.SpaceStore = (*SpaceStore)(nil)

// SpaceStore keeps one tenant's spaces. Every method runs in WithTenant, so
// it only ever sees that tenant's schema.
type SpaceStore struct {
	pool     *pgxpool.Pool
	tenantID int64
}

// NewSpaceStore returns a SpaceStore for tenant tenantID. It holds no
// connection, so making one per request costs nothing.
func NewSpaceStore(pool *pgxpool.Pool, tenantID int64) *SpaceStore {
	return &SpaceStore{pool: pool, tenantID: tenantID}
}

// CreateSpace stores sp with its opening windows, all or nothing, and returns
// it with its ID set. It returns cowbook.ErrInvalidLocation if the tenant has
// no location sp.LocationID().
func (s *SpaceStore) CreateSpace(ctx context.Context, sp cowbook.Space) (cowbook.Space, error) {
	err := WithTenant(ctx, s.pool, s.tenantID, func(q *tenantdb.Queries) error {
		id, err := q.CreateSpace(ctx, tenantdb.CreateSpaceParams{
			LocationID:       sp.LocationID(),
			Name:             sp.Name(),
			Kind:             string(sp.Kind()),
			Capacity:         int32(sp.Capacity()),
			MaxBookingLength: durationToInterval(sp.MaxBookingLength()),
			Available:        sp.Available(),
			CreatedAt:        pgtype.Timestamptz{Time: sp.CreatedAt(), Valid: true},
			UpdatedAt:        pgtype.Timestamptz{Time: sp.UpdatedAt(), Valid: true},
		})
		if err != nil {
			return err
		}
		for _, w := range sp.Hours().Windows() {
			err := q.CreateOpeningWindow(ctx, tenantdb.CreateOpeningWindowParams{
				SpaceID:   id,
				Weekday:   int16(w.Day),
				OpensMin:  int16(w.Open.Minutes()),
				ClosesMin: int16(w.Close.Minutes()),
			})
			if err != nil {
				return err
			}
		}
		sp.ID = id
		return nil
	})

	// errors.As finds a *pgconn.PgError anywhere in err's chain and sets
	// pgErr to it, so its code and constraint name can be read.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" && pgErr.ConstraintName == "spaces_location_id_fkey" {
		return cowbook.Space{}, cowbook.ErrInvalidLocation
	}
	if err != nil {
		return cowbook.Space{}, err
	}
	return sp, nil
}

// GetSpace returns the space and its location's timezone. It returns
// cowbook.ErrSpaceNotFound if the tenant has no such space.
func (s *SpaceStore) GetSpace(ctx context.Context, id int64) (cowbook.Space, *time.Location, error) {
	var row tenantdb.GetSpaceRow
	var windows []tenantdb.OpeningWindow
	err := WithTenant(ctx, s.pool, s.tenantID, func(q *tenantdb.Queries) error {
		var err error
		if row, err = q.GetSpace(ctx, id); err != nil {
			return err
		}
		windows, err = q.ListOpeningWindows(ctx, id)
		return err
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return cowbook.Space{}, nil, cowbook.ErrSpaceNotFound
	}
	if err != nil {
		return cowbook.Space{}, nil, err
	}

	sp, err := spaceFromRow(row.Space, windows)
	if err != nil {
		return cowbook.Space{}, nil, err
	}
	// The table only checks the name isn't blank; Go checks it is a zone.
	loc, err := time.LoadLocation(row.Timezone)
	if err != nil {
		return cowbook.Space{}, nil, fmt.Errorf("space %d: timezone %q: %w", id, row.Timezone, err)
	}
	return sp, loc, nil
}

// ListSpaces returns every space of the tenant, sorted by ID. It reads all
// opening windows in a second query and hands each space its own, rather
// than running one query per space.
func (s *SpaceStore) ListSpaces(ctx context.Context) ([]cowbook.Space, error) {
	var rows []tenantdb.Space
	var windows []tenantdb.OpeningWindow
	err := WithTenant(ctx, s.pool, s.tenantID, func(q *tenantdb.Queries) error {
		var err error
		if rows, err = q.ListSpaces(ctx); err != nil {
			return err
		}
		windows, err = q.ListAllOpeningWindows(ctx)
		return err
	})
	if err != nil {
		return nil, err
	}

	bySpace := make(map[int64][]tenantdb.OpeningWindow)
	for _, w := range windows {
		bySpace[w.SpaceID] = append(bySpace[w.SpaceID], w)
	}
	spaces := make([]cowbook.Space, 0, len(rows))
	for _, row := range rows {
		sp, err := spaceFromRow(row, bySpace[row.ID])
		if err != nil {
			return nil, err
		}
		spaces = append(spaces, sp)
	}
	return spaces, nil
}

// spaceFromRow rebuilds a cowbook.Space from its row and its opening windows.
// The domain checks the values again, so a row someone changed by hand into
// something NewSpace would refuse comes back as an error.
func spaceFromRow(row tenantdb.Space, windows []tenantdb.OpeningWindow) (cowbook.Space, error) {
	ws := make([]cowbook.OpeningWindow, 0, len(windows))
	for _, w := range windows {
		// The columns hold minutes since midnight, as ClockTime.Minutes.
		open, err := cowbook.NewClockTime(int(w.OpensMin)/60, int(w.OpensMin)%60)
		if err != nil {
			return cowbook.Space{}, fmt.Errorf("space %d: %w", row.ID, err)
		}
		closing, err := cowbook.NewClockTime(int(w.ClosesMin)/60, int(w.ClosesMin)%60)
		if err != nil {
			return cowbook.Space{}, fmt.Errorf("space %d: %w", row.ID, err)
		}
		ws = append(ws, cowbook.OpeningWindow{Day: time.Weekday(w.Weekday), Open: open, Close: closing})
	}
	hours, err := cowbook.NewWeeklyHours(ws)
	if err != nil {
		return cowbook.Space{}, fmt.Errorf("space %d: %w", row.ID, err)
	}
	maxLength, err := intervalToDuration(row.MaxBookingLength)
	if err != nil {
		return cowbook.Space{}, fmt.Errorf("space %d: max booking length: %w", row.ID, err)
	}

	sp, err := cowbook.RestoreSpace(cowbook.SpaceRecord{
		ID: row.ID,
		SpaceParams: cowbook.SpaceParams{
			LocationID:       row.LocationID,
			Name:             row.Name,
			Kind:             cowbook.SpaceKind(row.Kind),
			Capacity:         int(row.Capacity),
			MaxBookingLength: maxLength,
			Hours:            hours,
		},
		Available: row.Available,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	})
	if err != nil {
		return cowbook.Space{}, fmt.Errorf("space %d: %w", row.ID, err)
	}
	return sp, nil
}

// durationToInterval stores d as microseconds, the unit Postgres keeps.
func durationToInterval(d time.Duration) pgtype.Interval {
	return pgtype.Interval{Microseconds: d.Microseconds(), Valid: true}
}

// intervalToDuration reads an interval back. A day counts as 24 hours, which
// is how Postgres compares intervals. A month has no fixed length, so it is
// refused; the check on max_booking_length keeps months out anyway.
func intervalToDuration(i pgtype.Interval) (time.Duration, error) {
	switch {
	case !i.Valid:
		return 0, errors.New("interval is NULL")
	case i.Months != 0:
		return 0, fmt.Errorf("interval has %d months, which have no fixed length", i.Months)
	}
	return time.Duration(i.Days)*24*time.Hour + time.Duration(i.Microseconds)*time.Microsecond, nil
}
