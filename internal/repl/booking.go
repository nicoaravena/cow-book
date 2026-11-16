package repl

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"nicoaravena/cow-book-app/cowbook"
)

func (s *Session) book(ctx context.Context, args []string) (string, error) {
	if len(args) < 3 || len(args) > 6 {
		return "", errUsage
	}
	spaceID, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return "", errUsage
	}
	if s.who == "" {
		return "", errors.New("no one is acting: use as first")
	}

	attendees, rest := 1, args[3:]
	if len(rest) > 0 && rest[0] != "for" {
		if attendees, err = strconv.Atoi(rest[0]); err != nil {
			return "", fmt.Errorf("attendees %q is not a number", rest[0])
		}
		rest = rest[1:]
	}
	memberID := s.memberID
	switch {
	case len(rest) == 2 && rest[0] == "for":
		if memberID, err = strconv.ParseInt(rest[1], 10, 64); err != nil {
			return "", errUsage
		}
	case len(rest) != 0:
		return "", errUsage
	case memberID == 0:
		return "", errors.New("staff book for a member: add for <memberID>")
	}

	// The date and times are read on the wall clock at the space, so the
	// space's timezone is needed before the service sees the booking.
	_, loc, err := s.store.GetSpace(ctx, spaceID)
	if err != nil {
		return "", err
	}
	start, end, err := parseSlot(args[1], args[2], loc)
	if err != nil {
		return "", err
	}

	b, err := s.bookings.Book(ctx, s.actor, spaceID, cowbook.BookingParams{
		MemberID:  memberID,
		Attendees: attendees,
		Start:     start,
		End:       end,
	})
	if err != nil {
		return "", err
	}
	msg := fmt.Sprintf("ok booking #%d: space %d, %s-%s", b.ID, b.SpaceID(),
		b.Start().In(loc).Format("Mon 2006-01-02 15:04"), b.End().In(loc).Format("15:04 MST"))
	if deadline := b.ApprovalDeadline(); !deadline.IsZero() {
		msg += fmt.Sprintf(", pending: member %d must approve by %s", b.MemberID(), deadline.In(loc).Format(timeFormat))
	}
	return msg, nil
}

// parseSlot reads a date and HH:MM-HH:MM times as wall-clock times in loc.
// An end of 24:00 is midnight at the start of the next day.
func parseSlot(date, times string, loc *time.Location) (start, end time.Time, err error) {
	day, err := time.Parse("2006-01-02", date)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("date %q is not YYYY-MM-DD", date)
	}
	from, to, ok := parseRange(times)
	if !ok {
		return time.Time{}, time.Time{}, fmt.Errorf("times %q are not HH:MM-HH:MM", times)
	}
	y, m, d := day.Date()
	// time.Date normalizes, so hour 24 rolls over to the next day.
	start = time.Date(y, m, d, from.hour, from.minute, 0, 0, loc)
	end = time.Date(y, m, d, to.hour, to.minute, 0, 0, loc)
	return start, end, nil
}

// clockReading is an hour and minute as typed, 00:00 to 24:00.
type clockReading struct {
	hour, minute int
}

// parseRange reads HH:MM-HH:MM.
func parseRange(s string) (from, to clockReading, ok bool) {
	a, b, found := strings.Cut(s, "-")
	if !found {
		return clockReading{}, clockReading{}, false
	}
	from, okFrom := parseClock(a)
	to, okTo := parseClock(b)
	return from, to, okFrom && okTo
}

// parseClock reads HH:MM. NewClockTime decides which readings are valid, so
// the REPL and the domain agree on what 24:00 or 09:60 mean.
func parseClock(s string) (clockReading, bool) {
	h, m, found := strings.Cut(s, ":")
	if !found || len(h) != 2 || len(m) != 2 {
		return clockReading{}, false
	}
	hour, errH := strconv.Atoi(h)
	minute, errM := strconv.Atoi(m)
	if errH != nil || errM != nil {
		return clockReading{}, false
	}
	if _, err := cowbook.NewClockTime(hour, minute); err != nil {
		return clockReading{}, false
	}
	return clockReading{hour, minute}, true
}
