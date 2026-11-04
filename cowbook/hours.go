package cowbook

import (
	"cmp"
	"errors"
	"slices"
	"time"
)

const minutesPerDay = 24 * 60

// Errors returned when building opening hours. Callers compare them with errors.Is.
var (
	ErrInvalidClockTime     = errors.New("clock time must be between 00:00 and 24:00")
	ErrInvalidOpeningWindow = errors.New("opening window needs a valid weekday and must open before it closes")
	ErrOverlappingWindows   = errors.New("opening windows on the same day must not overlap or touch")
)

// ClockTime is a time of day as read on a wall clock, to the minute. It has
// no date or timezone. 24:00 is valid and means the very end of the day.
type ClockTime struct {
	minutes int
}

// NewClockTime returns the ClockTime for hour:minute.
func NewClockTime(hour, minute int) (ClockTime, error) {
	total := hour*60 + minute
	if hour < 0 || minute < 0 || minute > 59 || total > minutesPerDay {
		return ClockTime{}, ErrInvalidClockTime
	}
	return ClockTime{minutes: total}, nil
}

func (c ClockTime) sinceMidnight() time.Duration {
	return time.Duration(c.minutes) * time.Minute
}

// OpeningWindow is one open period on one weekday.
type OpeningWindow struct {
	Day   time.Weekday
	Open  ClockTime
	Close ClockTime
}

// WeeklyHours says when a space is open during a normal week. The zero value
// is closed all week.
type WeeklyHours struct {
	days [7][]OpeningWindow // indexed by time.Weekday, each day sorted by Open
}

// NewWeeklyHours validates windows and returns the week they describe. A day
// with no windows is closed. Windows on the same day must leave a gap between
// them: a booking has to fit inside one window, so 09-12 plus 12-17 would
// refuse 11-13 for no visible reason. Use 09-17 instead.
func NewWeeklyHours(windows []OpeningWindow) (WeeklyHours, error) {
	var h WeeklyHours

	for _, w := range windows {
		if w.Day < time.Sunday || w.Day > time.Saturday || w.Open.minutes >= w.Close.minutes {
			return WeeklyHours{}, ErrInvalidOpeningWindow
		}
		h.days[w.Day] = append(h.days[w.Day], w)
	}

	for _, day := range h.days {
		slices.SortFunc(day, func(a, b OpeningWindow) int {
			return cmp.Compare(a.Open.minutes, b.Open.minutes)
		})
		for i := 1; i < len(day); i++ {
			if day[i].Open.minutes <= day[i-1].Close.minutes {
				return WeeklyHours{}, ErrOverlappingWindows
			}
		}
	}

	return h, nil
}

func (h WeeklyHours) closedAllWeek() bool {
	for _, day := range h.days {
		if len(day) > 0 {
			return false
		}
	}
	return true
}

// Contains reports whether the period from start to end fits inside one
// opening window, read on the local clock of loc. A period ending exactly at
// midnight counts as ending at 24:00 on the earlier day. loc must not be nil.
//
// Only the two ends are checked. If the clock goes back during the period and
// a window opens or closes inside the repeated hour, the period can pass
// while briefly outside the window. See Risks in docs/ROADMAP.md.
func (h WeeklyHours) Contains(start, end time.Time, loc *time.Location) bool {
	if !start.Before(end) {
		return false
	}

	localStart := start.In(loc)
	localEnd := end.In(loc)

	// Look at the last moment of the period instead of the end itself, so
	// that ending at midnight stays on the earlier day. This also works when
	// the next day starts at 01:00 because the clocks changed at midnight.
	if !sameDate(localStart, end.Add(-time.Nanosecond).In(loc)) {
		return false
	}

	from := sinceMidnight(localStart)
	to := 24 * time.Hour
	if sameDate(localStart, localEnd) {
		to = sinceMidnight(localEnd)
	}

	for _, w := range h.days[localStart.Weekday()] {
		if w.Open.sinceMidnight() <= from && to <= w.Close.sinceMidnight() {
			return true
		}
	}
	return false
}

func sameDate(a, b time.Time) bool {
	aYear, aMonth, aDay := a.Date()
	bYear, bMonth, bDay := b.Date()
	return aYear == bYear && aMonth == bMonth && aDay == bDay
}

// sinceMidnight returns what t's clock shows as a duration. It includes the
// seconds, so 17:00:30 does not fit a window closing at 17:00.
func sinceMidnight(t time.Time) time.Duration {
	hour, minute, second := t.Clock()
	return time.Duration(hour)*time.Hour +
		time.Duration(minute)*time.Minute +
		time.Duration(second)*time.Second +
		time.Duration(t.Nanosecond())
}
