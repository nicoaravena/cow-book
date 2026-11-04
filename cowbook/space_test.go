package cowbook

import (
	"errors"
	"testing"
	"time"
)

// validSpaceParams returns params that pass validation. Each test case
// changes one field, so a failure points at exactly one rule.
func validSpaceParams() SpaceParams {
	return SpaceParams{
		LocationID:       1,
		Name:             "Blue Room",
		Kind:             KindMeetingRoom,
		Capacity:         8,
		MaxBookingLength: 4 * time.Hour,
		Hours:            mustHours(OpeningWindow{time.Monday, at(9, 0), at(17, 0)}),
	}
}

func TestNewSpaceKeepsHours(t *testing.T) {
	s, err := NewSpace(validSpaceParams(), time.Now())
	if err != nil {
		t.Fatalf("NewSpace() error = %v, want nil", err)
	}

	// Monday 2 November, inside the window, and Tuesday 3 November, outside it.
	monday := time.Date(2026, time.November, 2, 10, 0, 0, 0, time.UTC)
	if !s.hours.Contains(monday, monday.Add(time.Hour), time.UTC) {
		t.Error("hours do not contain Monday 10:00 to 11:00")
	}
	tuesday := monday.AddDate(0, 0, 1)
	if s.hours.Contains(tuesday, tuesday.Add(time.Hour), time.UTC) {
		t.Error("hours contain Tuesday 10:00 to 11:00")
	}
}

func TestNewSpaceSetsFields(t *testing.T) {
	now := time.Date(2026, time.November, 2, 9, 0, 0, 0, time.UTC)

	s, err := NewSpace(validSpaceParams(), now)
	if err != nil {
		t.Fatalf("NewSpace() error = %v, want nil", err)
	}

	if s.ID != 0 {
		t.Errorf("ID = %d, want 0 until the space is stored", s.ID)
	}
	if s.locationID != 1 {
		t.Errorf("locationID = %d, want 1", s.locationID)
	}
	if s.name != "Blue Room" {
		t.Errorf("name = %q, want %q", s.name, "Blue Room")
	}
	if s.kind != KindMeetingRoom {
		t.Errorf("kind = %q, want %q", s.kind, KindMeetingRoom)
	}
	if s.capacity != 8 {
		t.Errorf("capacity = %d, want 8", s.capacity)
	}
	if s.maxBookingLength != 4*time.Hour {
		t.Errorf("maxBookingLength = %v, want 4h", s.maxBookingLength)
	}
	if !s.available {
		t.Error("available = false, want a new space to be available")
	}
	if !s.createdAt.Equal(now) {
		t.Errorf("createdAt = %v, want %v", s.createdAt, now)
	}
	if !s.updatedAt.Equal(now) {
		t.Errorf("updatedAt = %v, want %v", s.updatedAt, now)
	}
}

func TestNewSpaceTrimsName(t *testing.T) {
	p := validSpaceParams()
	p.Name = "  Blue Room\n"

	s, err := NewSpace(p, time.Now())
	if err != nil {
		t.Fatalf("NewSpace() error = %v, want nil", err)
	}
	if s.name != "Blue Room" {
		t.Errorf("name = %q, want %q", s.name, "Blue Room")
	}
}

func TestNewSpaceValidation(t *testing.T) {
	tests := []struct {
		name    string
		change  func(p *SpaceParams)
		wantErr error
	}{
		{"valid params", func(p *SpaceParams) {}, nil},

		{"zero location", func(p *SpaceParams) { p.LocationID = 0 }, ErrInvalidLocation},
		{"negative location", func(p *SpaceParams) { p.LocationID = -1 }, ErrInvalidLocation},

		{"empty name", func(p *SpaceParams) { p.Name = "" }, ErrSpaceNameRequired},
		{"blank name", func(p *SpaceParams) { p.Name = "   " }, ErrSpaceNameRequired},

		{"hot desk", func(p *SpaceParams) { p.Kind = KindHotDesk }, nil},
		{"private office", func(p *SpaceParams) { p.Kind = KindPrivateOffice }, nil},
		{"meeting room", func(p *SpaceParams) { p.Kind = KindMeetingRoom }, nil},
		{"phone booth", func(p *SpaceParams) { p.Kind = KindPhoneBooth }, nil},
		{"empty kind", func(p *SpaceParams) { p.Kind = "" }, ErrInvalidSpaceKind},
		{"unknown kind", func(p *SpaceParams) { p.Kind = "castle" }, ErrInvalidSpaceKind},

		{"capacity of one", func(p *SpaceParams) { p.Capacity = 1 }, nil},
		{"zero capacity", func(p *SpaceParams) { p.Capacity = 0 }, ErrInvalidCapacity},
		{"negative capacity", func(p *SpaceParams) { p.Capacity = -1 }, ErrInvalidCapacity},

		{"one minute length", func(p *SpaceParams) { p.MaxBookingLength = time.Minute }, nil},
		{"full day length", func(p *SpaceParams) { p.MaxBookingLength = 24 * time.Hour }, nil},
		{"zero length", func(p *SpaceParams) { p.MaxBookingLength = 0 }, ErrInvalidMaxBookingLength},
		{"negative length", func(p *SpaceParams) { p.MaxBookingLength = -time.Hour }, ErrInvalidMaxBookingLength},
		{"over a day", func(p *SpaceParams) { p.MaxBookingLength = 24*time.Hour + time.Nanosecond }, ErrInvalidMaxBookingLength},

		{"no opening hours", func(p *SpaceParams) { p.Hours = WeeklyHours{} }, ErrNoOpeningHours},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validSpaceParams()
			tt.change(&p)

			_, err := NewSpace(p, time.Now())
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("NewSpace() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
