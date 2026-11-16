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
	if err := s.requireActor(); err != nil {
		return "", err
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
	msg := fmt.Sprintf("ok booking #%d: space %d, %s", b.ID, b.SpaceID(), formatSlot(b, loc))
	if deadline := b.ApprovalDeadline(); !deadline.IsZero() {
		msg += fmt.Sprintf(", pending: member %d must approve by %s", b.MemberID(), deadline.In(loc).Format(timeFormat))
	}
	return msg, nil
}

// cancel cancels a booking, or declines one staff made for the actor.
func (s *Session) cancel(ctx context.Context, args []string) (string, error) {
	id, err := bookingID(args)
	if err != nil {
		return "", err
	}
	if err := s.requireActor(); err != nil {
		return "", err
	}
	b, err := s.bookings.Cancel(ctx, s.actor, id)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("ok booking #%d cancelled", b.ID), nil
}

func (s *Session) approve(ctx context.Context, args []string) (string, error) {
	id, err := bookingID(args)
	if err != nil {
		return "", err
	}
	if err := s.requireActor(); err != nil {
		return "", err
	}
	b, err := s.bookings.Approve(ctx, s.actor, id)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("ok booking #%d approved", b.ID), nil
}

// bookingID reads the one argument of cancel and approve.
func bookingID(args []string) (int64, error) {
	if len(args) != 1 {
		return 0, errUsage
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return 0, errUsage
	}
	return id, nil
}

// booking lists every booking, with its state at the session's time. The
// state comes from the domain, so the list and the rules agree on when a
// pending booking expires.
func (s *Session) booking(ctx context.Context, args []string) (string, error) {
	if len(args) != 1 || args[0] != "list" {
		return "", errUsage
	}
	bookings, err := s.store.ListBookings(ctx)
	if err != nil || len(bookings) == 0 {
		return "no bookings", err
	}
	now := s.clock.Now()
	var rows [][]string
	for _, b := range bookings {
		_, loc, err := s.store.GetSpace(ctx, b.SpaceID())
		if err != nil {
			return "", err
		}
		rows = append(rows, []string{
			strconv.FormatInt(b.ID, 10), strconv.FormatInt(b.SpaceID(), 10), strconv.FormatInt(b.MemberID(), 10),
			formatSlot(b, loc), strconv.Itoa(b.Attendees()), string(b.State(now)),
		})
	}
	return table([]string{"ID", "SPACE", "MEMBER", "WHEN", "ATTENDEES", "STATE"}, rows), nil
}

// formatSlot shows a booking's date and times on the wall clock at the space,
// as in "Mon 2026-11-16 09:00-10:00 UTC".
func formatSlot(b cowbook.Booking, loc *time.Location) string {
	return b.Start().In(loc).Format("Mon 2006-01-02 15:04") + "-" + b.End().In(loc).Format("15:04 MST")
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
