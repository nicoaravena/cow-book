package repl_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"nicoaravena/cow-book-app/cowbook"
	"nicoaravena/cow-book-app/internal/memstore"
	"nicoaravena/cow-book-app/internal/repl"
)

// start is where the session clock begins: Monday 16 November 2026, 08:00 UTC.
var start = time.Date(2026, time.November, 16, 8, 0, 0, 0, time.UTC)

// run feeds script to a new session in script mode and returns all it printed.
func run(t *testing.T, script ...string) string {
	t.Helper()
	clock := repl.NewClock(start)
	store := memstore.New()
	bookings := cowbook.NewBookingService(store, store, store, clock.Now)
	members := cowbook.NewMemberService(store, store, clock.Now)
	s := repl.NewSession(store, bookings, members, clock)

	var out strings.Builder
	in := strings.NewReader(strings.Join(script, "\n") + "\n")
	if err := s.Run(context.Background(), in, &out, true); err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	return out.String()
}

// lastLine returns the last line printed, which is the answer to the last
// command in the script.
func lastLine(out string) string {
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	return lines[len(lines)-1]
}

// setup adds space 1 (meeting room for 6, bookings up to 2 hours, open
// weekdays 08:00-20:00 UTC) and member 1.
var setup = []string{
	"space add Room-A meeting_room 6 2h",
	"member add Ana ana@example.com",
}

func TestSessionTranscript(t *testing.T) {
	got := run(t, append(setup,
		"# Ana books, then tries the same slot again",
		"as member 1",
		"book 1 2026-11-16 09:00-10:00",
		"",
		"book 1 2026-11-16 09:30-10:30",
	)...)

	want := `[nobody]> space add Room-A meeting_room 6 2h
ok space #1
[nobody]> member add Ana ana@example.com
ok member #1
# Ana books, then tries the same slot again
[nobody]> as member 1
ok acting as member 1
[member 1]> book 1 2026-11-16 09:00-10:00
ok booking #1: space 1, Mon 2026-11-16 09:00-10:00 UTC

[member 1]> book 1 2026-11-16 09:30-10:30
error: space is already booked for that time
`
	if got != want {
		t.Errorf("output:\n%s\nwant:\n%s", got, want)
	}
}

func TestBookingRulesShowAsErrors(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{"outside opening hours", "book 1 2026-11-16 20:00-21:00", "error: booking must fit inside one opening window"},
		{"over capacity", "book 1 2026-11-16 09:00-10:00 7", "error: booking exceeds the space's capacity: 7 attendees, capacity 6"},
		{"too long", "book 1 2026-11-16 09:00-12:00", "error: booking exceeds the space's max booking length: 3h0m0s, limit 2h0m0s"},
		{"in the past", "book 1 2026-11-16 07:00-09:00", "error: booking must not start in the past"},
		{"no such space", "book 9 2026-11-16 09:00-10:00", "error: space not found"},
		{"member booking for someone else", "book 1 2026-11-16 09:00-10:00 1 for 5", "error: only the member or the operator's staff can book for this member"},
		{"missing times", "book 1 2026-11-16", "error: usage: book <spaceID> <YYYY-MM-DD> <HH:MM-HH:MM> [attendees] [for <memberID>]"},
		{"bad date", "book 1 16-11-2026 09:00-10:00", `error: date "16-11-2026" is not YYYY-MM-DD`},
		{"bad times", "book 1 2026-11-16 9-10", `error: times "9-10" are not HH:MM-HH:MM`},
		{"bad attendees", "book 1 2026-11-16 09:00-10:00 two", `error: attendees "two" is not a number`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := run(t, append(setup, "as member 1", tt.line)...)
			if got := lastLine(out); got != tt.want {
				t.Errorf("last line = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStaffBookingIsPending(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  string
	}{
		{
			"front desk at the space's location",
			[]string{"as frontdesk 1 1", "book 1 2026-11-16 11:00-12:00 for 1"},
			"ok booking #1: space 1, Mon 2026-11-16 11:00-12:00 UTC, pending: member 1 must approve by Mon 2026-11-16 11:00 UTC",
		},
		{
			"admin, deadline 24 hours away",
			[]string{"as admin 1", "book 1 2026-11-18 11:00-12:00 for 1"},
			"ok booking #1: space 1, Wed 2026-11-18 11:00-12:00 UTC, pending: member 1 must approve by Tue 2026-11-17 08:00 UTC",
		},
		{
			"front desk at another location",
			[]string{"as frontdesk 1 2,3", "book 1 2026-11-16 11:00-12:00 for 1"},
			"error: only the member or the operator's staff can book for this member",
		},
		{
			"staff must name the member",
			[]string{"as admin 1", "book 1 2026-11-16 11:00-12:00"},
			"error: staff book for a member: add for <memberID>",
		},
		{
			"nobody can't book",
			[]string{"book 1 2026-11-16 11:00-12:00"},
			"error: no one is acting: use as first",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := run(t, append(setup, tt.lines...)...)
			if got := lastLine(out); got != tt.want {
				t.Errorf("last line = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestClock(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  string
	}{
		{"show", []string{"clock"}, "now Mon 2026-11-16 08:00 UTC"},
		{"add", []string{"clock add 2h30m"}, "now Mon 2026-11-16 10:30 UTC"},
		{"set", []string{"clock set 2026-11-17 09:15"}, "now Tue 2026-11-17 09:15 UTC"},
		{"moving on makes a slot past", []string{"clock add 2h", "as member 1", "book 1 2026-11-16 09:00-10:00"}, "error: booking must not start in the past"},
		{"bad duration", []string{"clock add soon"}, `error: duration "soon" is not like 90m or 2h30m`},
		{"bad set", []string{"clock set tomorrow"}, "error: usage: clock [set <YYYY-MM-DD> <HH:MM> | add <duration>]"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := run(t, append(setup, tt.lines...)...)
			if got := lastLine(out); got != tt.want {
				t.Errorf("last line = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSpaceHoursAndTimezone(t *testing.T) {
	// Saturday 21 November 2026 in Madrid is CET, one hour ahead of UTC.
	desk := "space add Desk hot_desk 1 8h sat/10:00-14:00,mon-fri/09:00-17:30 Europe/Madrid"
	tests := []struct {
		name  string
		lines []string
		want  string
	}{
		{"inside a window, shown in the space's zone", []string{desk, "book 1 2026-11-21 10:00-11:00"}, "ok booking #1: space 1, Sat 2026-11-21 10:00-11:00 CET"},
		{"weekday range", []string{desk, "book 1 2026-11-20 17:00-17:30"}, "ok booking #1: space 1, Fri 2026-11-20 17:00-17:30 CET"},
		{"closed on Sunday", []string{desk, "book 1 2026-11-22 10:00-11:00"}, "error: booking must fit inside one opening window"},
		{"24:00 is the end of the day", []string{"space add Late hot_desk 1 4h sun/20:00-24:00", "book 1 2026-11-22 22:00-24:00"}, "ok booking #1: space 1, Sun 2026-11-22 22:00-00:00 UTC"},
		{"unknown day", []string{"space add Desk hot_desk 1 8h xyz/10:00-14:00"}, `error: day "xyz" is not one of mon tue wed thu fri sat sun`},
		{"backwards day range", []string{"space add Desk hot_desk 1 8h fri-mon/10:00-14:00"}, `error: day range "fri-mon" must run forward, mon to sun`},
		{"window without times", []string{"space add Desk hot_desk 1 8h sat"}, `error: window "sat" is not like mon-fri/09:00-18:00`},
		{"closing before opening", []string{"space add Desk hot_desk 1 8h sat/14:00-10:00"}, "error: opening window needs a valid weekday and must open before it closes"},
		{"unknown timezone", []string{"space add Desk hot_desk 1 8h sat/10:00-14:00 Mars/Olympus"}, `error: unknown timezone "Mars/Olympus"`},
		{"unknown kind", []string{"space add Desk sofa 1 8h"}, "error: space kind is not recognized"},
		{"zero capacity", []string{"space add Desk hot_desk 0 8h"}, "error: space capacity must be at least 1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := run(t, append([]string{"member add Ana ana@example.com", "as member 1"}, tt.lines...)...)
			if got := lastLine(out); got != tt.want {
				t.Errorf("last line = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMemberAdd(t *testing.T) {
	out := run(t, "member add Ana ana@example.com", "member add Bo bo@example.com", "member add Cy not-an-email")
	want := []string{"ok member #1", "ok member #2", "error: member email is not valid"}
	for _, w := range want {
		if !strings.Contains(out, w+"\n") {
			t.Errorf("output lacks %q:\n%s", w, out)
		}
	}
}

func TestListings(t *testing.T) {
	got := run(t, append(setup,
		"space add Desk-1 hot_desk 1 4h mon-fri/09:00-13:00 Europe/Madrid",
		"member add Bo bo@example.com",
		"member add Cy cy@example.com",
		"# staff book for Bo, who approves; Ana books and cancels; Cy is deleted",
		"as admin 9",
		"book 1 2026-11-16 11:00-12:00 3 for 2",
		"book 2 2026-11-17 09:00-10:00 for 3",
		"as member 2",
		"approve 1",
		"as member 1",
		"book 1 2026-11-16 09:00-10:00",
		"cancel 3",
		"book 2 2026-11-16 12:00-13:00",
		"member delete 3",
		"booking list",
		"member list",
		"space list",
	)...)

	want := `[nobody]> space add Room-A meeting_room 6 2h
ok space #1
[nobody]> member add Ana ana@example.com
ok member #1
[nobody]> space add Desk-1 hot_desk 1 4h mon-fri/09:00-13:00 Europe/Madrid
ok space #2
[nobody]> member add Bo bo@example.com
ok member #2
[nobody]> member add Cy cy@example.com
ok member #3
# staff book for Bo, who approves; Ana books and cancels; Cy is deleted
[nobody]> as admin 9
ok acting as admin 9
[admin 9]> book 1 2026-11-16 11:00-12:00 3 for 2
ok booking #1: space 1, Mon 2026-11-16 11:00-12:00 UTC, pending: member 2 must approve by Mon 2026-11-16 11:00 UTC
[admin 9]> book 2 2026-11-17 09:00-10:00 for 3
ok booking #2: space 2, Tue 2026-11-17 09:00-10:00 CET, pending: member 3 must approve by Tue 2026-11-17 09:00 CET
[admin 9]> as member 2
ok acting as member 2
[member 2]> approve 1
ok booking #1 approved
[member 2]> as member 1
ok acting as member 1
[member 1]> book 1 2026-11-16 09:00-10:00
ok booking #3: space 1, Mon 2026-11-16 09:00-10:00 UTC
[member 1]> cancel 3
ok booking #3 cancelled
[member 1]> book 2 2026-11-16 12:00-13:00
ok booking #4: space 2, Mon 2026-11-16 12:00-13:00 CET
[member 1]> member delete 3
ok member #3 deleted
[member 1]> booking list
ID  SPACE  MEMBER  WHEN                            ATTENDEES  STATE
1   1      2       Mon 2026-11-16 11:00-12:00 UTC  3          confirmed
2   2      3       Tue 2026-11-17 09:00-10:00 CET  1          cancelled
3   1      1       Mon 2026-11-16 09:00-10:00 UTC  1          cancelled
4   2      1       Mon 2026-11-16 12:00-13:00 CET  1          confirmed
[member 1]> member list
ID  NAME  EMAIL            STATE
1   Ana   ana@example.com  active
2   Bo    bo@example.com   active
3   Cy    cy@example.com   deleted
[member 1]> space list
ID  NAME    KIND          CAPACITY  MAX-LENGTH  TIMEZONE
1   Room-A  meeting_room  6         2h0m0s      UTC
2   Desk-1  hot_desk      1         4h0m0s      Europe/Madrid
`
	if got != want {
		t.Errorf("output:\n%s\nwant:\n%s", got, want)
	}
}

func TestEmptyListings(t *testing.T) {
	out := run(t, "space list", "member list", "booking list")
	for _, w := range []string{"no spaces", "no members", "no bookings"} {
		if !strings.Contains(out, "\n"+w+"\n") {
			t.Errorf("output lacks %q:\n%s", w, out)
		}
	}
}

// Each case starts with a booking admin 9 made for member 1 of space 1 at
// 11:00, pending until 11:00, and member 2 added.
func TestCancelApproveDelete(t *testing.T) {
	staffBooking := append(setup, "member add Bo bo@example.com", "as admin 9", "book 1 2026-11-16 11:00-12:00 for 1")
	tests := []struct {
		name  string
		lines []string
		want  string
	}{
		{"pending shows in the list", []string{"booking list"}, "1   1      1       Mon 2026-11-16 11:00-12:00 UTC  1          pending"},
		{"expired after the deadline", []string{"clock add 3h", "booking list"}, "1   1      1       Mon 2026-11-16 11:00-12:00 UTC  1          expired"},
		{"approve after the deadline", []string{"clock add 3h", "as member 1", "approve 1"}, "error: booking's approval deadline has passed"},
		{"approve by another member", []string{"as member 2", "approve 1"}, "error: only the member the booking is for can approve it"},
		{"approve by staff", []string{"approve 1"}, "error: only the member the booking is for can approve it"},
		{"approve twice", []string{"as member 1", "approve 1", "approve 1"}, "error: booking is not awaiting approval"},
		{"decline is cancel", []string{"as member 1", "cancel 1"}, "ok booking #1 cancelled"},
		{"cancel by another member", []string{"as member 2", "cancel 1"}, "error: only the booker or the operator's staff can cancel this booking"},
		{"cancel twice", []string{"cancel 1", "cancel 1"}, "error: booking is already cancelled"},
		{"no such booking", []string{"cancel 9"}, "error: booking not found"},
		{"deleting cancels the booking", []string{"member delete 1", "booking list"}, "1   1      1       Mon 2026-11-16 11:00-12:00 UTC  1          cancelled"},
		{"deleted member can't approve", []string{"member delete 1", "as member 1", "approve 1"}, "error: member is deleted"},
		{"deleted member can't be booked for", []string{"member delete 1", "book 1 2026-11-16 14:00-15:00 for 1"}, "error: member is deleted"},
		{"delete twice", []string{"member delete 1", "member delete 1"}, "error: member is deleted"},
		{"delete unknown member", []string{"member delete 9"}, "error: member not found"},
		{"cancel without an ID", []string{"cancel"}, "error: usage: cancel <bookingID>"},
		{"approve with a bad ID", []string{"approve one"}, "error: usage: approve <bookingID>"},
		{"bad member delete", []string{"member delete"}, "error: usage: member add <name> <email> | member delete <id> | member list"},
		{"bad booking command", []string{"booking show"}, "error: usage: booking list"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := run(t, append(staffBooking, tt.lines...)...)
			if got := lastLine(out); got != tt.want {
				t.Errorf("last line = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("nobody can't cancel or approve", func(t *testing.T) {
		for _, line := range []string{"cancel 1", "approve 1"} {
			if got := lastLine(run(t, line)); got != "error: no one is acting: use as first" {
				t.Errorf("%s: last line = %q", line, got)
			}
		}
	})
}

func TestSessionControl(t *testing.T) {
	t.Run("unknown command", func(t *testing.T) {
		if got := lastLine(run(t, "fly")); got != `error: unknown command "fly", try help` {
			t.Errorf("last line = %q", got)
		}
	})
	t.Run("help lists every command", func(t *testing.T) {
		out := run(t, "help")
		for _, cmd := range []string{"approve ", "as ", "book ", "booking list", "cancel ", "clock", "help", "member add", "quit", "space add"} {
			if !strings.Contains(out, "\n  "+cmd) {
				t.Errorf("help lacks %q:\n%s", cmd, out)
			}
		}
	})
	t.Run("quit stops reading", func(t *testing.T) {
		if got := lastLine(run(t, "quit", "clock")); got != "[nobody]> quit" {
			t.Errorf("last line = %q, want the quit echo", got)
		}
	})
	t.Run("bad as", func(t *testing.T) {
		if got := lastLine(run(t, "as frontdesk 1")); got != "error: usage: as member <id> | as frontdesk <id> <loc,loc...> | as admin <id>" {
			t.Errorf("last line = %q", got)
		}
	})
	t.Run("interactive mode prints prompts but no echo", func(t *testing.T) {
		clock := repl.NewClock(start)
		store := memstore.New()
		s := repl.NewSession(store, cowbook.NewBookingService(store, store, store, clock.Now), cowbook.NewMemberService(store, store, clock.Now), clock)
		var out strings.Builder
		if err := s.Run(context.Background(), strings.NewReader("clock\n"), &out, false); err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		want := "[nobody]> now Mon 2026-11-16 08:00 UTC\n[nobody]> \n"
		if out.String() != want {
			t.Errorf("output = %q, want %q", out.String(), want)
		}
	})
}
