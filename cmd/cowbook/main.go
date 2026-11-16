// Command cowbook is the coworking booking app. It grows with each step of
// the roadmap and ends as the full app; for now it has one subcommand:
//
//	cowbook repl    a command prompt over the services and the in-memory store
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"nicoaravena/cow-book-app/cowbook"
	"nicoaravena/cow-book-app/internal/memstore"
	"nicoaravena/cow-book-app/internal/repl"
)

const usage = `usage: cowbook <command>

commands:
  repl    a command prompt over the services and the in-memory store`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "repl":
		err = runREPL()
	default:
		fmt.Fprintf(os.Stderr, "cowbook: unknown command %q\n%s\n", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "cowbook:", err)
		os.Exit(1)
	}
}

func runREPL() error {
	// A fixed start, Monday 16 November 2026 08:00 UTC, so a session prints
	// the same every run.
	clock := repl.NewClock(time.Date(2026, time.November, 16, 8, 0, 0, 0, time.UTC))
	store := memstore.New()
	bookings := cowbook.NewBookingService(store, store, store, clock.Now)
	session := repl.NewSession(store, bookings, clock)

	// Input from a terminal is a person typing; anything else, such as a
	// file redirected with <, is a script.
	info, err := os.Stdin.Stat()
	script := err == nil && info.Mode()&os.ModeCharDevice == 0

	return session.Run(context.Background(), os.Stdin, os.Stdout, script)
}
