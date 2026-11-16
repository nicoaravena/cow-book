package repl

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"nicoaravena/cow-book-app/cowbook"
)

// locationID is the location every space goes to. Locations arrive in 3.7.
const locationID = 1

// defaultHours is used when space add is given no hours.
const defaultHours = "mon-fri/08:00-20:00"

// days are the day names hours use, in the order a range runs.
var days = []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}

func (s *Session) space(_ context.Context, args []string) (string, error) {
	if len(args) < 5 || len(args) > 7 || args[0] != "add" {
		return "", errUsage
	}
	capacity, err := strconv.Atoi(args[3])
	if err != nil {
		return "", fmt.Errorf("capacity %q is not a number", args[3])
	}
	maxLength, err := time.ParseDuration(args[4])
	if err != nil {
		return "", fmt.Errorf("max length %q is not like 90m or 2h30m", args[4])
	}

	spec := defaultHours
	if len(args) > 5 {
		spec = args[5]
	}
	windows, err := parseWindows(spec)
	if err != nil {
		return "", err
	}
	hours, err := cowbook.NewWeeklyHours(windows)
	if err != nil {
		return "", err
	}

	loc := time.UTC
	if len(args) > 6 {
		if loc, err = time.LoadLocation(args[6]); err != nil {
			return "", fmt.Errorf("unknown timezone %q", args[6])
		}
	}

	sp, err := cowbook.NewSpace(cowbook.SpaceParams{
		LocationID:       locationID,
		Name:             args[1],
		Kind:             cowbook.SpaceKind(args[2]),
		Capacity:         capacity,
		MaxBookingLength: maxLength,
		Hours:            hours,
	}, s.clock.Now())
	if err != nil {
		return "", err
	}
	s.lastSpaceID++
	sp.ID = s.lastSpaceID
	s.store.AddSpace(sp, loc)
	return fmt.Sprintf("ok space #%d", sp.ID), nil
}

// parseWindows reads opening hours like mon-fri/09:00-18:00,sat/10:00-14:00.
// It checks only the format; NewWeeklyHours checks the windows make sense.
func parseWindows(spec string) ([]cowbook.OpeningWindow, error) {
	var windows []cowbook.OpeningWindow
	for part := range strings.SplitSeq(spec, ",") {
		dayPart, timePart, found := strings.Cut(part, "/")
		from, to, ok := parseRange(timePart)
		if !found || !ok {
			return nil, fmt.Errorf("window %q is not like mon-fri/09:00-18:00", part)
		}
		first, last, err := parseDays(dayPart)
		if err != nil {
			return nil, err
		}
		open, err := cowbook.NewClockTime(from.hour, from.minute)
		if err != nil {
			return nil, err
		}
		closing, err := cowbook.NewClockTime(to.hour, to.minute)
		if err != nil {
			return nil, err
		}
		for i := first; i <= last; i++ {
			// days starts on Monday, time.Weekday on Sunday.
			day := time.Weekday((i + 1) % 7)
			windows = append(windows, cowbook.OpeningWindow{Day: day, Open: open, Close: closing})
		}
	}
	return windows, nil
}

// parseDays reads a day ("sat") or a range of days ("mon-fri") and returns
// the indexes of the first and last day in days.
func parseDays(s string) (first, last int, err error) {
	from, to, isRange := strings.Cut(s, "-")
	if !isRange {
		to = from
	}
	first, last = slices.Index(days, from), slices.Index(days, to)
	switch {
	case first < 0:
		return 0, 0, fmt.Errorf("day %q is not one of %s", from, strings.Join(days, " "))
	case last < 0:
		return 0, 0, fmt.Errorf("day %q is not one of %s", to, strings.Join(days, " "))
	case last < first:
		return 0, 0, fmt.Errorf("day range %q must run forward, mon to sun", s)
	}
	return first, last, nil
}
