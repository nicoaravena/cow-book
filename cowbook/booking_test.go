package cowbook

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// bookingNow is the clock used by these tests: Monday 2 November 2026, 09:00
// UTC, the moment the test space opens that day.
var bookingNow = utc(time.November, 2, 9, 0)

// storedSpace returns a space as it looks after being stored: open Mondays
// 09:00 to 17:00, capacity 8, bookings up to 4 hours.
func storedSpace(t *testing.T) Space {
	t.Helper()
	s, err := NewSpace(validSpaceParams(), bookingNow)
	if err != nil {
		t.Fatalf("NewSpace() error = %v, want nil", err)
	}
	s.ID = 7
	return s
}

// validBookingParams returns params that pass validation on storedSpace, on
// Monday 9 November. Each test case changes one field, so a failure points at
// exactly one rule.
func validBookingParams() BookingParams {
	return BookingParams{
		MemberID:  3,
		Attendees: 2,
		Start:     utc(time.November, 9, 10, 0),
		End:       utc(time.November, 9, 11, 0),
	}
}

func TestNewBookingSetsFields(t *testing.T) {
	p := validBookingParams()

	b, err := NewBooking(storedSpace(t), time.UTC, p, bookingNow)
	if err != nil {
		t.Fatalf("NewBooking() error = %v, want nil", err)
	}

	if b.ID != 0 {
		t.Errorf("ID = %d, want 0 until the booking is stored", b.ID)
	}
	if b.spaceID != 7 {
		t.Errorf("spaceID = %d, want 7", b.spaceID)
	}
	if b.memberID != 3 {
		t.Errorf("memberID = %d, want 3", b.memberID)
	}
	if b.attendees != 2 {
		t.Errorf("attendees = %d, want 2", b.attendees)
	}
	if !b.start.Equal(p.Start) {
		t.Errorf("start = %v, want %v", b.start, p.Start)
	}
	if !b.end.Equal(p.End) {
		t.Errorf("end = %v, want %v", b.end, p.End)
	}
	if !b.createdAt.Equal(bookingNow) {
		t.Errorf("createdAt = %v, want %v", b.createdAt, bookingNow)
	}
	if !b.updatedAt.Equal(bookingNow) {
		t.Errorf("updatedAt = %v, want %v", b.updatedAt, bookingNow)
	}
	if !b.cancelledAt.IsZero() {
		t.Errorf("cancelledAt = %v, want zero on a new booking", b.cancelledAt)
	}
}

func TestNewBookingValidation(t *testing.T) {
	// times sets the period on Monday 9 November.
	times := func(p *BookingParams, startHour, startMinute, endHour, endMinute int) {
		p.Start = utc(time.November, 9, startHour, startMinute)
		p.End = utc(time.November, 9, endHour, endMinute)
	}

	tests := []struct {
		name    string
		change  func(p *BookingParams, s *Space)
		wantErr error
	}{
		{"valid params", func(p *BookingParams, s *Space) {}, nil},

		{"zero member", func(p *BookingParams, s *Space) { p.MemberID = 0 }, ErrInvalidMember},
		{"negative member", func(p *BookingParams, s *Space) { p.MemberID = -1 }, ErrInvalidMember},

		{"space not stored", func(p *BookingParams, s *Space) { s.ID = 0 }, ErrInvalidSpace},
		{"negative space", func(p *BookingParams, s *Space) { s.ID = -1 }, ErrInvalidSpace},

		{"unavailable space", func(p *BookingParams, s *Space) { s.available = false }, ErrSpaceUnavailable},

		{"one attendee", func(p *BookingParams, s *Space) { p.Attendees = 1 }, nil},
		{"zero attendees", func(p *BookingParams, s *Space) { p.Attendees = 0 }, ErrInvalidAttendees},
		{"negative attendees", func(p *BookingParams, s *Space) { p.Attendees = -1 }, ErrInvalidAttendees},
		{"exactly at capacity", func(p *BookingParams, s *Space) { p.Attendees = 8 }, nil},
		{"one over capacity", func(p *BookingParams, s *Space) { p.Attendees = 9 }, ErrOverCapacity},

		{"end equals start", func(p *BookingParams, s *Space) { p.End = p.Start }, ErrInvalidBookingTime},
		{"end before start", func(p *BookingParams, s *Space) { times(p, 11, 0, 10, 0) }, ErrInvalidBookingTime},

		{"starts exactly now", func(p *BookingParams, s *Space) {
			p.Start = bookingNow
			p.End = bookingNow.Add(time.Hour)
		}, nil},
		{"starts a nanosecond ago", func(p *BookingParams, s *Space) {
			p.Start = bookingNow.Add(-time.Nanosecond)
			p.End = bookingNow.Add(time.Hour)
		}, ErrBookingInPast},

		{"exactly the max length", func(p *BookingParams, s *Space) { times(p, 10, 0, 14, 0) }, nil},
		{"a nanosecond over the max length", func(p *BookingParams, s *Space) {
			times(p, 10, 0, 14, 0)
			p.End = p.End.Add(time.Nanosecond)
		}, ErrBookingTooLong},

		{"starts before opening", func(p *BookingParams, s *Space) { times(p, 8, 0, 9, 30) }, ErrOutsideOpeningHours},
		{"ends after closing", func(p *BookingParams, s *Space) { times(p, 16, 30, 17, 30) }, ErrOutsideOpeningHours},
		{"closed day", func(p *BookingParams, s *Space) {
			p.Start = utc(time.November, 10, 10, 0)
			p.End = utc(time.November, 10, 11, 0)
		}, ErrOutsideOpeningHours},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := storedSpace(t)
			p := validBookingParams()
			tt.change(&p, &s)

			_, err := NewBooking(s, time.UTC, p, bookingNow)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("NewBooking() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// Each case breaks two neighbouring rules and expects the earlier one.
func TestNewBookingRuleOrder(t *testing.T) {
	tests := []struct {
		name    string
		change  func(p *BookingParams, s *Space)
		wantErr error
	}{
		{"member before space", func(p *BookingParams, s *Space) {
			p.MemberID = 0
			s.ID = 0
		}, ErrInvalidMember},
		{"space before unavailable", func(p *BookingParams, s *Space) {
			s.ID = 0
			s.available = false
		}, ErrInvalidSpace},
		{"unavailable before attendees", func(p *BookingParams, s *Space) {
			s.available = false
			p.Attendees = 0
		}, ErrSpaceUnavailable},
		{"capacity before times", func(p *BookingParams, s *Space) {
			p.Attendees = 9
			p.End = p.Start
		}, ErrOverCapacity},
		{"backwards before past", func(p *BookingParams, s *Space) {
			p.Start = bookingNow.Add(-time.Hour)
			p.End = bookingNow.Add(-2 * time.Hour)
		}, ErrInvalidBookingTime},
		{"backwards before outside hours", func(p *BookingParams, s *Space) {
			p.Start = utc(time.November, 9, 20, 0)
			p.End = utc(time.November, 9, 19, 0)
		}, ErrInvalidBookingTime},
		{"past before too long", func(p *BookingParams, s *Space) {
			p.Start = bookingNow.Add(-time.Hour)
			p.End = bookingNow.Add(5 * time.Hour)
		}, ErrBookingInPast},
		{"too long before outside hours", func(p *BookingParams, s *Space) {
			p.Start = utc(time.November, 9, 10, 0)
			p.End = utc(time.November, 9, 20, 0)
		}, ErrBookingTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := storedSpace(t)
			p := validBookingParams()
			tt.change(&p, &s)

			_, err := NewBooking(s, time.UTC, p, bookingNow)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("NewBooking() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// A wrapped error is a new value, so == no longer matches the sentinel. That
// is why callers use errors.Is, which also looks inside the wrapping.
func TestNewBookingWrappedErrors(t *testing.T) {
	tests := []struct {
		name       string
		change     func(p *BookingParams)
		sentinel   error
		wantDetail string
	}{
		{
			"over capacity",
			func(p *BookingParams) { p.Attendees = 9 },
			ErrOverCapacity,
			"9 attendees, capacity 8",
		},
		{
			"too long",
			func(p *BookingParams) { p.End = p.Start.Add(5 * time.Hour) },
			ErrBookingTooLong,
			"5h0m0s, limit 4h0m0s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validBookingParams()
			tt.change(&p)

			_, err := NewBooking(storedSpace(t), time.UTC, p, bookingNow)

			if !errors.Is(err, tt.sentinel) {
				t.Errorf("errors.Is(%v, %v) = false, want true", err, tt.sentinel)
			}
			if err == tt.sentinel {
				t.Errorf("error is the bare sentinel, want it wrapped with detail")
			}
			if err != nil && !strings.Contains(err.Error(), tt.wantDetail) {
				t.Errorf("error = %q, want it to contain %q", err, tt.wantDetail)
			}
		})
	}
}

func TestNewBookingTimezone(t *testing.T) {
	newYork := loadLocation(t, "America/New_York")
	tokyo := loadLocation(t, "Asia/Tokyo")

	// Monday 15:00 UTC is Monday 10:00 in New York and Tuesday 00:00 in Tokyo.
	p := validBookingParams()
	p.Start = utc(time.November, 9, 15, 0)
	p.End = utc(time.November, 9, 16, 0)

	if _, err := NewBooking(storedSpace(t), newYork, p, bookingNow); err != nil {
		t.Errorf("NewBooking() in New York error = %v, want nil", err)
	}
	if _, err := NewBooking(storedSpace(t), tokyo, p, bookingNow); !errors.Is(err, ErrOutsideOpeningHours) {
		t.Errorf("NewBooking() in Tokyo error = %v, want %v", err, ErrOutsideOpeningHours)
	}
}

// New York in 2026: on 8 March the clock jumps from 02:00 to 03:00, and on
// 1 November it goes back from 02:00 to 01:00. The limit is real elapsed
// time, not the difference between what the clock shows.
func TestNewBookingLengthAcrossDST(t *testing.T) {
	newYork := loadLocation(t, "America/New_York")

	s := storedSpace(t)
	s.maxBookingLength = time.Hour
	s.hours = mustHours(OpeningWindow{time.Sunday, at(0, 0), at(4, 0)})

	beforeBoth := utc(time.January, 1, 0, 0)

	tests := []struct {
		name       string
		start, end time.Time
		wantErr    error
	}{
		{
			"spring: clock shows 2 hours, 1 hour passes",
			utc(time.March, 8, 6, 30), utc(time.March, 8, 7, 30), // 01:30 EST to 03:30 EDT
			nil,
		},
		{
			"fall: clock shows 1 hour, 2 hours pass",
			utc(time.November, 1, 4, 30), utc(time.November, 1, 6, 30), // 00:30 EDT to 01:30 EST
			ErrBookingTooLong,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := validBookingParams()
			p.Start = tt.start
			p.End = tt.end

			_, err := NewBooking(s, newYork, p, beforeBoth)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("NewBooking() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
