package cowbook

import (
	"errors"
	"testing"
	"time"
)

// at returns a ClockTime for a time the test knows is valid.
func at(hour, minute int) ClockTime {
	c, err := NewClockTime(hour, minute)
	if err != nil {
		panic(err)
	}
	return c
}

// mustHours returns WeeklyHours for windows the test knows are valid.
func mustHours(windows ...OpeningWindow) WeeklyHours {
	h, err := NewWeeklyHours(windows)
	if err != nil {
		panic(err)
	}
	return h
}

func loadLocation(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	if err != nil {
		t.Fatalf("time.LoadLocation(%q) error = %v, the zone database may be missing", name, err)
	}
	return loc
}

// utc returns an instant given in UTC. The DST tests use it for times that
// happen twice on the local clock, because time.Date can't say which one.
func utc(month time.Month, day, hour, minute int) time.Time {
	return time.Date(2026, month, day, hour, minute, 0, 0, time.UTC)
}

func TestNewClockTime(t *testing.T) {
	tests := []struct {
		name         string
		hour, minute int
		wantErr      error
	}{
		{"midnight", 0, 0, nil},
		{"last minute of the day", 23, 59, nil},
		{"end of the day", 24, 0, nil},
		{"past the end of the day", 24, 1, ErrInvalidClockTime},
		{"hour too large", 25, 0, ErrInvalidClockTime},
		{"negative hour", -1, 0, ErrInvalidClockTime},
		{"minute too large", 12, 60, ErrInvalidClockTime},
		{"negative minute", 12, -1, ErrInvalidClockTime},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewClockTime(tt.hour, tt.minute)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("NewClockTime(%d, %d) error = %v, want %v", tt.hour, tt.minute, err, tt.wantErr)
			}
		})
	}
}

func TestNewWeeklyHoursValidation(t *testing.T) {
	morning := OpeningWindow{time.Monday, at(9, 0), at(12, 0)}
	afternoon := OpeningWindow{time.Monday, at(13, 0), at(17, 0)}

	tests := []struct {
		name    string
		windows []OpeningWindow
		wantErr error
	}{
		{"no windows", nil, nil},
		{"one window", []OpeningWindow{morning}, nil},
		{"two windows in a day", []OpeningWindow{morning, afternoon}, nil},
		{"windows out of order", []OpeningWindow{afternoon, morning}, nil},
		{"full day", []OpeningWindow{{time.Monday, at(0, 0), at(24, 0)}}, nil},
		{"same times on different days", []OpeningWindow{morning, {time.Tuesday, at(9, 0), at(12, 0)}}, nil},

		{"weekday too small", []OpeningWindow{{-1, at(9, 0), at(12, 0)}}, ErrInvalidOpeningWindow},
		{"weekday too large", []OpeningWindow{{7, at(9, 0), at(12, 0)}}, ErrInvalidOpeningWindow},
		{"open equals close", []OpeningWindow{{time.Monday, at(9, 0), at(9, 0)}}, ErrInvalidOpeningWindow},
		{"open after close", []OpeningWindow{{time.Monday, at(17, 0), at(9, 0)}}, ErrInvalidOpeningWindow},

		{"overlapping windows", []OpeningWindow{morning, {time.Monday, at(11, 0), at(14, 0)}}, ErrOverlappingWindows},
		{"overlapping out of order", []OpeningWindow{{time.Monday, at(11, 0), at(14, 0)}, morning}, ErrOverlappingWindows},
		{"touching windows", []OpeningWindow{morning, {time.Monday, at(12, 0), at(17, 0)}}, ErrOverlappingWindows},
		{"identical windows", []OpeningWindow{morning, morning}, ErrOverlappingWindows},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewWeeklyHours(tt.windows)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("NewWeeklyHours() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestNewWeeklyHoursCopiesInput(t *testing.T) {
	windows := []OpeningWindow{{time.Monday, at(9, 0), at(17, 0)}}

	h, err := NewWeeklyHours(windows)
	if err != nil {
		t.Fatalf("NewWeeklyHours() error = %v, want nil", err)
	}
	windows[0].Close = at(10, 0)

	// Monday 9 November, 16:00 to 17:00.
	start := utc(time.November, 9, 16, 0)
	end := utc(time.November, 9, 17, 0)
	if !h.Contains(start, end, time.UTC) {
		t.Error("changing the input slice changed the hours")
	}
}

func TestContains(t *testing.T) {
	week := mustHours(
		OpeningWindow{time.Monday, at(9, 0), at(17, 0)},
		OpeningWindow{time.Tuesday, at(9, 0), at(12, 0)},
		OpeningWindow{time.Tuesday, at(13, 0), at(17, 0)},
		OpeningWindow{time.Wednesday, at(18, 0), at(24, 0)},
		OpeningWindow{time.Thursday, at(0, 0), at(6, 0)},
	)

	// The week of 9 November 2026: Monday 9, Tuesday 10, and so on.
	// No clock change is near it, so UTC keeps the times easy to read.
	day := func(date, hour, minute int) time.Time {
		return utc(time.November, date, hour, minute)
	}
	seconds := func(date, hour, minute, second int) time.Time {
		return day(date, hour, minute).Add(time.Duration(second) * time.Second)
	}

	tests := []struct {
		name       string
		start, end time.Time
		want       bool
	}{
		{"exactly the window", day(9, 9, 0), day(9, 17, 0), true},
		{"inside the window", day(9, 10, 0), day(9, 11, 0), true},
		{"starts a minute early", day(9, 8, 59), day(9, 11, 0), false},
		{"ends a minute late", day(9, 16, 0), day(9, 17, 1), false},
		{"starts 30 seconds early", seconds(9, 8, 59, 30), day(9, 11, 0), false},
		{"ends 30 seconds late", day(9, 16, 0), seconds(9, 17, 0, 30), false},
		{"ends a nanosecond late", day(9, 16, 0), day(9, 17, 0).Add(time.Nanosecond), false},

		{"closed day", day(15, 10, 0), day(15, 11, 0), false},
		{"inside the morning", day(10, 10, 0), day(10, 11, 0), true},
		{"inside the afternoon", day(10, 14, 0), day(10, 15, 0), true},
		{"across the lunch closure", day(10, 11, 0), day(10, 14, 0), false},
		{"inside the lunch closure", day(10, 12, 15), day(10, 12, 45), false},

		{"end equals start", day(9, 10, 0), day(9, 10, 0), false},
		{"end before start", day(9, 11, 0), day(9, 10, 0), false},

		{"ends exactly at midnight", day(11, 22, 0), day(12, 0, 0), true},
		{"crosses midnight", day(11, 22, 0), day(12, 1, 0), false},
		{"starts exactly at midnight", day(12, 0, 0), day(12, 1, 0), true},
		{"same time a week later", day(9, 10, 0), day(16, 11, 0), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := week.Contains(tt.start, tt.end, time.UTC); got != tt.want {
				t.Errorf("Contains(%v, %v) = %v, want %v", tt.start, tt.end, got, tt.want)
			}
		})
	}
}

func TestContainsNoWindows(t *testing.T) {
	if (WeeklyHours{}).Contains(utc(time.November, 9, 10, 0), utc(time.November, 9, 11, 0), time.UTC) {
		t.Error("Contains() = true on hours with no windows, want false")
	}
}

func TestContainsTimezones(t *testing.T) {
	newYork := loadLocation(t, "America/New_York")
	tokyo := loadLocation(t, "Asia/Tokyo")

	t.Run("same instants, open in one zone and closed in another", func(t *testing.T) {
		// Monday 15:00 UTC is Monday 10:00 in New York and Tuesday 00:00 in Tokyo.
		week := mustHours(
			OpeningWindow{time.Monday, at(9, 0), at(17, 0)},
			OpeningWindow{time.Tuesday, at(9, 0), at(17, 0)},
		)
		start := utc(time.November, 9, 15, 0)
		end := utc(time.November, 9, 16, 0)

		if !week.Contains(start, end, newYork) {
			t.Error("Contains() in New York = false, want true")
		}
		if week.Contains(start, end, tokyo) {
			t.Error("Contains() in Tokyo = true, want false")
		}
	})

	t.Run("weekday comes from local time", func(t *testing.T) {
		// Friday 20:00 in New York is already Saturday in UTC.
		friday := mustHours(OpeningWindow{time.Friday, at(18, 0), at(22, 0)})
		start := time.Date(2026, time.November, 6, 20, 0, 0, 0, newYork)
		end := time.Date(2026, time.November, 6, 21, 0, 0, 0, newYork)

		if !friday.Contains(start, end, newYork) {
			t.Error("Contains() in New York = false, want true")
		}
		if friday.Contains(start, end, time.UTC) {
			t.Error("Contains() in UTC = true, want false")
		}
	})

	t.Run("the zone of the inputs does not matter", func(t *testing.T) {
		monday := mustHours(OpeningWindow{time.Monday, at(9, 0), at(17, 0)})
		start := time.Date(2026, time.November, 9, 10, 0, 0, 0, newYork)
		end := time.Date(2026, time.November, 9, 11, 0, 0, 0, newYork)

		if !monday.Contains(start, end, newYork) {
			t.Error("Contains() with New York inputs = false, want true")
		}
		if !monday.Contains(start.UTC(), end.UTC(), newYork) {
			t.Error("Contains() with UTC inputs = false, want true")
		}
		if !monday.Contains(start.In(tokyo), end.In(tokyo), newYork) {
			t.Error("Contains() with Tokyo inputs = false, want true")
		}
	})
}

// New York in 2026: on Sunday 8 March the clock jumps from 02:00 to 03:00.
// On Sunday 1 November it goes back from 02:00 to 01:00, so 01:xx happens
// twice, first in EDT (UTC-4) and then in EST (UTC-5).
func TestContainsNewYorkDST(t *testing.T) {
	newYork := loadLocation(t, "America/New_York")

	local := func(month time.Month, day, hour, minute int) time.Time {
		return time.Date(2026, month, day, hour, minute, 0, 0, newYork)
	}

	tests := []struct {
		name       string
		window     OpeningWindow
		start, end time.Time
		want       bool
	}{
		{
			"spring: opening time right after the jump",
			OpeningWindow{time.Sunday, at(3, 0), at(10, 0)},
			utc(time.March, 8, 7, 0), utc(time.March, 8, 8, 0), // 03:00 EDT to 04:00 EDT
			true,
		},
		{
			"spring: a moment before the jump is outside",
			OpeningWindow{time.Sunday, at(3, 0), at(10, 0)},
			utc(time.March, 8, 6, 59), utc(time.March, 8, 8, 0), // 01:59 EST to 04:00 EDT
			false,
		},
		{
			"spring: booking across the jump, inside the window",
			OpeningWindow{time.Sunday, at(1, 0), at(4, 0)},
			local(time.March, 8, 1, 30), local(time.March, 8, 3, 30),
			true,
		},
		{
			"spring: closing time is read on the local clock",
			OpeningWindow{time.Sunday, at(1, 0), at(3, 0)},
			local(time.March, 8, 1, 30), local(time.March, 8, 3, 30),
			false,
		},
		{
			"fall: booking across the repeated hour, inside the window",
			OpeningWindow{time.Sunday, at(0, 0), at(3, 0)},
			utc(time.November, 1, 4, 30), utc(time.November, 1, 6, 30), // 00:30 EDT to 01:30 EST
			true,
		},
		{
			"fall: clock reading goes down during the booking",
			OpeningWindow{time.Sunday, at(0, 0), at(3, 0)},
			utc(time.November, 1, 5, 50), utc(time.November, 1, 6, 10), // 01:50 EDT to 01:10 EST
			true,
		},
		{
			"fall: closing time is read on the local clock",
			OpeningWindow{time.Sunday, at(0, 0), at(2, 0)},
			utc(time.November, 1, 5, 30), utc(time.November, 1, 7, 30), // 01:30 EDT to 02:30 EST
			false,
		},
		// Known limit: only the two ends are checked. This booking is outside
		// the window from 01:30 to 02:00 EDT, yet it passes. See the Risks
		// section of docs/ROADMAP.md. Fixing that should change this case.
		{
			"fall: known limit, window closes inside the repeated hour",
			OpeningWindow{time.Sunday, at(0, 0), at(1, 30)},
			utc(time.November, 1, 5, 0), utc(time.November, 1, 6, 20), // 01:00 EDT to 01:20 EST
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mustHours(tt.window).Contains(tt.start, tt.end, newYork); got != tt.want {
				t.Errorf("Contains(%v, %v) = %v, want %v", tt.start.In(newYork), tt.end.In(newYork), got, tt.want)
			}
		})
	}
}

// Santiago in 2026 changes its clocks at midnight. On Saturday 4 April the
// clock goes back from 24:00 to 23:00, so 23:xx happens twice, first in -03
// and then in -04. On Saturday 5 September it jumps from 24:00 to 01:00, so
// Sunday 6 September has no 00:xx at all.
func TestContainsSantiagoDST(t *testing.T) {
	santiago := loadLocation(t, "America/Santiago")

	saturday := OpeningWindow{time.Saturday, at(8, 0), at(24, 0)}
	sunday := OpeningWindow{time.Sunday, at(0, 0), at(12, 0)}

	tests := []struct {
		name       string
		window     OpeningWindow
		start, end time.Time
		want       bool
	}{
		{
			"jump: Saturday booking ending at 24:00",
			saturday,
			utc(time.September, 6, 3, 0), utc(time.September, 6, 4, 0), // 23:00 -04 to Sunday 01:00 -03
			true,
		},
		{
			"jump: Saturday booking running into Sunday",
			saturday,
			utc(time.September, 6, 3, 0), utc(time.September, 6, 4, 1), // 23:00 -04 to Sunday 01:01 -03
			false,
		},
		{
			"jump: Sunday booking from the first moment of the day",
			sunday,
			utc(time.September, 6, 4, 0), utc(time.September, 6, 5, 0), // 01:00 -03 to 02:00 -03
			true,
		},
		{
			"back: booking inside the first 23:xx",
			saturday,
			utc(time.April, 5, 2, 10), utc(time.April, 5, 2, 40), // 23:10 -03 to 23:40 -03
			true,
		},
		{
			"back: booking inside the second 23:xx",
			saturday,
			utc(time.April, 5, 3, 10), utc(time.April, 5, 3, 40), // 23:10 -04 to 23:40 -04
			true,
		},
		{
			"back: booking across the change stays on Saturday",
			saturday,
			utc(time.April, 5, 2, 30), utc(time.April, 5, 3, 30), // 23:30 -03 to 23:30 -04
			true,
		},
		{
			"back: booking ending at the real midnight",
			saturday,
			utc(time.April, 5, 3, 30), utc(time.April, 5, 4, 0), // 23:30 -04 to Sunday 00:00 -04
			true,
		},
		{
			"back: booking running into Sunday",
			saturday,
			utc(time.April, 5, 3, 30), utc(time.April, 5, 4, 30), // 23:30 -04 to Sunday 00:30 -04
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mustHours(tt.window).Contains(tt.start, tt.end, santiago); got != tt.want {
				t.Errorf("Contains(%v, %v) = %v, want %v", tt.start.In(santiago), tt.end.In(santiago), got, tt.want)
			}
		})
	}
}
