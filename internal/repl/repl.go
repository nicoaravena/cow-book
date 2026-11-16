// Package repl is a command prompt over the booking services, for trying the
// domain by hand or from a script. It is a delivery adapter: each command
// parses its words, calls a service or the store and prints the result. Every
// rule stays in cowbook.
package repl

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strings"

	"nicoaravena/cow-book-app/cowbook"
	"nicoaravena/cow-book-app/internal/memstore"
)

// timeFormat shows a moment with its weekday and zone, as in
// "Mon 2026-11-16 09:00 UTC".
const timeFormat = "Mon 2006-01-02 15:04 MST"

var (
	// errUsage makes the loop print the command's usage line.
	errUsage = errors.New("usage")
	// errQuit ends the session.
	errQuit = errors.New("quit")
)

// command is one REPL command. run gets the words after the command's name
// and returns the text to print.
type command struct {
	usage string
	help  string
	run   func(ctx context.Context, args []string) (string, error)
}

// Session is one REPL run: the store, the service, the clock and who is
// acting. One goroutine uses it, so nothing in it is locked.
type Session struct {
	store    *memstore.Store
	bookings *cowbook.BookingService
	clock    *Clock

	actor    cowbook.Actor
	who      string // "member 1", "admin 2"...; empty until as
	memberID int64  // the acting member, 0 when staff or nobody acts

	lastSpaceID  int64
	lastMemberID int64

	commands map[string]command
}

// NewSession returns a Session over store and bookings. clock must be the
// clock bookings was built with, so clock commands move the service's time.
func NewSession(store *memstore.Store, bookings *cowbook.BookingService, clock *Clock) *Session {
	s := &Session{store: store, bookings: bookings, clock: clock}
	// s.as and the others are method values: functions with s already bound.
	s.commands = map[string]command{
		"as":     {"as member <id> | as frontdesk <id> <loc,loc...> | as admin <id>", "act as a member or staff", s.as},
		"book":   {"book <spaceID> <YYYY-MM-DD> <HH:MM-HH:MM> [attendees] [for <memberID>]", "book a space, in the space's timezone", s.book},
		"clock":  {"clock [set <YYYY-MM-DD> <HH:MM> | add <duration>]", "show or move the session's time (UTC)", s.clockCmd},
		"help":   {"help", "list the commands", s.help},
		"member": {"member add <name> <email>", "add a member", s.member},
		"quit":   {"quit", "end the session (end of input does too)", s.quit},
		"space":  {"space add <name> <kind> <capacity> <max-length> [hours] [timezone]", "add a space at location 1; hours like mon-fri/09:00-18:00,sat/10:00-14:00", s.space},
	}
	return s
}

// Run reads commands from in until quit or the end of input, and writes to
// out. With script set, it echoes each command after its prompt and copies
// blank and # comment lines, so a script's output reads like a session.
// Command errors are printed and the session carries on; Run returns only
// an error reading in.
func (s *Session) Run(ctx context.Context, in io.Reader, out io.Writer, script bool) error {
	sc := bufio.NewScanner(in)
	for {
		if !script {
			fmt.Fprintf(out, "[%s]> ", s.prompt())
		}
		if !sc.Scan() {
			break
		}
		line := sc.Text()
		words := strings.Fields(line)
		if len(words) == 0 || strings.HasPrefix(words[0], "#") {
			if script {
				fmt.Fprintln(out, line)
			}
			continue
		}
		if script {
			fmt.Fprintf(out, "[%s]> %s\n", s.prompt(), line)
		}

		cmd, ok := s.commands[words[0]]
		if !ok {
			fmt.Fprintf(out, "error: unknown command %q, try help\n", words[0])
			continue
		}
		msg, err := cmd.run(ctx, words[1:])
		switch {
		case errors.Is(err, errQuit):
			return nil
		case errors.Is(err, errUsage):
			fmt.Fprintf(out, "error: usage: %s\n", cmd.usage)
		case err != nil:
			fmt.Fprintf(out, "error: %v\n", err)
		default:
			fmt.Fprintln(out, msg)
		}
	}
	if !script {
		fmt.Fprintln(out) // end the last prompt's line
	}
	return sc.Err()
}

func (s *Session) prompt() string {
	if s.who == "" {
		return "nobody"
	}
	return s.who
}

func (s *Session) help(_ context.Context, _ []string) (string, error) {
	var b strings.Builder
	b.WriteString("commands:")
	for _, name := range slices.Sorted(maps.Keys(s.commands)) {
		cmd := s.commands[name]
		fmt.Fprintf(&b, "\n  %s\n      %s", cmd.usage, cmd.help)
	}
	return b.String(), nil
}

func (s *Session) quit(_ context.Context, _ []string) (string, error) {
	return "", errQuit
}
