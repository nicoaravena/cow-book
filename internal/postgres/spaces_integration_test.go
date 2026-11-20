//go:build integration

package postgres

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"nicoaravena/cow-book-app/cowbook"
	"nicoaravena/cow-book-app/internal/postgres/tenantdb"
)

// window returns an opening window on day from hour opens to hour closes.
func window(t *testing.T, day time.Weekday, opens, closes int) cowbook.OpeningWindow {
	t.Helper()
	o, err := cowbook.NewClockTime(opens, 0)
	if err != nil {
		t.Fatal(err)
	}
	c, err := cowbook.NewClockTime(closes, 0)
	if err != nil {
		t.Fatal(err)
	}
	return cowbook.OpeningWindow{Day: day, Open: o, Close: c}
}

// weekdays returns a window from opens to closes on Monday to Friday.
func weekdays(t *testing.T, opens, closes int) []cowbook.OpeningWindow {
	t.Helper()
	var ws []cowbook.OpeningWindow
	for day := time.Monday; day <= time.Friday; day++ {
		ws = append(ws, window(t, day, opens, closes))
	}
	return ws
}

// spaceParams returns valid params for a meeting room at locationID.
func spaceParams(t *testing.T, locationID int64, name string, windows []cowbook.OpeningWindow) cowbook.SpaceParams {
	t.Helper()
	hours, err := cowbook.NewWeeklyHours(windows)
	if err != nil {
		t.Fatalf("NewWeeklyHours: %v", err)
	}
	return cowbook.SpaceParams{
		LocationID:       locationID,
		Name:             name,
		Kind:             cowbook.KindMeetingRoom,
		Capacity:         8,
		MaxBookingLength: 2*time.Hour + 30*time.Minute,
		Hours:            hours,
	}
}

// createSpace stores a new space and returns it with its ID set.
func createSpace(t *testing.T, store *SpaceStore, p cowbook.SpaceParams, now time.Time) cowbook.Space {
	t.Helper()
	sp, err := cowbook.NewSpace(p, now)
	if err != nil {
		t.Fatalf("NewSpace: %v", err)
	}
	sp, err = store.CreateSpace(t.Context(), sp)
	if err != nil {
		t.Fatalf("CreateSpace: %v", err)
	}
	return sp
}

func TestSpaceStoreCreateAndGet(t *testing.T) {
	pool := testPool(t)
	tenant := createTestTenant(t, pool)
	store := NewSpaceStore(pool, tenant)
	locationID := createLocation(t, pool, tenant, "Centre")
	// Nanoseconds on purpose: Postgres keeps microseconds.
	now := time.Date(2026, time.November, 19, 10, 0, 0, 123456789, time.UTC)

	windows := append(weekdays(t, 9, 18), window(t, time.Saturday, 10, 14))
	created := createSpace(t, store, spaceParams(t, locationID, "Blue Room", windows), now)
	if created.ID <= 0 {
		t.Fatalf("CreateSpace ID = %d, want > 0", created.ID)
	}

	got, loc, err := store.GetSpace(t.Context(), created.ID)
	if err != nil {
		t.Fatalf("GetSpace: %v", err)
	}
	if got.ID != created.ID || got.LocationID() != locationID || got.Name() != "Blue Room" {
		t.Errorf("ID, LocationID, Name = %d, %d, %q; want %d, %d, Blue Room",
			got.ID, got.LocationID(), got.Name(), created.ID, locationID)
	}
	if got.Kind() != cowbook.KindMeetingRoom || got.Capacity() != 8 || got.MaxBookingLength() != 2*time.Hour+30*time.Minute {
		t.Errorf("Kind, Capacity, MaxBookingLength = %q, %d, %v; want meeting_room, 8, 2h30m",
			got.Kind(), got.Capacity(), got.MaxBookingLength())
	}
	if !slices.Equal(got.Hours().Windows(), created.Hours().Windows()) {
		t.Errorf("Hours().Windows() = %v, want %v", got.Hours().Windows(), created.Hours().Windows())
	}
	if !got.Available() {
		t.Error("Available() = false, want true")
	}
	// pgx sends times in binary as whole microseconds, dropping the rest, so
	// .123456789 is stored as .123456. Postgres itself would round the same
	// time typed as text to .123457.
	want := now.Truncate(time.Microsecond)
	if !got.CreatedAt().Equal(want) || !got.UpdatedAt().Equal(want) {
		t.Errorf("CreatedAt, UpdatedAt = %v, %v; want %v, cut to the microsecond",
			got.CreatedAt(), got.UpdatedAt(), want)
	}
	if loc == nil || loc.String() != "Europe/Madrid" {
		t.Errorf("location = %v, want Europe/Madrid", loc)
	}
}

func TestSpaceStoreUnavailableRoundTrip(t *testing.T) {
	pool := testPool(t)
	tenant := createTestTenant(t, pool)
	store := NewSpaceStore(pool, tenant)
	locationID := createLocation(t, pool, tenant, "Centre")
	now := time.Date(2026, time.November, 19, 10, 0, 0, 0, time.UTC)

	sp, err := cowbook.RestoreSpace(cowbook.SpaceRecord{
		SpaceParams: spaceParams(t, locationID, "Closed Room", weekdays(t, 9, 18)),
		Available:   false,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		t.Fatalf("RestoreSpace: %v", err)
	}
	sp, err = store.CreateSpace(t.Context(), sp)
	if err != nil {
		t.Fatalf("CreateSpace: %v", err)
	}

	got, _, err := store.GetSpace(t.Context(), sp.ID)
	if err != nil {
		t.Fatalf("GetSpace: %v", err)
	}
	if got.Available() {
		t.Error("Available() = true, want false as stored")
	}
}

func TestSpaceStoreGetMissing(t *testing.T) {
	pool := testPool(t)
	store := NewSpaceStore(pool, createTestTenant(t, pool))

	_, loc, err := store.GetSpace(t.Context(), 999999)
	if !errors.Is(err, cowbook.ErrSpaceNotFound) {
		t.Errorf("GetSpace error = %v, want ErrSpaceNotFound", err)
	}
	if loc != nil {
		t.Errorf("location = %v, want nil with an error", loc)
	}
}

func TestSpaceStoreOtherTenant(t *testing.T) {
	pool := testPool(t)
	a := createTestTenant(t, pool)
	b := createTestTenant(t, pool)
	now := time.Date(2026, time.November, 19, 10, 0, 0, 0, time.UTC)

	locationID := createLocation(t, pool, a, "Centre")
	sp := createSpace(t, NewSpaceStore(pool, a), spaceParams(t, locationID, "Blue Room", weekdays(t, 9, 18)), now)

	// Tenant b has no space with that id: its own spaces table is empty.
	_, _, err := NewSpaceStore(pool, b).GetSpace(t.Context(), sp.ID)
	if !errors.Is(err, cowbook.ErrSpaceNotFound) {
		t.Errorf("tenant b: GetSpace error = %v, want ErrSpaceNotFound", err)
	}
}

func TestSpaceStoreUnknownLocation(t *testing.T) {
	pool := testPool(t)
	store := NewSpaceStore(pool, createTestTenant(t, pool))
	now := time.Date(2026, time.November, 19, 10, 0, 0, 0, time.UTC)

	sp, err := cowbook.NewSpace(spaceParams(t, 999999, "Nowhere", weekdays(t, 9, 18)), now)
	if err != nil {
		t.Fatalf("NewSpace: %v", err)
	}
	if _, err := store.CreateSpace(t.Context(), sp); !errors.Is(err, cowbook.ErrInvalidLocation) {
		t.Errorf("CreateSpace error = %v, want ErrInvalidLocation", err)
	}

	spaces, err := store.ListSpaces(t.Context())
	if err != nil {
		t.Fatalf("ListSpaces: %v", err)
	}
	if len(spaces) != 0 {
		t.Errorf("ListSpaces = %d spaces, want 0 after the failed create", len(spaces))
	}
}

func TestSpaceStoreList(t *testing.T) {
	pool := testPool(t)
	tenant := createTestTenant(t, pool)
	store := NewSpaceStore(pool, tenant)
	now := time.Date(2026, time.November, 19, 10, 0, 0, 0, time.UTC)

	spaces, err := store.ListSpaces(t.Context())
	if err != nil || len(spaces) != 0 {
		t.Fatalf("empty tenant: ListSpaces = %d spaces, %v; want 0, nil", len(spaces), err)
	}

	locationID := createLocation(t, pool, tenant, "Centre")
	first := createSpace(t, store, spaceParams(t, locationID, "Blue Room", weekdays(t, 9, 18)), now)
	second := createSpace(t, store, spaceParams(t, locationID, "Weekend Desk",
		[]cowbook.OpeningWindow{window(t, time.Saturday, 10, 14), window(t, time.Sunday, 10, 12)}), now)

	spaces, err = store.ListSpaces(t.Context())
	if err != nil {
		t.Fatalf("ListSpaces: %v", err)
	}
	if len(spaces) != 2 || spaces[0].ID != first.ID || spaces[1].ID != second.ID {
		t.Fatalf("ListSpaces = %v, want spaces %d and %d in that order", spaces, first.ID, second.ID)
	}
	for i, want := range []cowbook.Space{first, second} {
		if !slices.Equal(spaces[i].Hours().Windows(), want.Hours().Windows()) {
			t.Errorf("space %d: Hours().Windows() = %v, want %v",
				want.ID, spaces[i].Hours().Windows(), want.Hours().Windows())
		}
	}
}

func TestSpaceStoreBadTimezone(t *testing.T) {
	pool := testPool(t)
	tenant := createTestTenant(t, pool)
	store := NewSpaceStore(pool, tenant)
	now := time.Date(2026, time.November, 19, 10, 0, 0, 0, time.UTC)

	// Only Go checks that a timezone exists, so the table takes this one.
	var locationID int64
	err := WithTenant(t.Context(), pool, tenant, func(q *tenantdb.Queries) error {
		var err error
		locationID, err = q.CreateLocation(t.Context(), tenantdb.CreateLocationParams{Name: "Olympus", Timezone: "Mars/Olympus"})
		return err
	})
	if err != nil {
		t.Fatalf("create location: %v", err)
	}
	sp := createSpace(t, store, spaceParams(t, locationID, "Red Room", weekdays(t, 9, 18)), now)

	_, loc, err := store.GetSpace(t.Context(), sp.ID)
	if err == nil || errors.Is(err, cowbook.ErrSpaceNotFound) {
		t.Errorf("GetSpace error = %v, want a timezone error", err)
	}
	if loc != nil {
		t.Errorf("location = %v, want nil with an error", loc)
	}
}

func TestSpaceStoreNoWindows(t *testing.T) {
	pool := testPool(t)
	tenant := createTestTenant(t, pool)
	store := NewSpaceStore(pool, tenant)
	now := time.Date(2026, time.November, 19, 10, 0, 0, 0, time.UTC)

	locationID := createLocation(t, pool, tenant, "Centre")
	sp := createSpace(t, store, spaceParams(t, locationID, "Blue Room", weekdays(t, 9, 18)), now)

	// As if someone deleted the rows by hand in psql.
	schema := pgx.Identifier{SchemaName(tenant)}.Sanitize()
	if _, err := pool.Exec(t.Context(), "DELETE FROM "+schema+".opening_windows"); err != nil {
		t.Fatalf("delete windows: %v", err)
	}

	if _, _, err := store.GetSpace(t.Context(), sp.ID); !errors.Is(err, cowbook.ErrNoOpeningHours) {
		t.Errorf("GetSpace error = %v, want ErrNoOpeningHours", err)
	}
}
