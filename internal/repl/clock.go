package repl

import (
	"context"
	"fmt"
	"time"
)

// Clock is the session's time, which commands can set or move. Pass its Now
// method to the services as their clock, so they see the same time.
type Clock struct {
	now time.Time
}

// NewClock returns a Clock reading start.
func NewClock(start time.Time) *Clock {
	return &Clock{now: start}
}

// Now returns the clock's current time.
func (c *Clock) Now() time.Time { return c.now }

func (s *Session) clockCmd(_ context.Context, args []string) (string, error) {
	switch {
	case len(args) == 0:
	case len(args) == 3 && args[0] == "set":
		t, err := time.Parse("2006-01-02 15:04", args[1]+" "+args[2])
		if err != nil {
			return "", errUsage
		}
		s.clock.now = t
	case len(args) == 2 && args[0] == "add":
		d, err := time.ParseDuration(args[1])
		if err != nil {
			return "", fmt.Errorf("duration %q is not like 90m or 2h30m", args[1])
		}
		s.clock.now = s.clock.now.Add(d)
	default:
		return "", errUsage
	}
	return "now " + s.clock.now.Format(timeFormat), nil
}
