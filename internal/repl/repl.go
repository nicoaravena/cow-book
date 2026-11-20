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
	"text/tabwriter"
	"time"

	"nicoaravena/cow-book-app/cowbook"
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

// Store is what a session reads and writes itself, besides what the services
// do. It is defined here, by its one user, and holds only the methods the
// commands call. internal/memstore's Store has them all, and so do the
// three Postgres stores embedded in one struct, so neither package needs to
// know this interface exists.
type Store interface {
	// LocationFor returns the location in timezone tz, adding one if needed.
	LocationFor(ctx context.Context, tz *time.Location) (int64, error)
	// CreateSpace stores sp and returns it with its ID set.
	CreateSpace(ctx context.Context, sp cowbook.Space) (cowbook.Space, error)
	// GetSpace returns the space and its location's timezone.
	GetSpace(ctx context.Context, id int64) (cowbook.Space, *time.Location, error)
	// ListSpaces returns every space, sorted by ID.
	ListSpaces(ctx context.Context) ([]cowbook.Space, error)
	// CreateMember stores m and returns it with its ID set, or
	// cowbook.ErrEmailTaken.
	CreateMember(ctx context.Context, m cowbook.Member) (cowbook.Member, error)
	// ListMembers returns every member, deleted ones included, sorted by ID.
	ListMembers(ctx context.Context) ([]cowbook.Member, error)
	// ListBookings returns every booking, cancelled ones included, sorted by ID.
	ListBookings(ctx context.Context) ([]cowbook.Booking, error)
}

// command is one REPL command. run gets the words after the command's name
// and returns the text to print.
type command struct {
	usage string
	help  string
	run   func(ctx context.Context, args []string) (string, error)
}

// Session is one REPL run: the store, the services, the clock and who is
// acting. One goroutine uses it, so nothing in it is locked.
type Session struct {
	store    Store
	bookings *cowbook.BookingService
	members  *cowbook.MemberService
	clock    *Clock

	actor    cowbook.Actor
	who      string // "member 1", "admin 2"...; empty until as
	memberID int64  // the acting member, 0 when staff or nobody acts

	commands map[string]command
}

// NewSession returns a Session over store and the services. clock must be
// the clock the services were built with, so clock commands move their time.
func NewSession(store Store, bookings *cowbook.BookingService, members *cowbook.MemberService, clock *Clock) *Session {
	s := &Session{store: store, bookings: bookings, members: members, clock: clock}
	// s.as and the others are method values: functions with s already bound.
	s.commands = map[string]command{
		"approve": {"approve <bookingID>", "approve a booking staff made for you", s.approve},
		"as":      {"as member <id> | as frontdesk <id> <loc,loc...> | as admin <id>", "act as a member or staff", s.as},
		"book":    {"book <spaceID> <YYYY-MM-DD> <HH:MM-HH:MM> [attendees] [for <memberID>]", "book a space, in the space's timezone", s.book},
		"booking": {"booking list", "list bookings with their state", s.booking},
		"cancel":  {"cancel <bookingID>", "cancel a booking, or decline one staff made for you", s.cancel},
		"clock":   {"clock [set <YYYY-MM-DD> <HH:MM> | add <duration>]", "show or move the session's time (UTC)", s.clockCmd},
		"help":    {"help", "list the commands", s.help},
		"member":  {"member add <name> <email> | member delete <id> | member list", "add, delete or list members; delete cancels their bookings", s.member},
		"quit":    {"quit", "end the session (end of input does too)", s.quit},
		"space":   {"space add <name> <kind> <capacity> <max-length> [hours] [timezone] | space list", "add a space at its timezone's location, or list spaces; hours like mon-fri/09:00-18:00,sat/10:00-14:00", s.space},
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

// requireActor returns an error unless someone is acting, for commands that
// need an actor.
func (s *Session) requireActor() error {
	if s.who == "" {
		return errors.New("no one is acting: use as first")
	}
	return nil
}

// table lines up header and rows in columns two spaces apart. A tabwriter
// buffers cells until Flush, because a column's width depends on its widest
// cell, which may come in the last row.
func table(header []string, rows [][]string) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(header, "\t"))
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	w.Flush()
	// Run ends each answer's line itself.
	return strings.TrimSuffix(b.String(), "\n")
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
