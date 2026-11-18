// Command cowbook is the coworking booking app. It grows with each step of
// the roadmap and ends as the full app. Its subcommands:
//
//	cowbook repl                         a command prompt over the services and the in-memory store
//	cowbook create-tenant <slug> <name>  add an operator with its own schema, in DATABASE_URL
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"nicoaravena/cow-book-app/cowbook"
	"nicoaravena/cow-book-app/internal/memstore"
	"nicoaravena/cow-book-app/internal/postgres"
	"nicoaravena/cow-book-app/internal/repl"
	"nicoaravena/cow-book-app/migrations"
)

const usage = `usage: cowbook <command>

commands:
  repl                         a command prompt over the services and the in-memory store
  create-tenant <slug> <name>  add an operator with its own schema, in DATABASE_URL`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, usage)
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "repl":
		err = runREPL()
	case "create-tenant":
		if len(os.Args) != 4 {
			fmt.Fprintln(os.Stderr, usage)
			os.Exit(2)
		}
		err = runCreateTenant(os.Args[2], os.Args[3])
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
	members := cowbook.NewMemberService(store, store, clock.Now)
	session := repl.NewSession(store, bookings, members, clock)

	// Input from a terminal is a person typing; anything else, such as a
	// file redirected with <, is a script.
	info, err := os.Stdin.Stat()
	script := err == nil && info.Mode()&os.ModeCharDevice == 0

	return session.Run(context.Background(), os.Stdin, os.Stdout, script)
}

func runCreateTenant(slug, name string) error {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errors.New("DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return err
	}
	defer pool.Close()

	id, err := postgres.NewProvisioner(pool, migrations.Tenant).CreateTenant(ctx, slug, name)
	if err != nil {
		return err
	}
	fmt.Printf("created tenant %d (%s) in schema %s\n", id, slug, postgres.SchemaName(id))
	return nil
}
